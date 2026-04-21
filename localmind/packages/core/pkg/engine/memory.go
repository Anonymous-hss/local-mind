package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/localmind/core/pkg/protocol"
)

// MemoryEngine handles repo memory requests.
// In the community edition, this returns a stub directing users to LocalMind Pro.
type MemoryEngine struct{}

// NewMemoryEngine creates a new memory engine (community stub)
func NewMemoryEngine() *MemoryEngine {
	return &MemoryEngine{}
}

// Type returns the request type this engine handles
func (e *MemoryEngine) Type() protocol.RequestType {
	return protocol.RequestTypeMemory
}

// Execute returns a stub message directing to LocalMind Pro
func (e *MemoryEngine) Execute(ctx context.Context, req *protocol.Request) (*Result, error) {
	var payload protocol.MemoryPayload
	if req.Payload != nil {
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return nil, fmt.Errorf("invalid memory payload: %w", err)
		}
	}

	response := map[string]interface{}{
		"status":  "community",
		"message": "🧠 Repo memory is a Pro feature. Upgrade to LocalMind Pro for: tech stack inference, convention learning, and context-aware suggestions.",
	}

	result, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, err
	}

	return &Result{
		Content: string(result),
		Model:   "community",
	}, nil
}
