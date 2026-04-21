package ipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/localmind/core/pkg/protocol"
)

// Writer writes length-prefixed JSON messages to an io.Writer
type Writer struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewWriter creates a new IPC writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer: w,
	}
}

// Write sends a response to the output stream.
// Thread-safe: can be called from multiple goroutines.
func (w *Writer) Write(resp *protocol.Response) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Marshal response to JSON
	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Write 4-byte length prefix (big-endian)
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(payload)))

	if _, err := w.writer.Write(lengthBytes); err != nil {
		return fmt.Errorf("failed to write length prefix: %w", err)
	}

	// Write JSON payload
	if _, err := w.writer.Write(payload); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}

	return nil
}

// WriteError writes an error response
func (w *Writer) WriteError(requestID string, code protocol.ErrorCode, message string) error {
	resp := protocol.NewErrorResponse(requestID, code, message)
	return w.Write(&resp)
}

// WriteResult writes a result response
func (w *Writer) WriteResult(requestID string, content string, latencyMs int64, model string) error {
	resp := protocol.NewResultResponse(requestID, content, latencyMs, model)
	return w.Write(&resp)
}

// WriteChunk writes a stream chunk
func (w *Writer) WriteChunk(requestID, chunk string) error {
	payload, err := json.Marshal(protocol.StreamPayload{
		Chunk: chunk,
		Done:  false,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal stream payload: %w", err)
	}

	resp := protocol.Response{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()), // Temporary ID generation
		Timestamp: time.Now().UnixMilli(),
		RequestID: requestID,
		Type:      protocol.ResponseTypeStream,
		Payload:   payload,
	}

	return w.Write(&resp)
}

// WriteCancelled writes a cancelled response
func (w *Writer) WriteCancelled(requestID string) error {
	resp := protocol.NewCancelledResponse(requestID)
	return w.Write(&resp)
}
