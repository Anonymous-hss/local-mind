package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// =============================================================================
// Graceful Shutdown
// =============================================================================

// GracefulShutdown manages graceful shutdown with pending request draining
type GracefulShutdown struct {
	mu             sync.Mutex
	pendingCount   int
	shutdownSignal chan struct{}
	drainTimeout   time.Duration
}

// NewGracefulShutdown creates a new graceful shutdown manager
func NewGracefulShutdown(drainTimeout time.Duration) *GracefulShutdown {
	return &GracefulShutdown{
		shutdownSignal: make(chan struct{}),
		drainTimeout:   drainTimeout,
	}
}

// TrackRequest increments pending request count
func (gs *GracefulShutdown) TrackRequest() {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.pendingCount++
}

// CompleteRequest decrements pending request count
func (gs *GracefulShutdown) CompleteRequest() {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.pendingCount--
}

// PendingCount returns current pending request count
func (gs *GracefulShutdown) PendingCount() int {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.pendingCount
}

// WaitForDrain waits for all pending requests to complete or timeout
func (gs *GracefulShutdown) WaitForDrain(ctx context.Context) error {
	deadline := time.After(gs.drainTimeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			pending := gs.PendingCount()
			if pending > 0 {
				return fmt.Errorf("drain timeout with %d pending requests", pending)
			}
			return nil
		case <-ticker.C:
			if gs.PendingCount() == 0 {
				return nil
			}
		}
	}
}

// =============================================================================
// Signal Handlers
// =============================================================================

// SetupSignalHandler sets up OS signal handling for graceful shutdown
func SetupSignalHandler(ctx context.Context, shutdown func()) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-ctx.Done():
			return
		case sig := <-sigChan:
			_ = sig // silence unused warning
			shutdown()
			cancel()
		}
	}()

	return ctx
}

// =============================================================================
// Error Recovery Helpers
// =============================================================================

// SafeExecute wraps a function with panic recovery
func SafeExecute(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()
	return fn()
}

// SafeExecuteWithResult wraps a function with panic recovery and returns result
func SafeExecuteWithResult[T any](fn func() (T, error)) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()
	return fn()
}

// MustNotPanic ensures a function doesn't panic, logs if it does
func MustNotPanic(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			// Log but don't crash
			fmt.Fprintf(os.Stderr, "[CRITICAL] Panic in MustNotPanic: %v\n", r)
		}
	}()
	fn()
}

// =============================================================================
// No Silent Failures
// =============================================================================

// ErrorLogger ensures all errors are logged (no silent failures)
type ErrorLogger struct {
	logger interface {
		Printf(format string, v ...interface{})
	}
	prefix string
}

// NewErrorLogger creates a new error logger
func NewErrorLogger(logger interface {
	Printf(format string, v ...interface{})
}, prefix string) *ErrorLogger {
	return &ErrorLogger{
		logger: logger,
		prefix: prefix,
	}
}

// LogError logs an error with context (no silent failures)
func (e *ErrorLogger) LogError(action string, err error) {
	if err != nil {
		e.logger.Printf("%s ERROR [%s]: %v", e.prefix, action, err)
	}
}

// LogAndReturn logs an error and returns it
func (e *ErrorLogger) LogAndReturn(action string, err error) error {
	if err != nil {
		e.LogError(action, err)
	}
	return err
}
