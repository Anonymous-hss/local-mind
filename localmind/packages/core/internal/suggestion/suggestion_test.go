package suggestion

import (
	"context"
	"testing"
)

func TestAnalyzer_DetectLongFunction(t *testing.T) {
	analyzer := NewAnalyzer(&SmellConfig{
		MaxFunctionLines: 10,
	})

	// Create a long function
	code := `package main

func longFunction() {
	line1()
	line2()
	line3()
	line4()
	line5()
	line6()
	line7()
	line8()
	line9()
	line10()
	line11()
	line12()
}
`
	smells, err := analyzer.AnalyzeFile("test.go", []byte(code))
	if err != nil {
		t.Fatalf("AnalyzeFile() error = %v", err)
	}

	// Should detect long function smell
	found := false
	for _, smell := range smells {
		if smell.Type == SmellLongFunction {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to detect long function smell")
	}
}

func TestAnalyzer_DetectDeepNesting(t *testing.T) {
	analyzer := NewAnalyzer(&SmellConfig{
		MaxNestingDepth: 3,
	})

	code := `package main

func nested() {
	if a {
		if b {
			if c {
				if d {
					if e {
						deep()
					}
				}
			}
		}
	}
}
`
	smells, err := analyzer.AnalyzeFile("test.go", []byte(code))
	if err != nil {
		t.Fatalf("AnalyzeFile() error = %v", err)
	}

	found := false
	for _, smell := range smells {
		if smell.Type == SmellDeepNesting {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to detect deep nesting smell")
	}
}

func TestDiff_ToUnifiedDiff(t *testing.T) {
	diff := &Diff{
		File: "main.go",
		Hunks: []Hunk{
			{
				StartLineOld: 5,
				EndLineOld:   7,
				StartLineNew: 5,
				EndLineNew:   6,
				Before:       "old line 1\nold line 2\nold line 3",
				After:        "new line 1\nnew line 2",
			},
		},
	}

	unified := diff.ToUnifiedDiff()

	if unified == "" {
		t.Error("ToUnifiedDiff() returned empty string")
	}

	// Should contain file headers
	if !contains(unified, "--- a/main.go") {
		t.Error("Missing old file header")
	}
	if !contains(unified, "+++ b/main.go") {
		t.Error("Missing new file header")
	}

	// Should contain hunk header
	if !contains(unified, "@@") {
		t.Error("Missing hunk header")
	}
}

func TestValidator_RequiresExplanation(t *testing.T) {
	validator := NewValidator()

	sugg := &Suggestion{
		ID:          "test-1",
		Title:       "Test Refactor",
		Explanation: "", // Missing!
		Diff: &Diff{
			File: "test.go",
			Hunks: []Hunk{{After: "valid code"}},
		},
	}

	result, err := validator.Validate(context.Background(), sugg, "")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Valid {
		t.Error("Suggestion without explanation should be invalid")
	}
}

func TestValidator_RequiresDiff(t *testing.T) {
	validator := NewValidator()

	sugg := &Suggestion{
		ID:          "test-1",
		Title:       "Test Refactor",
		Explanation: "This improves the code",
		Diff:        nil, // Missing!
	}

	result, err := validator.Validate(context.Background(), sugg, "")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Valid {
		t.Error("Suggestion without diff should be invalid")
	}
}

func TestApplyDiff(t *testing.T) {
	original := `line 1
line 2
line 3
line 4
line 5`

	diff := &Diff{
		File: "test.txt",
		Hunks: []Hunk{
			{
				StartLineOld: 2,
				EndLineOld:   3,
				StartLineNew: 2,
				EndLineNew:   2,
				Before:       "line 2\nline 3",
				After:        "new line 2",
			},
		},
	}

	result, err := ApplyDiff(original, diff)
	if err != nil {
		t.Fatalf("ApplyDiff() error = %v", err)
	}

	expected := `line 1
new line 2
line 4
line 5`

	if result != expected {
		t.Errorf("ApplyDiff() = %q, want %q", result, expected)
	}
}

func TestDiff_GetStats(t *testing.T) {
	diff := &Diff{
		File: "test.go",
		Hunks: []Hunk{
			{
				Before: "old1\nold2\nold3",
				After:  "new1\nnew2",
			},
		},
	}

	stats := diff.GetStats()

	if stats.LinesRemoved != 3 {
		t.Errorf("LinesRemoved = %d, want 3", stats.LinesRemoved)
	}
	if stats.LinesAdded != 2 {
		t.Errorf("LinesAdded = %d, want 2", stats.LinesAdded)
	}
	if stats.HunkCount != 1 {
		t.Errorf("HunkCount = %d, want 1", stats.HunkCount)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
