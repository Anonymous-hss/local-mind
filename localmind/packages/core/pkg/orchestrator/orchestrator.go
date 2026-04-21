package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	"github.com/localmind/core/pkg/engine"
	"github.com/localmind/core/pkg/ipc"
	"github.com/localmind/core/pkg/ollama"
	"github.com/localmind/core/pkg/protocol"
)

// Version is the core engine version
const Version = "0.0.1"

// Orchestrator is the main control plane for LocalMind
type Orchestrator struct {
	reader       *ipc.Reader
	writer       *ipc.Writer
	router       *Router
	cancellation *CancellationManager
	timeout      *TimeoutManager
	ollamaClient *ollama.Client
	modelRouter  *engine.ModelRouter

	// logger for internal events
	logger *log.Logger

	// shutdown signal
	shutdown chan struct{}
}

// Config holds orchestrator configuration
type Config struct {
	Input         io.Reader
	Output        io.Writer
	TimeoutConfig *TimeoutConfig
	Logger        *log.Logger
	OllamaURL     string
}

// DefaultConfig returns default configuration using stdin/stdout
func DefaultConfig() *Config {
	return &Config{
		Input:         os.Stdin,
		Output:        os.Stdout,
		TimeoutConfig: DefaultTimeoutConfig(),
		Logger:        log.New(os.Stderr, "[LocalMind] ", log.LstdFlags),
	}
}

