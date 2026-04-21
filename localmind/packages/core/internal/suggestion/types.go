// Package suggestion provides code suggestion and refactoring capabilities.
package suggestion

import (
	"time"
)

// SuggestionType represents the type of suggestion
type SuggestionType string

const (
	SuggestionTypeRefactor    SuggestionType = "refactor"
	SuggestionTypeSmell       SuggestionType = "smell"
	SuggestionTypePerformance SuggestionType = "performance"
	SuggestionTypeConvention  SuggestionType = "convention"
	SuggestionTypeBestPractice SuggestionType = "best_practice"
)

// Confidence represents confidence in a suggestion
type Confidence float64

const (
	ConfidenceLow    Confidence = 0.3
	ConfidenceMedium Confidence = 0.6
	ConfidenceHigh   Confidence = 0.8
	ConfidenceCertain Confidence = 1.0
)

// Risk represents the risk level of a suggestion
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// Suggestion represents a code improvement suggestion
type Suggestion struct {
	ID          string         `json:"id"`
	Type        SuggestionType `json:"type"`
	Title       string         `json:"title"`
	Explanation string         `json:"explanation"`
	Confidence  Confidence     `json:"confidence"`
	Risk        Risk           `json:"risk"`
	Diff        *Diff          `json:"diff"`
	RiskDetails []string       `json:"riskDetails,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	
	// Metadata
	SourceFile string `json:"sourceFile"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	
	// Validation status
	Validated   bool   `json:"validated"`
	ValidatedAt time.Time `json:"validatedAt,omitempty"`
	ValidationError string `json:"validationError,omitempty"`
}

// Diff represents a unified diff for a single file
type Diff struct {
	File     string `json:"file"`
	Language string `json:"language"`
	Hunks    []Hunk `json:"hunks"`
}

// Hunk represents a single change in a diff
type Hunk struct {
	StartLineOld int    `json:"startLineOld"`
	EndLineOld   int    `json:"endLineOld"`
	StartLineNew int    `json:"startLineNew"`
	EndLineNew   int    `json:"endLineNew"`
	Before       string `json:"before"`
	After        string `json:"after"`
}

// ToUnifiedDiff converts the structured diff to unified diff format
func (d *Diff) ToUnifiedDiff() string {
	var result string
	result += "--- a/" + d.File + "\n"
	result += "+++ b/" + d.File + "\n"
	
	for _, hunk := range d.Hunks {
		result += hunk.ToUnifiedFormat()
	}
	
	return result
}

// ToUnifiedFormat converts a hunk to unified diff format
func (h *Hunk) ToUnifiedFormat() string {
	oldLines := h.EndLineOld - h.StartLineOld + 1
	newLines := h.EndLineNew - h.StartLineNew + 1
	
	header := "@@ -" + itoa(h.StartLineOld) + "," + itoa(oldLines) + 
		" +" + itoa(h.StartLineNew) + "," + itoa(newLines) + " @@\n"
	
	var body string
	for _, line := range splitLines(h.Before) {
		body += "-" + line + "\n"
	}
	for _, line := range splitLines(h.After) {
		body += "+" + line + "\n"
	}
	
	return header + body
}

// itoa converts int to string (simple implementation)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// SmellType represents types of code smells
type SmellType string

const (
	SmellLongFunction    SmellType = "long_function"
	SmellDeepNesting     SmellType = "deep_nesting"
	SmellDuplicateCode   SmellType = "duplicate_code"
	SmellLargeClass      SmellType = "large_class"
	SmellLongParamList   SmellType = "long_param_list"
	SmellComplexCondition SmellType = "complex_condition"
	SmellMagicNumber     SmellType = "magic_number"
	SmellDeadCode        SmellType = "dead_code"
)

// CodeSmell represents a detected code smell
type CodeSmell struct {
	Type        SmellType `json:"type"`
	File        string    `json:"file"`
	StartLine   int       `json:"startLine"`
	EndLine     int       `json:"endLine"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"` // "info", "warning", "error"
	FixAvailable bool     `json:"fixAvailable"`
}

// SuggestionRequest represents a request for suggestions
type SuggestionRequest struct {
	File        string   `json:"file"`
	StartLine   int      `json:"startLine,omitempty"`
	EndLine     int      `json:"endLine,omitempty"`
	Content     string   `json:"content"`
	Context     string   `json:"context,omitempty"` // Surrounding code
	Intent      string   `json:"intent,omitempty"`  // User's intent if specified
	Types       []SuggestionType `json:"types,omitempty"` // Filter by type
}

// SuggestionResult represents the result of a suggestion request
type SuggestionResult struct {
	Suggestions []Suggestion `json:"suggestions"`
	Smells      []CodeSmell  `json:"smells,omitempty"`
	TotalTime   int64        `json:"totalTimeMs"`
}
