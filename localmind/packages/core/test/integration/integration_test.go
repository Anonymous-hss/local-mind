package integration

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/localmind/core/internal/orchestrator"
	"github.com/localmind/core/pkg/protocol"
)

// MockOllamaHandler simulates Ollama API behavior
func MockOllamaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Generate response
		if r.URL.Path == "/api/generate" {
			w.Header().Set("Content-Type", "application/json")
			// Return a simple completion
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response": "fmt.Println(\"Hello Integrated World\")",
				"done":     true,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func TestCoreIntegration_FullRoundTrip(t *testing.T) {
	// 1. Setup Mock Ollama
	server := httptest.NewServer(MockOllamaHandler())
	defer server.Close()

	// 2. Setup Pipes for IPC
	// Input: Test -> Orchestrator
	inReader, inWriter := io.Pipe()
	// Output: Orchestrator -> Test
	outReader, outWriter := io.Pipe()

	// 3. Setup Orchestrator
	config := orchestrator.DefaultConfig()
	config.OllamaURL = server.URL
	config.Input = inReader
	config.Output = outWriter

	orch := orchestrator.New(config)

	// 4. Run Orchestrator in Goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error)
	go func() {
		done <- orch.Run(ctx)
	}()

	// 5. Send Request
	payload := protocol.CompletionPayload{
		Prefix:   "package main\nfunc main() {\n",
		Language: "go",
	}
	payloadBytes, _ := json.Marshal(payload)

	reqID := "integration-req-1"
	req := &protocol.Request{
		ID:        reqID,
		Timestamp: time.Now().Unix(),
		Type:      protocol.RequestTypeCompletion,
		Payload:   payloadBytes,
	}

	// Write Request to Pipe (Length-Prefixed)
	reqJson, _ := json.Marshal(req)
	length := uint32(len(reqJson))

	// Write length (Big Endian)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, length)

	_, err := inWriter.Write(lenBuf)
	if err != nil {
		t.Fatalf("Failed to write length: %v", err)
	}
	_, err = inWriter.Write(reqJson)
	if err != nil {
		t.Fatalf("Failed to write json: %v", err)
	}

	// 6. Read Response from Pipe
	// We read length then JSON
	respLenBuf := make([]byte, 4)
	if _, err := io.ReadFull(outReader, respLenBuf); err != nil {
		t.Fatalf("Failed to read response length: %v", err)
	}

	respLen := binary.BigEndian.Uint32(respLenBuf)
	respJson := make([]byte, respLen)
	if _, err := io.ReadFull(outReader, respJson); err != nil {
		t.Fatalf("Failed to read response json: %v", err)
	}

	// 7. Verify Response
	var resp protocol.Response
	if err := json.Unmarshal(respJson, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.RequestID != reqID {
		t.Errorf("Response RequestID mismatch: got %s, want %s", resp.RequestID, reqID)
	}
	if resp.Type != protocol.ResponseTypeResult {
		t.Errorf("Response Type mismatch: got %s, want %s", resp.Type, protocol.ResponseTypeResult)
	}

	// Verify Payload
	var resPayload protocol.ResultPayload
	json.Unmarshal(resp.Payload, &resPayload)

	expectedContent := "fmt.Println(\"Hello Integrated World\")"
	if resPayload.Content != expectedContent {
		t.Errorf("Completion Content mismatch: got %q, want %q", resPayload.Content, expectedContent)
	}

	// Cleanup
	cancel()
	inWriter.Close() // Signal EOF to orchestrator if it reads fully
	<-done
}
