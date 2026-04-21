package ctxbuilder

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// Import Resolver — Scans for cross-file dependencies
// =============================================================================

// Language patterns for import detection
var (
	// Go: import "fmt" or import ( ... )
	goImportRegex = regexp.MustCompile(`^\s*(?:import\s+)?["']([^"']+)["']`)

	// TS/JS: import ... from '...' OR import '...' OR require('...')
	tsImportRegex = regexp.MustCompile(`(?:import\s+(?:.*?from\s+)?["']([^"']+)["'])|(?:require\s*\(\s*["']([^"']+)["']\s*\))`)

	// Python: from ... import ... or import ...
	pyImportRegex = regexp.MustCompile(`^\s*(?:from\s+([a-zA-Z0-9_\.]+)\s+import)|(?:import\s+([a-zA-Z0-9_\.]+))`)
)

// ImportResolver finds local file dependencies based on import statements.
type ImportResolver struct {
	workspaceRoot string
}

// NewImportResolver creates a new resolver for the given workspace.
func NewImportResolver(root string) *ImportResolver {
	return &ImportResolver{workspaceRoot: root}
}

// ResolveImports scans the given files for imports and returns a list of
// referenced local files that exist in the workspace.
func (ir *ImportResolver) ResolveImports(files []string) ([]string, error) {
	seen := make(map[string]bool)
	var results []string

	// Mark input files as seen so we don't re-include them
	for _, f := range files {
		seen[f] = true
	}

	for _, file := range files {
		imports, err := ir.extractImports(file)
		if err != nil {
			continue // Skip errors, best effort
		}

		for _, imp := range imports {
			// Try to resolve to a local file
			resolved := ir.resolvePath(file, imp)
			if resolved != "" && !seen[resolved] {
				seen[resolved] = true
				results = append(results, resolved)
			}
		}
	}

	return results, nil
}

// extractImports extracts import strings from a single file.
func (ir *ImportResolver) extractImports(file string) ([]string, error) {
	path := filepath.Join(ir.workspaceRoot, file)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(file))
	var imports []string
	scanner := bufio.NewScanner(f)

	inGoImportBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		switch ext {
		case ".go":
			if trimmed == "import (" {
				inGoImportBlock = true
				continue
			}
			if inGoImportBlock && trimmed == ")" {
				inGoImportBlock = false
				continue
			}

			if strings.HasPrefix(trimmed, "import ") || inGoImportBlock {
				matches := goImportRegex.FindStringSubmatch(trimmed)
				if len(matches) > 1 {
					imports = append(imports, matches[1])
				}
			}

		case ".ts", ".tsx", ".js", ".jsx":
			matches := tsImportRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				// match[1] is 'import ...', match[2] is 'require(...)'
				if match[1] != "" {
					imports = append(imports, match[1])
				} else if len(match) > 2 && match[2] != "" {
					imports = append(imports, match[2])
				}
			}

		case ".py":
			matches := pyImportRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				// match[1] is 'from X import', match[2] is 'import X'
				if matches[1] != "" {
					imports = append(imports, matches[1])
				} else if len(matches) > 2 && matches[2] != "" {
					imports = append(imports, matches[2])
				}
			}
		}
	}

	return imports, nil
}

// resolvePath attempts to map an import string to a local file.
func (ir *ImportResolver) resolvePath(sourceFile, importPath string) string {
	// 1. Handle relative paths (starting with . or ..)
	if strings.HasPrefix(importPath, ".") {
		dir := filepath.Dir(sourceFile)
		basePath := filepath.Join(dir, importPath)

		// Check direct file extensions
		candidates := ir.expandExtensions(basePath)
		for _, cand := range candidates {
			if ir.fileExists(cand) {
				return cand
			}
		}

		// Check directory resolution
		if ir.isDir(basePath) {
			// JS/TS: index.ts, index.js
			if ir.fileExists(filepath.Join(basePath, "index.ts")) {
				return filepath.Join(basePath, "index.ts")
			}
			if ir.fileExists(filepath.Join(basePath, "index.js")) {
				return filepath.Join(basePath, "index.js")
			}
			if ir.fileExists(filepath.Join(basePath, "index.tsx")) {
				return filepath.Join(basePath, "index.tsx")
			}
			if ir.fileExists(filepath.Join(basePath, "index.jsx")) {
				return filepath.Join(basePath, "index.jsx")
			}

			// Go: package/package.go or any .go file
			// Try package name match: pkg/utils -> pkg/utils/utils.go
			baseName := filepath.Base(basePath)
			if ir.fileExists(filepath.Join(basePath, baseName+".go")) {
				return filepath.Join(basePath, baseName+".go")
			}
			// Fallback: look for any .go file in the dir
			if files, err := os.ReadDir(filepath.Join(ir.workspaceRoot, basePath)); err == nil {
				for _, f := range files {
					if !f.IsDir() && strings.HasSuffix(f.Name(), ".go") {
						return filepath.Join(basePath, f.Name())
					}
				}
			}
		}
	} else {
		// 2. Handle absolute/module paths
		// Heuristic: check if unknown import path corresponds to a file in workspace
		// e.g., "internal/agent" -> "internal/agent/..."

		// Direct match check (if import is like "pkg/file")
		candidates := ir.expandExtensions(importPath)
		for _, cand := range candidates {
			// Verify it's inside workspace to avoid accidentally picking up system files if CWD is strict
			if ir.fileExists(cand) {
				return cand
			}
		}

		// Go style: "github.com/user/repo/pkg" -> "pkg"
		// If the import path ends with a directory that exists in our workspace, assume it's that package
		// This is a naive heuristic but works for many monorepo setups
		parts := strings.Split(importPath, "/")
		if len(parts) > 0 {
			// Try finding the last segment as a directory or file
			// This is expensive to search everything, so we'll skip deep search for now
			// and just check if it exists at root or commonly used dirs

			// Check if import path exists relative to root (e.g. "packages/core/...")
			if ir.fileExists(importPath) { // as file
				return importPath
			}
			// Check if it's a dir relative to root
			if ir.isDir(importPath) {
				// Try to find a go file inside
				if files, err := os.ReadDir(filepath.Join(ir.workspaceRoot, importPath)); err == nil {
					for _, f := range files {
						if !f.IsDir() && strings.HasSuffix(f.Name(), ".go") {
							return filepath.Join(importPath, f.Name())
						}
					}
				}
			}

			for _, ext := range []string{".go", ".ts", ".js", ".py"} {
				if ir.fileExists(importPath + ext) {
					return importPath + ext
				}
			}
		}
	}

	return ""
}

// expandExtensions adds common extensions to a path
func (ir *ImportResolver) expandExtensions(path string) []string {
	// Clean path
	clean := filepath.Clean(path)

	// If it already has an ext, return it
	if filepath.Ext(clean) != "" {
		return []string{clean}
	}

	return []string{
		clean + ".go",
		clean + ".ts",
		clean + ".tsx",
		clean + ".js",
		clean + ".jsx",
		clean + ".py",
	}
}

// fileExists checks if a file exists in the workspace
func (ir *ImportResolver) fileExists(relPath string) bool {
	info, err := os.Stat(filepath.Join(ir.workspaceRoot, relPath))
	return err == nil && !info.IsDir()
}

// isDir checks if a path is a directory in the workspace
func (ir *ImportResolver) isDir(relPath string) bool {
	info, err := os.Stat(filepath.Join(ir.workspaceRoot, relPath))
	return err == nil && info.IsDir()
}
