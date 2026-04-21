package engine

import (
	"testing"
)

func TestCompletionEngine_BuildPrompt(t *testing.T) {
	e := NewCompletionEngine()

	tests := []struct {
		name     string
		prefix   string
		suffix   string
		language string
		want     string
	}{
		{
			name:   "Prefix only",
			prefix: "func main() {",
			suffix: "",
			want:   "func main() {",
		},
		{
			name:   "Prefix and Suffix (FIM)",
			prefix: "func main() {",
			suffix: "}",
			want:   "<|fim_prefix|>func main() {<|fim_suffix|>}<|fim_middle|>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.buildPrompt(tt.prefix, tt.suffix, tt.language)
			if got != tt.want {
				t.Errorf("buildPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompletionEngine_CleanCompletion(t *testing.T) {
	e := NewCompletionEngine()
	e.config.StopTokens = []string{"<|end", "```"}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Clean FIM tokens",
			input: "<|fim_middle|>fmt.Println('Hello')<|fim_end|>",
			want:  "fmt.Println('Hello')",
		},
		{
			name:  "Truncate at Stop Token",
			input: "fmt.Println('Hello')<|end of file|>",
			want:  "fmt.Println('Hello')",
		},
		{
			name:  "Truncate at Code Block",
			input: "fmt.Println('Hello')```python",
			want:  "fmt.Println('Hello')",
		},
		{
			name:  "Trim Trailing Whitespace",
			input: "fmt.Println('Hello')   \n\t",
			want:  "fmt.Println('Hello')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.cleanCompletion(tt.input)
			if got != tt.want {
				t.Errorf("cleanCompletion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompletionEngine_ExtractContext(t *testing.T) {
	e := NewCompletionEngine()
	e.config.MaxPrefixChars = 10
	e.config.MaxSuffixChars = 10

	// Test Prefix
	prefix := "123456789012345"
	gotPrefix := e.extractPrefix(prefix)
	if gotPrefix != "6789012345" {
		t.Errorf("extractPrefix() = %q, want %q", gotPrefix, "6789012345")
	}

	// Test Suffix
	suffix := "123456789012345"
	gotSuffix := e.extractSuffix(suffix)
	if gotSuffix != "1234567890" {
		t.Errorf("extractSuffix() = %q, want %q", gotSuffix, "1234567890")
	}
}
