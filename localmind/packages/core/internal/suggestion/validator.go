package suggestion

import (
	"context"
	"fmt"

	"github.com/localmind/core/internal/ast"
)

// Validator validates suggestions before they can be applied
type Validator struct {
	parser *ast.Parser
}

// NewValidator creates a new suggestion validator
func NewValidator() *Validator {
	return &Validator{
		parser: ast.NewParser(),
	}
}

// ValidationResult contains the result of validating a suggestion
type ValidationResult struct {
	Valid       bool     `json:"valid"`
	Errors      []string `json:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	ParsesClean bool     `json:"parsesClean"`
}

// Validate validates a suggestion before apply
func (v *Validator) Validate(ctx context.Context, sugg *Suggestion, originalCode string) (*ValidationResult, error) {
	result := &ValidationResult{Valid: true}

	// Must have diff
	if sugg.Diff == nil || len(sugg.Diff.Hunks) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "No diff provided")
		return result, nil
	}

	// Must have explanation
	if sugg.Explanation == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Explanation is required")
		return result, nil
	}

	// Validate each hunk
	for i, hunk := range sugg.Diff.Hunks {
		// Check that new code parses
		parseResult, err := v.parser.Parse(ctx, sugg.SourceFile, []byte(hunk.After))
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("Hunk %d: Failed to parse: %v", i+1, err))
			continue
		}

		// Check for parse errors
		if len(parseResult.Errors) > 0 {
			result.Valid = false
			result.ParsesClean = false
			for _, e := range parseResult.Errors {
				result.Errors = append(result.Errors, fmt.Sprintf("Hunk %d: Parse error at line %d: %s", 
					i+1, e.Location.StartLine, e.Message))
			}
		} else {
			result.ParsesClean = true
		}
	}

	// Check for destructive changes
	warnings := v.checkDestructiveChanges(sugg)
	result.Warnings = append(result.Warnings, warnings...)

	return result, nil
}

// checkDestructiveChanges detects potentially dangerous changes
func (v *Validator) checkDestructiveChanges(sugg *Suggestion) []string {
	var warnings []string

	if sugg.Diff == nil {
		return warnings
	}

	for _, hunk := range sugg.Diff.Hunks {
		// Check if deleting significantly more than adding
		beforeLines := countLines(hunk.Before)
		afterLines := countLines(hunk.After)

		if beforeLines > 0 && afterLines == 0 {
			warnings = append(warnings, "This change deletes code without replacement")
		}

		if beforeLines > afterLines*2 && beforeLines > 10 {
			warnings = append(warnings, fmt.Sprintf("This change removes %d lines, only adding %d", 
				beforeLines, afterLines))
		}
	}

	return warnings
}

// countLines counts lines in a string
func countLines(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			count++
		}
	}
	return count
}

// ValidateAndMark validates and updates the suggestion
func (v *Validator) ValidateAndMark(ctx context.Context, sugg *Suggestion, originalCode string) error {
	result, err := v.Validate(ctx, sugg, originalCode)
	if err != nil {
		return err
	}

	sugg.Validated = result.Valid
	if !result.Valid && len(result.Errors) > 0 {
		sugg.ValidationError = result.Errors[0]
	}

	return nil
}

// ApplyDiff applies a diff to the original code
func ApplyDiff(original string, diff *Diff) (string, error) {
	if diff == nil || len(diff.Hunks) == 0 {
		return original, nil
	}

	lines := splitLines(original)
	
	// Apply hunks in reverse order to preserve line numbers
	for i := len(diff.Hunks) - 1; i >= 0; i-- {
		hunk := diff.Hunks[i]
		
		// Validate line numbers
		if hunk.StartLineOld < 1 || hunk.EndLineOld > len(lines) {
			return "", fmt.Errorf("invalid line range: %d-%d", hunk.StartLineOld, hunk.EndLineOld)
		}

		// Replace lines
		newLines := splitLines(hunk.After)
		
		// Create new slice with the replacement
		result := make([]string, 0, len(lines)-hunk.EndLineOld+hunk.StartLineOld+len(newLines))
		result = append(result, lines[:hunk.StartLineOld-1]...)
		result = append(result, newLines...)
		result = append(result, lines[hunk.EndLineOld:]...)
		
		lines = result
	}

	return joinLines(lines), nil
}

// joinLines joins lines with newlines
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}

// PreviewApply generates a preview of what the code will look like after applying
func PreviewApply(original string, diff *Diff) (string, error) {
	return ApplyDiff(original, diff)
}

// DiffStats returns statistics about a diff
type DiffStats struct {
	LinesAdded   int `json:"linesAdded"`
	LinesRemoved int `json:"linesRemoved"`
	HunkCount    int `json:"hunkCount"`
}

// GetStats returns statistics about a diff
func (d *Diff) GetStats() DiffStats {
	stats := DiffStats{
		HunkCount: len(d.Hunks),
	}

	for _, hunk := range d.Hunks {
		stats.LinesRemoved += countLines(hunk.Before)
		stats.LinesAdded += countLines(hunk.After)
	}

	return stats
}
