package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/localmind/core/pkg/ollama"
	"github.com/localmind/core/pkg/protocol"
)

// CompletionEngine handles code completion requests
type CompletionEngine struct {
	client *ollama.Client
	cache  *CompletionCache
	config *CompletionConfig
}

// CompletionConfig holds completion engine configuration
type CompletionConfig struct {
	Model          string
	MaxTokens      int
	Temperature    float64
	MaxPrefixChars int
	MaxSuffixChars int
	StopTokens     []string
	UseCache       bool
}

// DefaultCompletionConfig returns default configuration
func DefaultCompletionConfig() *CompletionConfig {
	return &CompletionConfig{
		Model:          "qwen2.5-coder:1.5b",
		MaxTokens:      50,
		Temperature:    0.1,
		MaxPrefixChars: 1500, // ~400 tokens
		MaxSuffixChars: 500,  // ~100 tokens
		StopTokens:     []string{"\n\n", "```", "<|fim", "<|end"},
		UseCache:       true,
	}
}

// NewCompletionEngine creates a new completion engine
func NewCompletionEngine() *CompletionEngine {
	return &CompletionEngine{
		client: ollama.NewClient(nil),
		cache:  DefaultCompletionCache(),
		config: DefaultCompletionConfig(),
	}
}

// NewCompletionEngineWithConfig creates a completion engine with custom config
func NewCompletionEngineWithConfig(client *ollama.Client, config *CompletionConfig) *CompletionEngine {
	if config == nil {
		config = DefaultCompletionConfig()
	}
	return &CompletionEngine{
		client: client,
		cache:  DefaultCompletionCache(),
		config: config,
	}
}

// Type returns the request type this engine handles
func (e *CompletionEngine) Type() protocol.RequestType {
	return protocol.RequestTypeCompletion
}

// Execute processes a completion request
func (e *CompletionEngine) Execute(ctx context.Context, req *protocol.Request) (*Result, error) {
	startTime := time.Now()

	// Parse payload
	var payload protocol.CompletionPayload
	if req.Payload != nil {
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return nil, fmt.Errorf("invalid completion payload: %w", err)
		}
	}

	if payload.Prefix == "" {
		return &Result{Content: "", Model: e.config.Model}, nil
	}

	// Extract context (prefix only, limited)
	prefix := e.extractPrefix(payload.Prefix)
	suffix := e.extractSuffix(payload.Suffix)
	language := payload.Language

	// Check cache first
	if e.config.UseCache {
		if cached, ok := e.cache.Get(prefix, language); ok {
			return &Result{
				Content: cached.Completion,
				Model:   cached.Model + " (cached)",
			}, nil
		}
	}

	// Build prompt
	prompt := e.buildPrompt(prefix, suffix, language)

	// Determine max tokens
	maxTokens := e.config.MaxTokens
	if payload.MaxTokens > 0 && payload.MaxTokens < maxTokens {
		maxTokens = payload.MaxTokens
	}

	// Create Ollama request
	ollamaReq := &ollama.GenerateRequest{
		Model:  e.config.Model,
		Prompt: prompt,
		Raw:    true, // Raw mode for code completion
		Options: &ollama.ModelOptions{
			NumPredict:  maxTokens,
			Temperature: e.config.Temperature,
			Stop:        e.config.StopTokens,
		},
	}

	// Track tokens for streaming
	var completion strings.Builder
	var firstTokenTime time.Duration

	// Execute with streaming
	_, err := e.client.Generate(ctx, ollamaReq, func(token string, done bool) error {
		if firstTokenTime == 0 && token != "" {
			firstTokenTime = time.Since(startTime)
		}
		completion.WriteString(token)
		return nil
	})

	if err != nil {
		// On error, return empty completion (don't break UX)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return &Result{Content: "", Model: e.config.Model}, nil
	}

	result := e.cleanCompletion(completion.String())

	// Cache the result
	if e.config.UseCache && result != "" {
		e.cache.Set(prefix, language, result, e.config.Model)
	}

	return &Result{
		Content: result,
		Model:   e.config.Model,
	}, nil
}

// extractPrefix extracts and limits the prefix
func (e *CompletionEngine) extractPrefix(prefix string) string {
	if len(prefix) > e.config.MaxPrefixChars {
		return prefix[len(prefix)-e.config.MaxPrefixChars:]
	}
	return prefix
}

// extractSuffix extracts and limits the suffix
func (e *CompletionEngine) extractSuffix(suffix string) string {
	if len(suffix) > e.config.MaxSuffixChars {
		return suffix[:e.config.MaxSuffixChars]
	}
	return suffix
}

// buildPrompt creates the prompt for code completion
func (e *CompletionEngine) buildPrompt(prefix, suffix, language string) string {
	// Use FIM (Fill-in-Middle) format for code models
	// This format is supported by most code completion models
	if suffix != "" {
		return fmt.Sprintf("<|fim_prefix|>%s<|fim_suffix|>%s<|fim_middle|>", prefix, suffix)
	}

	// For simple completion without suffix, just use prefix
	// The model will continue from where prefix ends
	return prefix
}

// cleanCompletion removes artifacts from the completion
func (e *CompletionEngine) cleanCompletion(completion string) string {
	// Remove common artifacts
	completion = strings.TrimPrefix(completion, "<|fim_middle|>")
	completion = strings.TrimSuffix(completion, "<|fim_end|>")

	// Remove trailing special tokens
	for _, stop := range e.config.StopTokens {
		if idx := strings.Index(completion, stop); idx >= 0 {
			completion = completion[:idx]
		}
	}

	// Trim trailing whitespace but preserve leading
	completion = strings.TrimRight(completion, " \t\n\r")

	return completion
}

// SetModel updates the model used for completion
func (e *CompletionEngine) SetModel(model string) {
	e.config.Model = model
}

// ClearCache clears the completion cache
func (e *CompletionEngine) ClearCache() {
	e.cache.Clear()
}

// CacheSize returns the current cache size
func (e *CompletionEngine) CacheSize() int {
	return e.cache.Size()
}