// New creates a new Orchestrator with the given configuration
func New(config *Config) *Orchestrator {
	if config == nil {
		config = DefaultConfig()
	}

	// Create Ollama client
	ollamaConfig := ollama.DefaultClientConfig()
	if config.OllamaURL != "" {
		ollamaConfig.BaseURL = config.OllamaURL
	}
	ollamaClient := ollama.NewClient(ollamaConfig)

	// Create model router — discovers models and assigns by capability
	modelRouter := engine.NewModelRouter(ollamaClient, config.Logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := modelRouter.Discover(ctx); err != nil {
		config.Logger.Printf("Model discovery failed (Ollama may be offline): %v", err)
	}
	cancel()

	// Create engines with router-assigned models
	completionEngine := engine.NewCompletionEngineWithConfig(ollamaClient, nil)
	if m := modelRouter.GetModelForRole(protocol.ModelRoleCompletion); m != "" {
		completionEngine.SetModel(m)
	}

	suggestionEngine := engine.NewSuggestionEngineWithClient(ollamaClient)
	if m := modelRouter.GetModelForRole(protocol.ModelRoleSuggestion); m != "" {
		suggestionEngine.SetModel(m)
	}

	agentEngine := engine.NewAgentEngine()
	if m := modelRouter.GetModelForRole(protocol.ModelRoleAgent); m != "" {
		agentEngine.SetModel(m)
	}

	memoryEngine := engine.NewMemoryEngine()

	// Create engine registry
	registry := engine.NewRegistry()
	registry.Register(completionEngine)
	registry.Register(suggestionEngine)
	registry.Register(agentEngine)
	registry.Register(memoryEngine)

	return &Orchestrator{
		reader:       ipc.NewReader(config.Input),
		writer:       ipc.NewWriter(config.Output),
		router:       NewRouter(registry),
		cancellation: NewCancellationManager(),
		timeout:      NewTimeoutManager(config.TimeoutConfig),
		ollamaClient: ollamaClient,
		modelRouter:  modelRouter,
		logger:       config.Logger,
		shutdown:     make(chan struct{}),
	}
}

// Run starts the orchestrator main loop
// Blocks until context is cancelled or EOF is received
func (o *Orchestrator) Run(ctx context.Context) error {
	o.logger.Println("Starting LocalMind Core Engine v" + Version)

	requests := make(chan *protocol.Request, 10)
	errors := make(chan error, 10)

	// Start read loop in goroutine
	go o.reader.ReadLoop(ctx, requests, errors)

	for {
		select {
		case <-ctx.Done():
			o.logger.Println("Shutting down...")
			o.cancellation.CancelAll()
			return ctx.Err()

		case <-o.shutdown:
			o.logger.Println("Shutdown requested")
			o.cancellation.CancelAll()
			return nil

		case err := <-errors:
			if err == io.EOF {
				o.logger.Println("Input stream closed")
				return nil
			}
			o.logger.Printf("Read error: %v", err)

		case req := <-requests:
			// Handle request in goroutine (non-blocking)
			go o.handleRequest(ctx, req)
		}
	}
}

// Shutdown signals the orchestrator to stop
func (o *Orchestrator) Shutdown() {
	close(o.shutdown)
}

// handleRequest processes a single request
func (o *Orchestrator) handleRequest(parentCtx context.Context, req *protocol.Request) {
	// Panic recovery - ensures no unhandled panics crash the engine
	defer func() {
		if r := recover(); r != nil {
			o.logger.Printf("PANIC recovered in request %s: %v", req.ID, r)
			o.sendError(req.ID, protocol.ErrorCodeInternalError, "internal panic recovered")
		}
	}()

	startTime := time.Now()
	o.logger.Printf("Received request: type=%s id=%s", req.Type, req.ID)

	// Handle special request types
	switch req.Type {
	case protocol.RequestTypeHandshake:
		o.handleHandshake(req)
		return
	case protocol.RequestTypePing:
		o.handlePing(req)
		return
	case protocol.RequestTypeCancel:
		o.handleCancel(req)
		return
	}

	// Register request for cancellation tracking (with debounce)
	ctx := o.cancellation.Register(parentCtx, req)
	defer o.cancellation.Complete(req.ID)

	// Apply timeout
	ctx, cancel := o.timeout.WrapContext(ctx, req.Type)
	defer cancel()

	// Route to appropriate engine
	eng, err := o.router.Route(req.Type)
	if err != nil {
		o.logger.Printf("Routing error: %v", err)
		o.sendError(req.ID, protocol.ErrorCodeInvalidRequest, err.Error())
		return
	}

	// Inject stream writer for agent requests
	if req.Type == protocol.RequestTypeAgent {
		ctx = engine.WithStreamWriter(ctx, o.writer)
	}

	// Execute request
	result, err := eng.Execute(ctx, req)
	latencyMs := time.Since(startTime).Milliseconds()

	// Handle execution result
	if err != nil {
		if IsCancelled(err) {
			o.logger.Printf("Request cancelled: id=%s", req.ID)
			o.sendCancelled(req.ID)
			return
		}
		if IsTimeout(err) {
			o.logger.Printf("Request timeout: id=%s latency=%dms", req.ID, latencyMs)
			o.sendError(req.ID, protocol.ErrorCodeTimeout, "Request exceeded latency budget")
			return
		}
		o.logger.Printf("Execution error: %v", err)
		o.sendError(req.ID, protocol.ErrorCodeInternalError, err.Error())
		return
	}

	// Send successful result
	o.logger.Printf("Request complete: id=%s latency=%dms", req.ID, latencyMs)
	o.sendResult(req.ID, result.Content, latencyMs, result.Model)
}

// handleHandshake processes a handshake request
func (o *Orchestrator) handleHandshake(req *protocol.Request) {
	var payload protocol.HandshakePayload
	if req.Payload != nil {
		json.Unmarshal(req.Payload, &payload)
	}

	// Check protocol version compatibility
	compatible := payload.ProtocolVersion == protocol.ProtocolVersion

	ackPayload, _ := json.Marshal(protocol.HandshakeAckPayload{
		ProtocolVersion: protocol.ProtocolVersion,
		CoreVersion:     Version,
		Compatible:      compatible,
	})

	resp := protocol.Response{
		ID:        generateID(),
		Timestamp: time.Now().UnixMilli(),
		RequestID: req.ID,
		Type:      protocol.ResponseTypeHandshakeAck,
		Payload:   ackPayload,
	}

	o.writer.Write(&resp)
	o.logger.Printf("Handshake complete: compatible=%v", compatible)
}

// handlePing processes a ping request
func (o *Orchestrator) handlePing(req *protocol.Request) {
	// Check actual Ollama status
	ollamaStatus := protocol.OllamaUnknown
	if o.ollamaClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := o.ollamaClient.Ping(ctx); err == nil {
			ollamaStatus = protocol.OllamaConnected

			// Re-discover models if we don't have any yet (Ollama may have just started)
			if o.modelRouter != nil && !o.modelRouter.HasModels() {
				o.modelRouter.Discover(ctx)
			}
		} else {
			ollamaStatus = protocol.OllamaDisconnected
		}
	}

	// Build pong with model info
	pong := protocol.PongPayload{
		Version:      Version,
		OllamaStatus: ollamaStatus,
	}
	if o.modelRouter != nil {
		pong.Models = o.modelRouter.GetModelsInfo()
		pong.ActiveModels = o.modelRouter.GetActiveModels()
	}

	pongPayload, _ := json.Marshal(pong)

	resp := protocol.Response{
		ID:        generateID(),
		Timestamp: time.Now().UnixMilli(),
		RequestID: req.ID,
		Type:      protocol.ResponseTypePong,
		Payload:   pongPayload,
	}

	o.writer.Write(&resp)
}

// handleCancel processes a cancel request
func (o *Orchestrator) handleCancel(req *protocol.Request) {
	var payload protocol.CancelPayload
	if req.Payload != nil {
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			o.sendError(req.ID, protocol.ErrorCodeInvalidRequest, "Invalid cancel payload")
			return
		}
	}

	cancelled := o.cancellation.Cancel(payload.RequestID)
	if cancelled {
		o.logger.Printf("Cancelled request: %s", payload.RequestID)
	} else {
		o.logger.Printf("Cancel request for unknown/completed request: %s", payload.RequestID)
	}

	// Always send cancelled response (even if already complete)
	o.sendCancelled(payload.RequestID)
}

// sendError sends an error response
func (o *Orchestrator) sendError(requestID string, code protocol.ErrorCode, message string) {
	o.writer.WriteError(requestID, code, message)
}

// sendResult sends a result response
func (o *Orchestrator) sendResult(requestID string, content string, latencyMs int64, model string) {
	o.writer.WriteResult(requestID, content, latencyMs, model)
}

// sendCancelled sends a cancelled response
func (o *Orchestrator) sendCancelled(requestID string) {
	o.writer.WriteCancelled(requestID)
}

// generateID generates a unique ID (simple implementation)
func generateID() string {
	return time.Now().Format("20060102150405.000000000")
}
