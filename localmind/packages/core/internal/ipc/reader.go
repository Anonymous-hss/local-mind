// Package ipc handles inter-process communication between
// the VS Code extension and the LocalMind core engine.
package ipc

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/localmind/core/pkg/protocol"
)

// Reader reads length-prefixed JSON messages from an io.Reader
type Reader struct {
	reader *bufio.Reader
}

// NewReader creates a new IPC reader
func NewReader(r io.Reader) *Reader {
	return &Reader{
		reader: bufio.NewReader(r),
	}
}

// Read reads the next message from the input stream.
// Returns the parsed request or an error.
// Respects context cancellation.
func (r *Reader) Read(ctx context.Context) (*protocol.Request, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Read 4-byte length prefix (big-endian)
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(r.reader, lengthBytes); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to read length prefix: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBytes)

	// Sanity check on message length (max 10MB)
	const maxMessageSize = 10 * 1024 * 1024
	if length > maxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read the JSON payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(r.reader, payload); err != nil {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}

	// Parse JSON into Request
	var req protocol.Request
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("failed to parse request JSON: %w", err)
	}

	return &req, nil
}

// ReadLoop continuously reads messages and sends them to the channel.
// Stops when context is cancelled or EOF is reached.
func (r *Reader) ReadLoop(ctx context.Context, requests chan<- *protocol.Request, errors chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			req, err := r.Read(ctx)
			if err != nil {
				if err == io.EOF {
					return
				}
				if ctx.Err() != nil {
					return
				}
				errors <- err
				continue
			}
			requests <- req
		}
	}
}
