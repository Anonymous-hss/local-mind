package orchestrator

import (
	"context"
	"testing"
	"time"
)

// TestPanicRecovery tests that panics are recovered and don't crash the engine
func TestPanicRecovery(t *testing.T) {
	t.Run("SafeExecute recovers panics", func(t *testing.T) {
		err := SafeExecute(func() error {
			panic("intentional panic for test")
		})

		if err == nil {
			t.Fatal("Expected error from panic recovery, got nil")
		}

		if err.Error() != "panic recovered: intentional panic for test" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("SafeExecute passes through normal errors", func(t *testing.T) {
		expectedErr := context.DeadlineExceeded
		err := SafeExecute(func() error {
			return expectedErr
		})

		if err != expectedErr {
			t.Errorf("Expected %v, got %v", expectedErr, err)
		}
	})

	t.Run("SafeExecute returns nil on success", func(t *testing.T) {
		err := SafeExecute(func() error {
			return nil
		})

		if err != nil {
			t.Errorf("Expected nil, got %v", err)
		}
	})

	t.Run("SafeExecuteWithResult returns result on success", func(t *testing.T) {
		result, err := SafeExecuteWithResult(func() (int, error) {
			return 42, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != 42 {
			t.Errorf("Expected 42, got %d", result)
		}
	})

	t.Run("SafeExecuteWithResult recovers panic", func(t *testing.T) {
		result, err := SafeExecuteWithResult(func() (int, error) {
			panic("panic in result function")
		})

		if err == nil {
			t.Fatal("Expected error from panic recovery")
		}
		if result != 0 {
			t.Errorf("Expected zero value, got %d", result)
		}
	})
}

// TestGracefulShutdown tests the graceful shutdown manager
func TestGracefulShutdown(t *testing.T) {
	t.Run("tracks pending requests", func(t *testing.T) {
		gs := NewGracefulShutdown(1 * time.Second)

		if gs.PendingCount() != 0 {
			t.Errorf("Expected 0 pending, got %d", gs.PendingCount())
		}

		gs.TrackRequest()
		if gs.PendingCount() != 1 {
			t.Errorf("Expected 1 pending, got %d", gs.PendingCount())
		}

		gs.TrackRequest()
		if gs.PendingCount() != 2 {
			t.Errorf("Expected 2 pending, got %d", gs.PendingCount())
		}

		gs.CompleteRequest()
		if gs.PendingCount() != 1 {
			t.Errorf("Expected 1 pending, got %d", gs.PendingCount())
		}

		gs.CompleteRequest()
		if gs.PendingCount() != 0 {
			t.Errorf("Expected 0 pending, got %d", gs.PendingCount())
		}
	})

	t.Run("drain completes when no pending requests", func(t *testing.T) {
		gs := NewGracefulShutdown(100 * time.Millisecond)
		ctx := context.Background()

		err := gs.WaitForDrain(ctx)
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})

	t.Run("drain waits for pending requests", func(t *testing.T) {
		gs := NewGracefulShutdown(500 * time.Millisecond)
		ctx := context.Background()

		gs.TrackRequest()

		// Complete request after 100ms
		go func() {
			time.Sleep(100 * time.Millisecond)
			gs.CompleteRequest()
		}()

		start := time.Now()
		err := gs.WaitForDrain(ctx)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}

		if elapsed < 100*time.Millisecond {
			t.Errorf("Drain completed too early: %v", elapsed)
		}
	})
}

// TestMustNotPanic tests the MustNotPanic helper
func TestMustNotPanic(t *testing.T) {
	t.Run("catches panic without crashing", func(t *testing.T) {
		// This should not panic the test
		MustNotPanic(func() {
			panic("test panic")
		})

		// If we get here, the panic was caught
		t.Log("Panic was successfully caught by MustNotPanic")
	})

	t.Run("normal function executes", func(t *testing.T) {
		executed := false
		MustNotPanic(func() {
			executed = true
		})

		if !executed {
			t.Error("Function was not executed")
		}
	})
}
