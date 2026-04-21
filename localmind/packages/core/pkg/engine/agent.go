package engine

import (
	"context"
	"encoding/json"

	"github.com/localmind/core/pkg/protocol"
)

// AgentEngine handles multi-file task requests.
// In the community edition, this returns a stub directing users to LocalMind Pro.
type AgentEngine struct {
	model string
}

// NewAgentEngine creates a new agent engine (community stub)
func NewAgentEngine() *AgentEngine {
	return &AgentEngine{
		model: "community",
	}
}

// SetModel updates the model (no-op in community edition)
func (e *AgentEngine) SetModel(model string) {
	e.model = model
}

// SetMemoryEngine is a no-op in the community edition
func (e *AgentEngine) SetMemoryEngine(m *MemoryEngine) {
	// Premium feature — available in LocalMind Pro
}

// Type returns the request type this engine handles
func (e *AgentEngine) Type() protocol.RequestType {
	return protocol.RequestTypeAgent
}

// Execute returns a stub message directing to LocalMind Pro
func (e *AgentEngine) Execute(ctx context.Context, req *protocol.Request) (*Result, error) {
	var payload protocol.AgentPayload
	if req.Payload != nil {
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return nil, err
		}
	}

	// Status requests return minimal info
	if payload.Action == "status" {
		status := map[string]interface{}{
			"status":  "community",
			"message": "Agent features are available in LocalMind Pro.",
		}
		result, _ := json.MarshalIndent(status, "", "  ")
		return &Result{Content: string(result), Model: "community"}, nil
	}

	// All other actions return upgrade message
	response := map[string]interface{}{
		"status":  "upgrade_required",
		"message": "🚀 Multi-file agent tasks are a Pro feature. Upgrade to LocalMind Pro for: task planning, execution, rollback, error recovery, scoring, and strategy learning.",
		"task":    truncate(payload.Task, 100),
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
