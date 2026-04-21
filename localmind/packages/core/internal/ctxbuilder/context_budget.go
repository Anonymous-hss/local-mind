package ctxbuilder

import (
	"sort"
	"strings"
)

// =============================================================================
// Context Budget — Token-aware trimming with priority ranking
// =============================================================================

// ContextPriority defines the importance of a context section.
// Higher values are kept first when trimming.
const (
	PriorityTask    = 100 // Task description — never trimmed
	PriorityChange  = 80  // Git diff — most relevant to current work
	PriorityCursor  = 70  // Cursor scope — immediate editing context
	PriorityFiles   = 60  // File contents — needed for edits
	PriorityImports = 50  // Imported dependencies — context for editing
	PriorityRepo    = 40  // Repo conventions/tech stack
	PriorityHistory = 20  // Past task lessons — trim first
)

// DefaultSectionCaps defines hard character limits per section type.
var DefaultSectionCaps = map[string]int{
	"files":   6000,
	"imports": 3000,
	"changes": 2000,
	"cursor":  2000,
	"repo":    1500,
	"history": 1000,
}

// ContextSection represents a named section of prompt context.
type ContextSection struct {
	Name     string
	Content  string
	Priority int
}

// ContextBudget manages total prompt size across context types.
type ContextBudget struct {
	MaxTotalChars int            // Default: 12000
	SectionCaps   map[string]int // Per-section hard caps (optional)
}

// NewContextBudget creates a budget manager with the given total character limit.
func NewContextBudget(maxChars int) *ContextBudget {
	if maxChars <= 0 {
		maxChars = 12000
	}
	return &ContextBudget{
		MaxTotalChars: maxChars,
		SectionCaps:   DefaultSectionCaps,
	}
}

// Allocate trims context sections to fit within the budget.
// Higher-priority sections are preserved; lower-priority sections are
// truncated or removed entirely when the budget is exceeded.
func (cb *ContextBudget) Allocate(sections []ContextSection) []ContextSection {
	// Filter empty sections and apply per-section hard caps
	var nonEmpty []ContextSection
	for _, s := range sections {
		if strings.TrimSpace(s.Content) == "" {
			continue
		}
		// Enforce hard cap per section type
		if cap, ok := cb.SectionCaps[s.Name]; ok && len(s.Content) > cap {
			truncated := s.Content[:cap]
			if nl := strings.LastIndex(truncated, "\n"); nl > cap/2 {
				truncated = truncated[:nl]
			}
			s.Content = truncated + "\n... (section cap reached)"
		}
		nonEmpty = append(nonEmpty, s)
	}

	if len(nonEmpty) == 0 {
		return nil
	}

	// Sort by priority (highest first)
	sort.Slice(nonEmpty, func(i, j int) bool {
		return nonEmpty[i].Priority > nonEmpty[j].Priority
	})

	// Calculate total size
	total := 0
	for _, s := range nonEmpty {
		total += len(s.Content)
	}

	// Under budget — return everything
	if total <= cb.MaxTotalChars {
		return nonEmpty
	}

	// Over budget — allocate from highest priority down
	remaining := cb.MaxTotalChars
	var result []ContextSection

	for _, s := range nonEmpty {
		if remaining <= 0 {
			break
		}

		if len(s.Content) <= remaining {
			// Section fits entirely
			result = append(result, s)
			remaining -= len(s.Content)
		} else {
			// Truncate the section to fit
			truncated := s.Content[:remaining]
			// Try to truncate at a line boundary
			if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > len(truncated)/2 {
				truncated = truncated[:lastNewline]
			}
			truncated += "\n... (trimmed to fit context budget)"

			result = append(result, ContextSection{
				Name:     s.Name,
				Content:  truncated,
				Priority: s.Priority,
			})
			remaining = 0
		}
	}

	return result
}

// BuildPromptSections combines allocated sections into a formatted string.
func (cb *ContextBudget) BuildPromptSections(sections []ContextSection) string {
	allocated := cb.Allocate(sections)
	if len(allocated) == 0 {
		return ""
	}

	var b strings.Builder
	for _, s := range allocated {
		b.WriteString(s.Content)
		b.WriteString("\n\n")
	}

	return strings.TrimSpace(b.String())
}
