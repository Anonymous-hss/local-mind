package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/localmind/core/pkg/protocol"
)

func TestCancellationManager_Register(t *testing.T) {
	cm := NewCancellationManager()

	req := &protocol.Request{ID: "test-1", Type: protocol.RequestTypeCompletion}
	ctx := cm.Register(context.Background(), req)

	if ctx == nil {
		t.Fatal("Register should return a context")
	}

	if !cm.IsInflight("test-1") {
		t.Error("Request should be tracked as inflight")
	}
}

func TestCancellationManager_Debounce(t *testing.T) {
	cm := NewCancellationManager()

	// Register first completion request
	req1 := &protocol.Request{ID: "comp-1", Type: protocol.RequestTypeCompletion}
	ctx1 := cm.Register(context.Background(), req1)

	// Register second completion request - should cancel first
	req2 := &protocol.Request{ID: "comp-2", Type: protocol.RequestTypeCompletion}
	_ = cm.Register(context.Background(), req2)

	// First context should be cancelled
	select {
	case <-ctx1.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("First request should be cancelled by debounce")
	}

	// Second should still be tracked
	if !cm.IsInflight("comp-2") {
		t.Error("Second request should be tracked")
	}
}

func TestCancellationManager_Cancel(t *testing.T) {
	cm := NewCancellationManager()

	req := &protocol.Request{ID: "test-1", Type: protocol.RequestTypeCompletion}
	ctx := cm.Register(context.Background(), req)

	cancelled := cm.Cancel("test-1")
	if !cancelled {
		t.Error("Cancel should return true for tracked request")
	}

	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled")
	}

	// Should no longer be tracked
	if cm.IsInflight("test-1") {
		t.Error("Cancelled request should not be inflight")
	}
}

func TestCancellationManager_AgentNoDebounce(t *testing.T) {
	cm := NewCancellationManager()

	// Agent requests should NOT debounce
	req1 := &protocol.Request{ID: "agent-1", Type: protocol.RequestTypeAgent}
	ctx1 := cm.Register(context.Background(), req1)

	req2 := &protocol.Request{ID: "agent-2", Type: protocol.RequestTypeAgent}
	_ = cm.Register(context.Background(), req2)

	// First context should NOT be cancelled
	select {
	case <-ctx1.Done():
		t.Error("Agent requests should not debounce")
	case <-time.After(50 * time.Millisecond):
		// Expected - context still active
	}

	// Both should be tracked
	if !cm.IsInflight("agent-1") || !cm.IsInflight("agent-2") {
		t.Error("Both agent requests should be tracked")
	}
}

func TestCancellationManager_CancelAll(t *testing.T) {
	cm := NewCancellationManager()

	ctxs := make([]context.Context, 5)
	for i := 0; i < 5; i++ {
		req := &protocol.Request{ID: string(rune('a' + i)), Type: protocol.RequestTypeAgent}
		ctxs[i] = cm.Register(context.Background(), req)
	}

	cm.CancelAll()

	for i, ctx := range ctxs {
		select {
		case <-ctx.Done():
			// Expected
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Context %d should be cancelled", i)
		}
	}

	if cm.Count() != 0 {
		t.Error("All requests should be removed")
	}
}
