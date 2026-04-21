// Package orchestrator provides the core control plane for LocalMind.
// It handles request routing, cancellation, and timeout enforcement.
package orchestrator

import (
	"context"
	"sync"

	"github.com/localmind/core/pkg/protocol"
)

// CancellationManager tracks in-flight requests and handles cancellation
type CancellationManager struct {
	mu sync.Mutex

	// inflight maps request ID to cancel function
	inflight map[string]context.CancelFunc

	// byType tracks the latest request ID for each debounce-able type
	// When a new request of this type arrives, the previous one is cancelled
	byType map[protocol.RequestType]string

	// debounceTypes defines which request types should debounce (cancel previous)
	debounceTypes map[protocol.RequestType]bool
}

// NewCancellationManager creates a new cancellation manager
func NewCancellationManager() *CancellationManager {
	return &CancellationManager{
		inflight: make(map[string]context.CancelFunc),
		byType:   make(map[protocol.RequestType]string),
		debounceTypes: map[protocol.RequestType]bool{
			// Completion requests debounce - rapid typing cancels previous
			protocol.RequestTypeCompletion: true,
			// Suggestion requests also debounce
			protocol.RequestTypeSuggestion: true,
			// Agent requests do NOT debounce - explicit cancel only
			protocol.RequestTypeAgent: false,
		},
	}
}

// Register registers a new request and returns a cancellable context.
// If the request type is debounce-able, cancels any previous request of same type.
// Returns the context to use for the request execution.
func (cm *CancellationManager) Register(parentCtx context.Context, req *protocol.Request) context.Context {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// If this type debounces, cancel previous request of same type
	if cm.debounceTypes[req.Type] {
		if prevID, exists := cm.byType[req.Type]; exists {
			if cancel, ok := cm.inflight[prevID]; ok {
				cancel()
				delete(cm.inflight, prevID)
			}
		}
		cm.byType[req.Type] = req.ID
	}

	// Create new cancellable context
	ctx, cancel := context.WithCancel(parentCtx)
	cm.inflight[req.ID] = cancel

	return ctx
}

// Cancel cancels a specific request by ID
// Returns true if the request was found and cancelled
func (cm *CancellationManager) Cancel(requestID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cancel, ok := cm.inflight[requestID]; ok {
		cancel()
		delete(cm.inflight, requestID)
		return true
	}
	return false
}

// Complete marks a request as complete and removes it from tracking
func (cm *CancellationManager) Complete(requestID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.inflight, requestID)

	// Clean up byType if this was the tracked request
	for reqType, id := range cm.byType {
		if id == requestID {
			delete(cm.byType, reqType)
		}
	}
}

// CancelAll cancels all in-flight requests
func (cm *CancellationManager) CancelAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, cancel := range cm.inflight {
		cancel()
		delete(cm.inflight, id)
	}
	cm.byType = make(map[protocol.RequestType]string)
}

// Count returns the number of in-flight requests
func (cm *CancellationManager) Count() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.inflight)
}

// IsInflight checks if a request is currently in-flight
func (cm *CancellationManager) IsInflight(requestID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	_, ok := cm.inflight[requestID]
	return ok
}
