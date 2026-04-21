package ctxbuilder

import (
	"strings"
	"testing"
)

func TestAllocateUnderBudget(t *testing.T) {
	cb := NewContextBudget(10000)

	sections := []ContextSection{
		{Name: "task", Content: "Fix the login bug", Priority: PriorityTask},
		{Name: "diff", Content: "modified: auth.go", Priority: PriorityChange},
		{Name: "files", Content: "func Login() {}", Priority: PriorityFiles},
	}

	result := cb.Allocate(sections)

	if len(result) != 3 {
		t.Errorf("Expected 3 sections, got %d", len(result))
	}

	// Highest priority should be first
	if result[0].Name != "task" {
		t.Errorf("Expected first section to be 'task', got %q", result[0].Name)
	}
}

func TestAllocateOverBudget(t *testing.T) {
	cb := NewContextBudget(50) // Very small budget

	sections := []ContextSection{
		{Name: "task", Content: "Fix the login bug", Priority: PriorityTask},                                                            // 17 chars
		{Name: "diff", Content: "modified: auth.go (+10 -5)", Priority: PriorityChange},                                                 // 26 chars
		{Name: "history", Content: "Previous: learned to use proper error handling patterns for auth flows", Priority: PriorityHistory}, // Long
	}

	result := cb.Allocate(sections)

	// Task (17) + Diff (26) = 43, fits. History should be truncated or dropped.
	// The truncation marker adds some overhead, but highest-priority sections must survive
	foundTask := false
	foundDiff := false
	for _, s := range result {
		if s.Name == "task" {
			foundTask = true
			// Task should be unchanged (fits in budget)
			if s.Content != "Fix the login bug" {
				t.Errorf("Expected task content preserved, got: %q", s.Content)
			}
		}
		if s.Name == "diff" {
			foundDiff = true
		}
	}

	if !foundTask {
		t.Error("Expected task section to be preserved")
	}
	if !foundDiff {
		t.Error("Expected diff section to be preserved")
	}
}

func TestAllocateWithEmpty(t *testing.T) {
	cb := NewContextBudget(10000)

	sections := []ContextSection{
		{Name: "task", Content: "Fix bug", Priority: PriorityTask},
		{Name: "diff", Content: "", Priority: PriorityChange},
		{Name: "files", Content: "   ", Priority: PriorityFiles},
		{Name: "history", Content: "Learned pattern", Priority: PriorityHistory},
	}

	result := cb.Allocate(sections)

	// Empty and whitespace-only sections should be filtered
	if len(result) != 2 {
		t.Errorf("Expected 2 non-empty sections, got %d", len(result))
	}
}

func TestAllocateTruncationAtLineBoundary(t *testing.T) {
	cb := NewContextBudget(50)

	longContent := "line one\nline two\nline three\nline four\nline five\nline six\nline seven\nline eight\nline nine\nline ten\n"

	sections := []ContextSection{
		{Name: "only", Content: longContent, Priority: PriorityTask},
	}

	result := cb.Allocate(sections)

	if len(result) != 1 {
		t.Fatalf("Expected 1 section, got %d", len(result))
	}

	// Should end with the trimmed marker
	if !strings.Contains(result[0].Content, "trimmed to fit") {
		t.Error("Expected truncation marker in output")
	}
}

func TestBuildPromptSections(t *testing.T) {
	cb := NewContextBudget(10000)

	sections := []ContextSection{
		{Name: "task", Content: "Fix the bug", Priority: PriorityTask},
		{Name: "diff", Content: "modified: main.go", Priority: PriorityChange},
	}

	prompt := cb.BuildPromptSections(sections)

	if !strings.Contains(prompt, "Fix the bug") {
		t.Error("Expected prompt to contain task")
	}
	if !strings.Contains(prompt, "modified: main.go") {
		t.Error("Expected prompt to contain diff")
	}
}

func TestDefaultBudget(t *testing.T) {
	cb := NewContextBudget(0)
	if cb.MaxTotalChars != 12000 {
		t.Errorf("Expected default 12000, got %d", cb.MaxTotalChars)
	}
}
