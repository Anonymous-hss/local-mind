package ipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/localmind/core/pkg/protocol"
)

func TestWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	resp := protocol.NewResultResponse("req-1", "test content", 50, "test-model")

	err := writer.Write(&resp)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Read length prefix
	if buf.Len() < 4 {
		t.Fatal("Output too short")
	}

	lengthBytes := buf.Bytes()[:4]
	length := binary.BigEndian.Uint32(lengthBytes)

	// Verify JSON payload
	payload := buf.Bytes()[4:]
	if uint32(len(payload)) != length {
		t.Errorf("Length mismatch: prefix=%d, actual=%d", length, len(payload))
	}

	var parsed protocol.Response
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if parsed.RequestID != "req-1" {
		t.Errorf("RequestID = %v, want req-1", parsed.RequestID)
	}
	if parsed.Type != protocol.ResponseTypeResult {
		t.Errorf("Type = %v, want result", parsed.Type)
	}
}

func TestWriter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	// Write multiple responses concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			resp := protocol.NewResultResponse("req", "content", 0, "")
			writer.Write(&resp)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 10 valid messages
	// (Just verify no panic occurred - thread safety test)
	if buf.Len() == 0 {
		t.Error("No data written")
	}
}

func TestWriter_WriteError(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	err := writer.WriteError("req-1", protocol.ErrorCodeTimeout, "Request timed out")
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	// Parse and verify
	payload := buf.Bytes()[4:]
	var resp protocol.Response
	json.Unmarshal(payload, &resp)

	if resp.Type != protocol.ResponseTypeError {
		t.Errorf("Type = %v, want error", resp.Type)
	}

	var errorPayload protocol.ErrorPayload
	json.Unmarshal(resp.Payload, &errorPayload)

	if errorPayload.Code != protocol.ErrorCodeTimeout {
		t.Errorf("Error code = %v, want TIMEOUT", errorPayload.Code)
	}
}
