package ctxbuilder

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// =============================================================================
// Cursor Context — Extracts surrounding scope for better planning
// =============================================================================

// CursorContext extracts the function or class surrounding a cursor position.
// Uses bracket counting (not AST) for language-agnostic scope detection.
type CursorContext struct {
	workspaceRoot string
}

// NewCursorContext creates a new cursor context extractor.
func NewCursorContext(root string) *CursorContext {
	return &CursorContext{workspaceRoot: root}
}

// ScopeResult contains the extracted scope information.
type ScopeResult struct {
	Name      string // Function/class name (best-effort)
	StartLine int    // 1-indexed
	EndLine   int    // 1-indexed
	Content   string // The scope body
	Kind      string // "function", "class", "method", or "block"
}

// ExtractScope finds the nearest enclosing function/class for the given line.
// Returns nil if no scope is found (e.g. top-level code).
func (cc *CursorContext) ExtractScope(file string, line int) *ScopeResult {
	path := filepath.Join(cc.workspaceRoot, file)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if line < 1 || line > len(lines) {
		return nil
	}

	// Find scope boundaries by walking backward/forward with bracket counting
	scopeStart, scopeEnd := cc.findScopeBounds(lines, line-1) // Convert to 0-indexed

	if scopeStart < 0 || scopeEnd < 0 {
		return nil
	}

	// Extract scope content (limit to 200 lines for budget)
	maxLines := 200
	if scopeEnd-scopeStart+1 > maxLines {
		// Keep first 100 and last 100 lines
		topPart := lines[scopeStart : scopeStart+100]
		bottomPart := lines[scopeEnd-99 : scopeEnd+1]
		content := strings.Join(topPart, "\n") + "\n  // ... (scope truncated) ...\n" + strings.Join(bottomPart, "\n")
		return &ScopeResult{
			Name:      cc.detectName(lines, scopeStart),
			StartLine: scopeStart + 1,
			EndLine:   scopeEnd + 1,
			Content:   content,
			Kind:      cc.detectKind(lines, scopeStart),
		}
	}

	content := strings.Join(lines[scopeStart:scopeEnd+1], "\n")
	return &ScopeResult{
		Name:      cc.detectName(lines, scopeStart),
		StartLine: scopeStart + 1,
		EndLine:   scopeEnd + 1,
		Content:   content,
		Kind:      cc.detectKind(lines, scopeStart),
	}
}

// findScopeBounds finds the start and end of the enclosing scope.
// Uses bracket counting to track nesting depth.
func (cc *CursorContext) findScopeBounds(lines []string, cursorIdx int) (int, int) {
	// Walk backward from cursor to find the opening brace of the enclosing scope
	depth := 0
	scopeStart := -1

	for i := cursorIdx; i >= 0; i-- {
		line := lines[i]
		for j := len(line) - 1; j >= 0; j-- {
			switch line[j] {
			case '}':
				depth++
			case '{':
				if depth == 0 {
					// Found our opening brace — walk back to the declaration line
					scopeStart = cc.findDeclarationStart(lines, i)
					goto foundStart
				}
				depth--
			}
		}
	}

foundStart:
	if scopeStart < 0 {
		return -1, -1
	}

	// Walk forward from the opening brace to find the matching closing brace
	depth = 0
	for i := scopeStart; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return scopeStart, i
				}
			}
		}
	}

	return scopeStart, len(lines) - 1
}

// findDeclarationStart walks back from a brace line to find the start of
// the declaration (e.g., "func foo(...)" may span multiple lines).
func (cc *CursorContext) findDeclarationStart(lines []string, braceLineIdx int) int {
	start := braceLineIdx

	// Walk back while lines look like continuation (no semicolons, keywords)
	for i := braceLineIdx - 1; i >= 0 && i >= braceLineIdx-5; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || trimmed == "}" || strings.HasSuffix(trimmed, ";") {
			break
		}
		// Check if this looks like a declaration keyword
		if cc.hasDeclarationKeyword(trimmed) {
			start = i
			break
		}
		// If it looks like a continuation (e.g., parameter list), extend
		if strings.HasSuffix(trimmed, ",") || strings.HasSuffix(trimmed, "(") {
			start = i
		}
	}

	return start
}

// detectName tries to extract the function/class name from the declaration line.
func (cc *CursorContext) detectName(lines []string, startIdx int) string {
	if startIdx < 0 || startIdx >= len(lines) {
		return "unknown"
	}

	line := strings.TrimSpace(lines[startIdx])

	// Go: func (x *T) Name(...) or func Name(...)
	if strings.HasPrefix(line, "func ") {
		rest := line[5:]
		// Skip receiver
		if strings.HasPrefix(rest, "(") {
			closeIdx := strings.Index(rest, ")")
			if closeIdx > 0 && closeIdx+2 < len(rest) {
				rest = strings.TrimSpace(rest[closeIdx+1:])
			}
		}
		if parenIdx := strings.Index(rest, "("); parenIdx > 0 {
			return strings.TrimSpace(rest[:parenIdx])
		}
	}

	// JS/TS: function name(...), const name = ..., or name(...)  {
	for _, prefix := range []string{"function ", "async function "} {
		if strings.HasPrefix(line, prefix) {
			rest := line[len(prefix):]
			if parenIdx := strings.Index(rest, "("); parenIdx > 0 {
				return strings.TrimSpace(rest[:parenIdx])
			}
		}
	}

	// Python: def name(...) or class Name:
	if strings.HasPrefix(line, "def ") {
		rest := line[4:]
		if parenIdx := strings.Index(rest, "("); parenIdx > 0 {
			return strings.TrimSpace(rest[:parenIdx])
		}
	}
	if strings.HasPrefix(line, "class ") {
		rest := line[6:]
		for i, ch := range rest {
			if ch == '(' || ch == ':' || ch == ' ' {
				return strings.TrimSpace(rest[:i])
			}
		}
	}

	return "unknown"
}

// detectKind determines whether the scope is a function, class, method, or block.
func (cc *CursorContext) detectKind(lines []string, startIdx int) string {
	if startIdx < 0 || startIdx >= len(lines) {
		return "block"
	}
	line := strings.TrimSpace(lines[startIdx])

	if strings.HasPrefix(line, "class ") || strings.Contains(line, " class ") {
		return "class"
	}
	if strings.HasPrefix(line, "func (") {
		return "method"
	}
	if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "function ") ||
		strings.HasPrefix(line, "async function ") || strings.HasPrefix(line, "def ") {
		return "function"
	}
	return "block"
}

// hasDeclarationKeyword checks if a line starts with a common declaration keyword.
func (cc *CursorContext) hasDeclarationKeyword(line string) bool {
	keywords := []string{
		"func ", "function ", "async function ", "def ", "class ",
		"if ", "for ", "while ", "switch ", "select ", "case ",
		"export ", "public ", "private ", "protected ",
	}
	for _, kw := range keywords {
		if strings.HasPrefix(line, kw) {
			return true
		}
	}
	return false
}

// BuildCursorContext creates a prompt-ready context string for the cursor position.
func (cc *CursorContext) BuildCursorContext(file string, line int) string {
	scope := cc.ExtractScope(file, line)
	if scope == nil {
		return ""
	}

	return strings.Join([]string{
		"=== CURSOR CONTEXT ===",
		"File: " + file,
		"Scope: " + scope.Kind + " " + scope.Name + " (lines " +
			strconv.Itoa(scope.StartLine) + "-" + strconv.Itoa(scope.EndLine) + ")",
		"",
		scope.Content,
		"=== END CURSOR CONTEXT ===",
	}, "\n")
}
