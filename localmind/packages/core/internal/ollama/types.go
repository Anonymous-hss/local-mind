// Package ollama provides a client for the Ollama API.
package ollama

import "time"

// GenerateRequest represents a request to the /api/generate endpoint
type GenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options *ModelOptions  `json:"options,omitempty"`
	Context []int          `json:"context,omitempty"`
	Raw     bool           `json:"raw,omitempty"`
}

// ModelOptions contains generation parameters
type ModelOptions struct {
	NumPredict   int     `json:"num_predict,omitempty"`   // Max tokens to generate
	Temperature  float64 `json:"temperature,omitempty"`   // 0.0 = deterministic
	TopK         int     `json:"top_k,omitempty"`
	TopP         float64 `json:"top_p,omitempty"`
	Stop         []string `json:"stop,omitempty"`          // Stop sequences
	NumCtx       int     `json:"num_ctx,omitempty"`       // Context window size
}

// GenerateResponse represents a response from the /api/generate endpoint
type GenerateResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	Context            []int     `json:"context,omitempty"`
	TotalDuration      int64     `json:"total_duration,omitempty"`
	LoadDuration       int64     `json:"load_duration,omitempty"`
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
	EvalCount          int       `json:"eval_count,omitempty"`
	EvalDuration       int64     `json:"eval_duration,omitempty"`
}

// ModelInfo represents information about an available model
type ModelInfo struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
}

// ListModelsResponse represents the response from /api/tags
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// VersionResponse represents the response from /api/version
type VersionResponse struct {
	Version string `json:"version"`
}

// CompletionOptions are options specific to code completion
type CompletionOptions struct {
	MaxTokens   int      // Maximum tokens to generate (default: 50)
	Temperature float64  // Temperature (default: 0.1 for code)
	StopTokens  []string // Stop generation on these tokens
}

// DefaultCompletionOptions returns sensible defaults for code completion
func DefaultCompletionOptions() *CompletionOptions {
	return &CompletionOptions{
		MaxTokens:   50,
		Temperature: 0.1,
		StopTokens:  []string{"\n\n", "```", "<|fim"},
	}
}
