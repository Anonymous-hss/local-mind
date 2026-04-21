package suggestion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/localmind/core/internal/ollama"
	"github.com/localmind/core/pkg/protocol"
)

// Engine is the main suggestion engine
type Engine struct {
	analyzer  *Analyzer
	generator *Generator
	validator *Validator
}

// NewEngine creates a new suggestion engine
func NewEngine(client *ollama.Client, model string) *Engine {
	return &Engine{
		analyzer:  NewAnalyzer(nil),
		generator: NewGenerator(client, model),
		validator: NewValidator(),
	}
}

// Type returns the request type this engine handles
func (e *Engine) Type() protocol.RequestType {
	return protocol.RequestTypeSuggestion
}

// Execute processes a suggestion request
func (e *Engine) Execute(ctx context.Context, req *protocol.Request) (*Result, error) {
	startTime := time.Now()

	// Parse suggestion request from payload
	suggReq, err := parseSuggestionRequest(req)
	if err != nil {
		return nil, err
	}

	var suggestions []Suggestion
	var smells []CodeSmell

	// Step 1: Analyze for code smells (only if full file analysis requested or large enough chunk)
	// For small chunks, we might skip full smell analysis or limit scope
	if len(suggReq.Content) > 0 {
		// Mock smell analysis for now, or use real analyzer if implemented
		// The current analyzer seems to expect file path binding
	}

	// Step 2-3: Generate suggestion
	// If specific type requested, do that
	refactorSugg, err := e.generator.GenerateRefactor(ctx, suggReq)
	if err == nil && refactorSugg != nil {
		suggestions = append(suggestions, *refactorSugg)
	}

	// Step 4: Validate all suggestions
	for i := range suggestions {
		e.validator.ValidateAndMark(ctx, &suggestions[i], suggReq.Content)
	}

	// Filter to only valid suggestions
	var validSuggestions []Suggestion
	for _, s := range suggestions {
		if s.Validated {
			validSuggestions = append(validSuggestions, s)
		}
	}

	// Build result
	result := &SuggestionResult{
		Suggestions: validSuggestions,
		Smells:      smells,
		TotalTime:   time.Since(startTime).Milliseconds(),
	}

	return &Result{
		Suggestions: result,
	}, nil
}

// Result wraps the suggestion result for the engine interface
type Result struct {
	Suggestions *SuggestionResult
}

// parseSuggestionRequest parses the protocol request payload
func parseSuggestionRequest(req *protocol.Request) (*SuggestionRequest, error) {
	var payload protocol.SuggestionPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return nil, fmt.Errorf("invalid suggestion payload: %w", err)
	}

	return &SuggestionRequest{
		File:      payload.File,
		StartLine: payload.StartLine,
		EndLine:   payload.EndLine,
		Content:   payload.Content,
		Context:   payload.Context,
		Types:     []SuggestionType{SuggestionType(payload.SuggestionType)},
	}, nil
}
