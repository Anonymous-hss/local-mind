package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/localmind/core/pkg/protocol"
)

func TestSuggestionEngine_Execute(t *testing.T) {
	e := NewSuggestionEngine()
	e.SimulatedDelay = 1 * time.Millisecond // Fast test

	payload := protocol.SuggestionPayload{
		Content:        "func Foo() {}",
		Language:       "go",
		SuggestionType: "refactor",
		File:           "test.go",
		StartLine:      1,
		EndLine:        1,
	}
	payloadBytes, _ := json.Marshal(payload)

	req := &protocol.Request{
		ID:        "test-id",
		Timestamp: time.Now().Unix(),
		Type:      protocol.RequestTypeSuggestion,
		Payload:   payloadBytes,
	}

	result, err := e.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Model != "stub-suggestion-model" {
		t.Errorf("Expected stub model, got %s", result.Model)
	}

	if !strings.Contains(result.Content, "[STUB] refactor suggestion") {
		t.Errorf("Expected stub content, got %s", result.Content)
	}
}
