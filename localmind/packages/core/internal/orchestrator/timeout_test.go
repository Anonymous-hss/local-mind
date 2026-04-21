package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/localmind/core/pkg/protocol"
)

func TestTimeoutManager_WrapContext(t *testing.T) {
	tm := NewTimeoutManager(nil)

	// Test completion budget
	ctx, cancel := tm.WrapContext(context.Background(), protocol.RequestTypeCompletion)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("Context should have deadline")
	}

	expectedBudget := 3000 * time.Millisecond
	actualBudget := time.Until(deadline)

	// Allow some tolerance
	if actualBudget > expectedBudget+10*time.Millisecond {
		t.Errorf("Completion budget too long: %v (expected ~%v)", actualBudget, expectedBudget)
	}
}

func TestTimeoutManager_TimeoutEnforcement(t *testing.T) {
	tm := NewTimeoutManager(&TimeoutConfig{
		Budgets: map[protocol.RequestType]time.Duration{
			protocol.RequestTypeCompletion: 50 * time.Millisecond,
		},
		DefaultTimeout: 1 * time.Second,
	})

	ctx, cancel := tm.WrapContext(context.Background(), protocol.RequestTypeCompletion)
	defer cancel()

	// Simulate work that exceeds budget
	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("Should have timed out before 200ms")
	case <-ctx.Done():
		if !IsTimeout(ctx.Err()) {
			t.Errorf("Expected timeout error, got %v", ctx.Err())
		}
	}
}

func TestTimeoutManager_DefaultTimeout(t *testing.T) {
	tm := NewTimeoutManager(&TimeoutConfig{
		Budgets:        map[protocol.RequestType]time.Duration{},
		DefaultTimeout: 100 * time.Millisecond,
	})

	budget := tm.GetBudget("unknown_type")
	if budget != 100*time.Millisecond {
		t.Errorf("Expected default timeout, got %v", budget)
	}
}

func TestIsTimeout(t *testing.T) {
	if IsTimeout(nil) {
		t.Error("nil should not be timeout")
	}
	if !IsTimeout(context.DeadlineExceeded) {
		t.Error("DeadlineExceeded should be timeout")
	}
	if IsTimeout(context.Canceled) {
		t.Error("Canceled should not be timeout")
	}
}

func TestIsCancelled(t *testing.T) {
	if IsCancelled(nil) {
		t.Error("nil should not be cancelled")
	}
	if IsCancelled(context.DeadlineExceeded) {
		t.Error("DeadlineExceeded should not be cancelled")
	}
	if !IsCancelled(context.Canceled) {
		t.Error("Canceled should be cancelled")
	}
}
