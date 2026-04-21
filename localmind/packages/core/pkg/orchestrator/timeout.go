package orchestrator

import (
	"context"
	"time"

	"github.com/localmind/core/pkg/protocol"
)

// TimeoutConfig holds timeout settings
type TimeoutConfig struct {
	// Budgets maps request type to timeout duration
	Budgets map[protocol.RequestType]time.Duration

	// DefaultTimeout is used for request types not in Budgets
	DefaultTimeout time.Duration
}

// DefaultTimeoutConfig returns the default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		Budgets: map[protocol.RequestType]time.Duration{
			// Completions: 10s budget — first request triggers model load (~3-5s cold start)
			// Subsequent requests are sub-second thanks to Ollama's model caching.
			protocol.RequestTypeCompletion: 10 * time.Second,
			protocol.RequestTypeSuggestion: 15 * time.Second,
			protocol.RequestTypePing:       2 * time.Second,
			protocol.RequestTypeHandshake:  5 * time.Second,
			// Agent has no default timeout - task-dependent
		},
		DefaultTimeout: 30 * time.Second,
	}
}

// TimeoutManager enforces latency budgets on requests
type TimeoutManager struct {
	config *TimeoutConfig
}

// NewTimeoutManager creates a new timeout manager with the given config
func NewTimeoutManager(config *TimeoutConfig) *TimeoutManager {
	if config == nil {
		config = DefaultTimeoutConfig()
	}
	return &TimeoutManager{config: config}
}

// WrapContext wraps a context with the appropriate timeout for the request type
// Returns the wrapped context and a cancel function that MUST be called
func (tm *TimeoutManager) WrapContext(ctx context.Context, reqType protocol.RequestType) (context.Context, context.CancelFunc) {
	budget, ok := tm.config.Budgets[reqType]
	if !ok {
		budget = tm.config.DefaultTimeout
	}

	return context.WithTimeout(ctx, budget)
}

// GetBudget returns the timeout budget for a request type
func (tm *TimeoutManager) GetBudget(reqType protocol.RequestType) time.Duration {
	if budget, ok := tm.config.Budgets[reqType]; ok {
		return budget
	}
	return tm.config.DefaultTimeout
}

// IsTimeout checks if an error is a timeout error
func IsTimeout(err error) bool {
	return err == context.DeadlineExceeded
}

// IsCancelled checks if an error is a cancellation error
func IsCancelled(err error) bool {
	return err == context.Canceled
}
