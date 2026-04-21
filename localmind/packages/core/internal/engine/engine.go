// Package engine defines the interfaces and stub implementations
// for the LocalMind processing engines.
package engine

import (
	"context"

	"github.com/localmind/core/pkg/protocol"
)

// Engine is the interface that all processing engines must implement
type Engine interface {
	// Execute processes a request and returns a result
	// The context should be used for cancellation and timeouts
	Execute(ctx context.Context, req *protocol.Request) (*Result, error)

	// Type returns the request type this engine handles
	Type() protocol.RequestType
}

// Result represents the output from an engine execution
type Result struct {
	Content string
	Model   string
}

// Registry holds all registered engines
type Registry struct {
	engines map[protocol.RequestType]Engine
}

// NewRegistry creates a new engine registry
func NewRegistry() *Registry {
	return &Registry{
		engines: make(map[protocol.RequestType]Engine),
	}
}

// Register adds an engine to the registry
func (r *Registry) Register(e Engine) {
	r.engines[e.Type()] = e
}

// Get returns the engine for a given request type
func (r *Registry) Get(reqType protocol.RequestType) (Engine, bool) {
	e, ok := r.engines[reqType]
	return e, ok
}

// Types returns all registered engine types
func (r *Registry) Types() []protocol.RequestType {
	types := make([]protocol.RequestType, 0, len(r.engines))
	for t := range r.engines {
		types = append(types, t)
	}
	return types
}
