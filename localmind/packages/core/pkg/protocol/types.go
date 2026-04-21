// Package protocol defines the message types for IPC communication
// between the VS Code extension and the LocalMind core engine.
package protocol

import (
	"encoding/json"
	"time"
)

// Protocol version for compatibility checking
const ProtocolVersion = "1.0.0"

// RequestType defines the type of request
type RequestType string

const (
	RequestTypeHandshake  RequestType = "handshake"
	RequestTypePing       RequestType = "ping"
	RequestTypeCompletion RequestType = "completion"
	RequestTypeSuggestion RequestType = "suggestion"
	RequestTypeAgent      RequestType = "agent"
	RequestTypeMemory     RequestType = "memory"
	RequestTypeCancel     RequestType = "cancel"
)

// ResponseType defines the type of response
type ResponseType string

const (
	ResponseTypeHandshakeAck ResponseType = "handshake_ack"
	ResponseTypePong         ResponseType = "pong"
	ResponseTypeResult       ResponseType = "result"
	ResponseTypeStream       ResponseType = "stream"
	ResponseTypeError        ResponseType = "error"
	ResponseTypeCancelled    ResponseType = "cancelled"
)

// ErrorCode defines standardized error codes
type ErrorCode string

const (
	ErrorCodeInternalError   ErrorCode = "INTERNAL_ERROR"
	ErrorCodeTimeout         ErrorCode = "TIMEOUT"
	ErrorCodeCancelled       ErrorCode = "CANCELLED"
	ErrorCodeInvalidRequest  ErrorCode = "INVALID_REQUEST"
	ErrorCodeModelUnavail    ErrorCode = "MODEL_UNAVAILABLE"
	ErrorCodeContextTooLarge ErrorCode = "CONTEXT_TOO_LARGE"
	ErrorCodeLatencyExceeded ErrorCode = "LATENCY_EXCEEDED"
)

// OllamaStatus represents the connection status to Ollama
type OllamaStatus string

const (
	OllamaConnected    OllamaStatus = "connected"
	OllamaDisconnected OllamaStatus = "disconnected"
	OllamaUnknown      OllamaStatus = "unknown"
)

// SuggestionType defines the type of suggestion request
type SuggestionType string

const (
	SuggestionRefactor SuggestionType = "refactor"
	SuggestionOptimize SuggestionType = "optimize"
	SuggestionExplain  SuggestionType = "explain"
	SuggestionFix      SuggestionType = "fix"
)

// LatencyBudgets defines the timeout budgets for each request type
var LatencyBudgets = map[RequestType]time.Duration{
	RequestTypeCompletion: 150 * time.Millisecond,
	RequestTypeSuggestion: 1000 * time.Millisecond,
	RequestTypePing:       100 * time.Millisecond,
	RequestTypeMemory:     5 * time.Second,
}

// Request represents an incoming request from the extension
type Request struct {
	ID        string          `json:"id"`
	Timestamp int64           `json:"timestamp"`
	Type      RequestType     `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// Response represents an outgoing response to the extension
type Response struct {
	ID        string          `json:"id"`
	Timestamp int64           `json:"timestamp"`
	RequestID string          `json:"requestId"`
	Type      ResponseType    `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// HandshakePayload is the payload for handshake requests
type HandshakePayload struct {
	ProtocolVersion  string `json:"protocolVersion"`
	ExtensionVersion string `json:"extensionVersion"`
}

// HandshakeAckPayload is the payload for handshake acknowledgment
type HandshakeAckPayload struct {
	ProtocolVersion string `json:"protocolVersion"`
	CoreVersion     string `json:"coreVersion"`
	Compatible      bool   `json:"compatible"`
}

// ModelRole defines the role a model is assigned to
type ModelRole string

const (
	ModelRoleCompletion ModelRole = "completion"
	ModelRoleSuggestion ModelRole = "suggestion"
	ModelRoleAgent      ModelRole = "agent"
)

// ModelInfo describes an available Ollama model
type ModelInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"` // bytes
	Role string `json:"role"` // assigned role (completion/suggestion/agent), empty if unassigned
}

