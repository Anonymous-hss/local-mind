package orchestrator

import (
	"fmt"

	"github.com/localmind/core/internal/engine"
	"github.com/localmind/core/pkg/protocol"
)

// Router dispatches requests to the appropriate engine
type Router struct {
	registry *engine.Registry
}

// NewRouter creates a new router with the given engine registry
func NewRouter(registry *engine.Registry) *Router {
	return &Router{registry: registry}
}

// Route returns the engine for handling the given request type
// Returns an error if no engine is registered for the type
func (r *Router) Route(reqType protocol.RequestType) (engine.Engine, error) {
	eng, ok := r.registry.Get(reqType)
	if !ok {
		return nil, fmt.Errorf("no engine registered for request type: %s", reqType)
	}
	return eng, nil
}

// CanHandle checks if there's an engine registered for the request type
func (r *Router) CanHandle(reqType protocol.RequestType) bool {
	_, ok := r.registry.Get(reqType)
	return ok
}

// RegisteredTypes returns all registered engine types
func (r *Router) RegisteredTypes() []protocol.RequestType {
	return r.registry.Types()
}
