package engine

import (
	"context"
)

// StreamWriter defines the interface for streaming response chunks
type StreamWriter interface {
	WriteChunk(requestID, chunk string) error
}

type streamWriterKey struct{}

// WithStreamWriter adds a stream writer to the context
func WithStreamWriter(ctx context.Context, w StreamWriter) context.Context {
	return context.WithValue(ctx, streamWriterKey{}, w)
}

// GetStreamWriter retrieves a stream writer from the context
func GetStreamWriter(ctx context.Context) StreamWriter {
	if w, ok := ctx.Value(streamWriterKey{}).(StreamWriter); ok {
		return w
	}
	return nil
}