// PongPayload is the payload for pong responses
type PongPayload struct {
	Version      string            `json:"version"`
	OllamaStatus OllamaStatus      `json:"ollamaStatus"`
	Models       []ModelInfo       `json:"models,omitempty"`
	ActiveModels map[string]string `json:"activeModels,omitempty"` // role → model name
}

// CompletionPayload is the payload for completion requests
type CompletionPayload struct {
	Prefix    string `json:"prefix"`
	Suffix    string `json:"suffix,omitempty"`
	Language  string `json:"language"`
	FilePath  string `json:"filePath,omitempty"`
	MaxTokens int    `json:"maxTokens,omitempty"`
}

// SuggestionPayload is the payload for suggestion requests
type SuggestionPayload struct {
	File           string         `json:"file"`
	StartLine      int            `json:"startLine"`
	EndLine        int            `json:"endLine"`
	Content        string         `json:"content"`
	Context        string         `json:"context"`
	Language       string         `json:"language"`
	SuggestionType SuggestionType `json:"suggestionType"`
}

// AgentPayload is the payload for agent requests
type AgentPayload struct {
	// Action: plan, execute, approve, reject, rollback, status
	Action        string   `json:"action,omitempty"`
	Task          string   `json:"task,omitempty"`
	Files         []string `json:"files,omitempty"`
	WorkspaceRoot string   `json:"workspaceRoot,omitempty"`
	TaskID        string   `json:"taskId,omitempty"`
	StepID        string   `json:"stepId,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	// Settings (from VS Code configuration)
	Model         string `json:"model,omitempty"`
	ContextBudget int    `json:"contextBudget,omitempty"`
	MaxSteps      int    `json:"maxSteps,omitempty"`
	CursorFile    string `json:"cursorFile,omitempty"`
	CursorLine    int    `json:"cursorLine,omitempty"`
}

// MemoryPayload is the payload for memory requests
type MemoryPayload struct {
	// Action: get, update, delete
	Action    string `json:"action"`
	Workspace string `json:"workspace,omitempty"`
	Category  string `json:"category,omitempty"`
	Key       string `json:"key,omitempty"`
	Value     string `json:"value,omitempty"`
}

// CancelPayload is the payload for cancel requests
type CancelPayload struct {
	RequestID string `json:"requestId"`
}

// ResultPayload is the payload for result responses
type ResultPayload struct {
	Content   string `json:"content"`
	LatencyMs int64  `json:"latencyMs"`
	Model     string `json:"model,omitempty"`
}

// StreamPayload is the payload for streaming responses
type StreamPayload struct {
	Chunk string `json:"chunk"`
	Done  bool   `json:"done"`
}

// ErrorPayload is the payload for error responses
type ErrorPayload struct {
	Code    ErrorCode              `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewResponse creates a new response with current timestamp
func NewResponse(requestID string, respType ResponseType) Response {
	return Response{
		ID:        generateID(),
		Timestamp: time.Now().UnixMilli(),
		RequestID: requestID,
		Type:      respType,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(requestID string, code ErrorCode, message string) Response {
	resp := NewResponse(requestID, ResponseTypeError)
	payload, _ := json.Marshal(ErrorPayload{
		Code:    code,
		Message: message,
	})
	resp.Payload = payload
	return resp
}

// NewResultResponse creates a result response
func NewResultResponse(requestID string, content string, latencyMs int64, model string) Response {
	resp := NewResponse(requestID, ResponseTypeResult)
	payload, _ := json.Marshal(ResultPayload{
		Content:   content,
		LatencyMs: latencyMs,
		Model:     model,
	})
	resp.Payload = payload
	return resp
}

// NewCancelledResponse creates a cancelled response
func NewCancelledResponse(requestID string) Response {
	return NewResponse(requestID, ResponseTypeCancelled)
}

// generateID generates a simple unique ID
// In production, use github.com/google/uuid
func generateID() string {
	return time.Now().Format("20060102150405.000000000")
}
