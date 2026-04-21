package orchestrator

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/localmind/core/internal/engine"
	"github.com/localmind/core/internal/ipc"
	"github.com/localmind/core/pkg/protocol"
)

// testOrchestrator creates an orchestrator with custom I/O for testing
func testOrchestrator(input *bytes.Buffer, output *bytes.Buffer) *Orchestrator {
	registry := engine.NewRegistry()

	// Register mock/stub engines for testing (stubs have configurable delays)
	registry.Register(&mockCompletionEngine{delay: 10 * time.Millisecond})

	suggestionEng := engine.NewSuggestionEngine()
	suggestionEng.SimulatedDelay = 10 * time.Millisecond
	registry.Register(suggestionEng)

	agentEng := engine.NewAgentEngine()
	registry.Register(agentEng)

	return &Orchestrator{
		reader:       ipc.NewReader(input),
		writer:       ipc.NewWriter(output),
		router:       NewRouter(registry),
		cancellation: NewCancellationManager(),
		timeout:      NewTimeoutManager(nil),
		logger:       log.New(io.Discard, "", 0), // Silent logger for tests
		shutdown:     make(chan struct{}),
	}
}

// mockCompletionEngine is a test-only mock that doesn't require Ollama
type mockCompletionEngine struct {
	delay time.Duration
}

func (m *mockCompletionEngine) Type() protocol.RequestType {
	return protocol.RequestTypeCompletion
}

func (m *mockCompletionEngine) Execute(ctx context.Context, req *protocol.Request) (*engine.Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.delay):
	}
	return &engine.Result{Content: "// mock completion", Model: "mock"}, nil
}

func createTestMessage(t *testing.T, req protocol.Request) []byte {
	t.Helper()
	payload, _ := json.Marshal(req)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(payload)))
	return append(length, payload...)
}

func parseResponse(t *testing.T, data []byte) protocol.Response {
	t.Helper()
	if len(data) < 4 {
		t.Fatal("Response too short")
	}
	length := binary.BigEndian.Uint32(data[:4])
	var resp protocol.Response
	if err := json.Unmarshal(data[4:4+length], &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return resp
}

func TestOrchestrator_Ping(t *testing.T) {
	input := &bytes.Buffer{}
	output := &bytes.Buffer{}
	orch := testOrchestrator(input, output)

	// Write ping request
	req := protocol.Request{
		ID:        "ping-1",
		Timestamp: time.Now().UnixMilli(),
		Type:      protocol.RequestTypePing,
	}
	input.Write(createTestMessage(t, req))

	// Run orchestrator briefly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	orch.Run(ctx)

	// Parse response
	if output.Len() == 0 {
		t.Fatal("No response received")
	}

	resp := parseResponse(t, output.Bytes())
	if resp.Type != protocol.ResponseTypePong {
		t.Errorf("Expected pong, got %s", resp.Type)
	}
	if resp.RequestID != "ping-1" {
		t.Errorf("RequestID mismatch: got %s", resp.RequestID)
	}
}

func TestOrchestrator_InvalidRequestType(t *testing.T) {
	input := &bytes.Buffer{}
	output := &bytes.Buffer{}
	orch := testOrchestrator(input, output)

	// Write request with unknown type
	req := protocol.Request{
		ID:        "invalid-1",
		Timestamp: time.Now().UnixMilli(),
		Type:      "unknown_type",
	}
	input.Write(createTestMessage(t, req))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	orch.Run(ctx)

	if output.Len() == 0 {
		t.Fatal("No response received")
	}

	resp := parseResponse(t, output.Bytes())
	if resp.Type != protocol.ResponseTypeError {
		t.Errorf("Expected error response, got %s", resp.Type)
	}

	var errPayload protocol.ErrorPayload
	json.Unmarshal(resp.Payload, &errPayload)
	if errPayload.Code != protocol.ErrorCodeInvalidRequest {
		t.Errorf("Expected INVALID_REQUEST, got %s", errPayload.Code)
	}
}

func TestOrchestrator_RapidRequestsDebounce(t *testing.T) {
	input := &bytes.Buffer{}
	output := &bytes.Buffer{}
	orch := testOrchestrator(input, output)

	// Send 10 rapid completion requests
	for i := 0; i < 10; i++ {
		req := protocol.Request{
			ID:        string(rune('a' + i)),
			Timestamp: time.Now().UnixMilli(),
			Type:      protocol.RequestTypeCompletion,
			Payload:   json.RawMessage(`{"prefix":"test","language":"go"}`),
		}
		input.Write(createTestMessage(t, req))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	orch.Run(ctx)

	// Count responses
	var resultCount, cancelledCount int
	data := output.Bytes()
	offset := 0

	for offset < len(data) {
		if len(data)-offset < 4 {
			break
		}
		length := binary.BigEndian.Uint32(data[offset : offset+4])
		if len(data)-offset < int(4+length) {
			break
		}
		var resp protocol.Response
		json.Unmarshal(data[offset+4:offset+4+int(length)], &resp)
		offset += 4 + int(length)

		switch resp.Type {
		case protocol.ResponseTypeResult:
			resultCount++
		case protocol.ResponseTypeCancelled:
			cancelledCount++
		}
	}

	// Due to debouncing, most requests should be cancelled
	// Only the last one (or few) should complete
	t.Logf("Results: %d, Cancelled: %d", resultCount, cancelledCount)

	if resultCount == 0 {
		t.Error("At least one request should complete")
	}
	if cancelledCount == 0 && resultCount >= 10 {
		t.Error("Some requests should be cancelled due to debouncing")
	}
}

func TestOrchestrator_ConcurrentRequests(t *testing.T) {
	// Test for deadlocks with concurrent requests
	input := &bytes.Buffer{}
	output := &bytes.Buffer{}
	orch := testOrchestrator(input, output)

	// Create many concurrent agent requests (no debounce)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		req := protocol.Request{
			ID:        string(rune('A' + (i % 26))),
			Timestamp: time.Now().UnixMilli(),
			Type:      protocol.RequestTypeAgent,
			Payload:   json.RawMessage(`{"task":"test","workspaceRoot":"/"}`),
		}
		input.Write(createTestMessage(t, req))
	}

	// Run with timeout - if there's a deadlock, this will fail
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		orch.Run(ctx)
	}()

	wg.Wait()

	// If we got here without hanging, no deadlock
	t.Log("Concurrent requests completed without deadlock")
}
