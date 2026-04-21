package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/localmind/core/internal/ollama"
	"github.com/localmind/core/internal/suggestion"
	"github.com/localmind/core/pkg/protocol"
)

// SuggestionEngine wraps the real suggestion.Engine to implement the engine.Engine interface
type SuggestionEngine struct {
	real *suggestion.Engine

	// SimulatedDelay is kept for backward compatibility with tests
	// Only used when real engine is nil (stub mode)
	SimulatedDelay time.Duration

	// Store for model re-assignment
	client *ollama.Client
	model  string
}

// NewSuggestionEngine creates a suggestion engine stub (no Ollama client)
// This is kept for backward compatibility with tests
func NewSuggestionEngine() *SuggestionEngine {
	return &SuggestionEngine{
		real:           nil,
		SimulatedDelay: 200 * time.Millisecond,
	}
}

// NewSuggestionEngineWithClient creates a suggestion engine backed by the real suggestion pipeline
func NewSuggestionEngineWithClient(client *ollama.Client) *SuggestionEngine {
	model := "codellama"
	return &SuggestionEngine{
		real:   suggestion.NewEngine(client, model),
		client: client,
		model:  model,
	}
}

// SetModel updates the model used by the suggestion engine
func (e *SuggestionEngine) SetModel(model string) {
	e.model = model
	if e.client != nil {
		e.real = suggestion.NewEngine(e.client, model)
	}
}

// Type returns the request type this engine handles
func (e *SuggestionEngine) Type() protocol.RequestType {
	return protocol.RequestTypeSuggestion
}

// Execute processes a suggestion request
func (e *SuggestionEngine) Execute(ctx context.Context, req *protocol.Request) (*Result, error) {
	// If no real engine, return a stub response
	if e.real == nil {
		return e.executeStub(ctx, req)
	}

	// Delegate to the real suggestion engine
	result, err := e.real.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// Serialize the suggestion result to JSON for the Result.Content field
	data, err := json.Marshal(result.Suggestions)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize suggestions: %w", err)
	}

	return &Result{
		Content: string(data),
		Model:   "suggestion-engine",
	}, nil
}

// executeStub returns a mock response when no Ollama client is available
func (e *SuggestionEngine) executeStub(ctx context.Context, req *protocol.Request) (*Result, error) {
	var payload protocol.SuggestionPayload
	if req.Payload != nil {
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return nil, err
		}
	}

	// Simulate processing time (respects cancellation)
	if e.SimulatedDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(e.SimulatedDelay):
		}
	}

	stub := fmt.Sprintf(`[STUB] %s suggestion for %s code`,
		payload.SuggestionType,
		payload.Language,
	)

	return &Result{
		Content: stub,
		Model:   "stub-suggestion-model",
	}, nil
}
