package ipc

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/localmind/core/pkg/protocol"
)

func TestReader_Read(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    *protocol.Request
		wantErr bool
	}{
		{
			name:  "valid ping request",
			input: createMessage(t, `{"id":"test-1","timestamp":1234567890,"type":"ping"}`),
			want: &protocol.Request{
				ID:        "test-1",
				Timestamp: 1234567890,
				Type:      protocol.RequestTypePing,
			},
		},
		{
			name:  "valid completion request",
			input: createMessage(t, `{"id":"test-2","timestamp":1234567890,"type":"completion","payload":{"prefix":"func main()","language":"go"}}`),
			want: &protocol.Request{
				ID:        "test-2",
				Timestamp: 1234567890,
				Type:      protocol.RequestTypeCompletion,
			},
		},
		{
			name:    "invalid JSON",
			input:   createMessage(t, `{invalid json`),
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewReader(bytes.NewReader(tt.input))
			got, err := reader.Read(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.want != nil {
				if got.ID != tt.want.ID {
					t.Errorf("Read() ID = %v, want %v", got.ID, tt.want.ID)
				}
				if got.Type != tt.want.Type {
					t.Errorf("Read() Type = %v, want %v", got.Type, tt.want.Type)
				}
			}
		})
	}
}

func TestReader_ReadMultiple(t *testing.T) {
	// Create multiple messages in sequence
	msg1 := createMessage(t, `{"id":"1","timestamp":1,"type":"ping"}`)
	msg2 := createMessage(t, `{"id":"2","timestamp":2,"type":"ping"}`)
	msg3 := createMessage(t, `{"id":"3","timestamp":3,"type":"ping"}`)

	combined := append(msg1, msg2...)
	combined = append(combined, msg3...)

	reader := NewReader(bytes.NewReader(combined))

	for i := 1; i <= 3; i++ {
		req, err := reader.Read(context.Background())
		if err != nil {
			t.Fatalf("Read() error on message %d: %v", i, err)
		}
		expectedID := string('0' + rune(i))
		if req.ID != expectedID {
			t.Errorf("Read() message %d: ID = %v, want %v", i, req.ID, expectedID)
		}
	}
}

func TestReader_ContextCancellation(t *testing.T) {
	// Create a reader with no data (will block)
	reader := NewReader(bytes.NewReader(nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := reader.Read(ctx)
	if err == nil {
		t.Error("Read() should return error when context is cancelled")
	}
}

// createMessage creates a length-prefixed message for testing
func createMessage(t *testing.T, jsonStr string) []byte {
	t.Helper()
	payload := []byte(jsonStr)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(payload)))
	return append(length, payload...)
}
