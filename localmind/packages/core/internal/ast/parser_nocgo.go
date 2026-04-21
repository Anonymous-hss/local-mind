//go:build !cgo || nocgo

// Package ast provides AST parsing and code intelligence.
// This file contains a stub implementation for when CGO is not available.
// The stub returns basic results without actual AST parsing.
package ast

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Parser provides multi-language AST parsing (stub implementation without CGO)
type Parser struct {
	// No fields needed for stub
}

// NewParser creates a new parser (stub)
func NewParser() *Parser {
	return &Parser{}
}

// RegisterAnalyzer is a no-op in stub mode
func (p *Parser) RegisterAnalyzer(lang Language, analyzer interface{}) {
	// No-op in stub mode
}

// Parse parses source code and returns a ParseResult (stub implementation)
// This uses regex-based extraction which is less accurate but works without CGO
func (p *Parser) Parse(ctx context.Context, filename string, content []byte) (*ParseResult, error) {
	startTime := time.Now()

	lang := LanguageFromFilename(filename)
	if lang == LanguageUnknown {
		return nil, fmt.Errorf("unsupported file type: %s", filename)
	}

	result := &ParseResult{
		File:     filename,
		Language: string(lang),
	}

	// Use simple regex-based extraction
	switch lang {
	case LanguageGo:
		result.Package = extractGoPackage(content)
		result.Imports = extractGoImports(content)
		result.Symbols = extractGoSymbols(content)
	case LanguageTypeScript, LanguageJavaScript:
		result.Imports = extractJSImports(content)
		result.Symbols = extractJSSymbols(content)
	case LanguagePython:
		result.Imports = extractPythonImports(content)
		result.Symbols = extractPythonSymbols(content)
	}

	// Mark exports for public symbols
	for _, sym := range result.Symbols {
		if sym.Visibility == VisibilityPublic {
			result.Exports = append(result.Exports, Export{
				Name:       sym.Name,
				SymbolKind: sym.Kind,
			})
		}
	}

	result.ParseTimeMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// ParseFile parses a file from disk (stub)
func (p *Parser) ParseFile(ctx context.Context, filepath string) (*ParseResult, error) {
	return nil, fmt.Errorf("ParseFile not implemented in stub mode - use Parse with content")
}

// IsCGOEnabled returns false when CGO is not enabled
func IsCGOEnabled() bool {
	return false
}

// ============================================
// Regex-based extraction (fallback for no CGO)
// ============================================

var (
	goPackageRe = regexp.MustCompile(`(?m)^package\s+(\w+)`)
	goImportRe  = regexp.MustCompile(`(?m)^\s*(?:import\s+)?(?:(\w+)\s+)?"([^"]+)"`)
	goFuncRe    = regexp.MustCompile(`(?m)^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`)
	goTypeRe    = regexp.MustCompile(`(?m)^type\s+(\w+)\s+(struct|interface)`)
	goConstRe   = regexp.MustCompile(`(?m)^(?:const|var)\s+(\w+)`)

	jsImportRe = regexp.MustCompile(`(?m)^import\s+.*?from\s+['"]([^'"]+)['"]`)
	jsFuncRe   = regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	jsClassRe  = regexp.MustCompile(`(?m)^(?:export\s+)?class\s+(\w+)`)
	jsConstRe  = regexp.MustCompile(`(?m)^(?:export\s+)?(?:const|let|var)\s+(\w+)`)

	pyImportRe = regexp.MustCompile(`(?m)^(?:from\s+(\S+)\s+)?import\s+([^#\n]+)`)
	pyFuncRe   = regexp.MustCompile(`(?m)^def\s+(\w+)\s*\(`)
	pyClassRe  = regexp.MustCompile(`(?m)^class\s+(\w+)`)
)

func extractGoPackage(content []byte) string {
	matches := goPackageRe.FindSubmatch(content)
	if len(matches) >= 2 {
		return string(matches[1])
	}
	return ""
}

func extractGoImports(content []byte) []Import {
	var imports []Import
	matches := goImportRe.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			imp := Import{
				Path:    string(m[2]),
				IsLocal: strings.HasPrefix(string(m[2]), "./") || strings.HasPrefix(string(m[2]), "../"),
			}
			if len(m[1]) > 0 {
				imp.Alias = string(m[1])
			}
			imports = append(imports, imp)
		}
	}
	return imports
}

func extractGoSymbols(content []byte) []Symbol {
	var symbols []Symbol
	lines := strings.Split(string(content), "\n")

	// Extract functions
	for i, line := range lines {
		if matches := goFuncRe.FindStringSubmatch(line); len(matches) >= 2 {
			name := matches[1]
			vis := VisibilityPrivate
			if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
				vis = VisibilityPublic
			}
			symbols = append(symbols, Symbol{
				Name:       name,
				Kind:       SymbolKindFunction,
				Visibility: vis,
				Location:   Location{StartLine: i + 1, EndLine: i + 1},
			})
		}
	}

	// Extract types
	for i, line := range lines {
		if matches := goTypeRe.FindStringSubmatch(line); len(matches) >= 3 {
			name := matches[1]
			kind := SymbolKindStruct
			if matches[2] == "interface" {
				kind = SymbolKindInterface
			}
			vis := VisibilityPrivate
			if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
				vis = VisibilityPublic
			}
			symbols = append(symbols, Symbol{
				Name:       name,
				Kind:       kind,
				Visibility: vis,
				Location:   Location{StartLine: i + 1, EndLine: i + 1},
			})
		}
	}

	return symbols
}

func extractJSImports(content []byte) []Import {
	var imports []Import
	matches := jsImportRe.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			path := string(m[1])
			imports = append(imports, Import{
				Path:    path,
				IsLocal: strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../"),
			})
		}
	}
	return imports
}

func extractJSSymbols(content []byte) []Symbol {
	var symbols []Symbol
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		// Functions
		if matches := jsFuncRe.FindStringSubmatch(line); len(matches) >= 2 {
			symbols = append(symbols, Symbol{
				Name:       matches[1],
				Kind:       SymbolKindFunction,
				Visibility: VisibilityPublic,
				Location:   Location{StartLine: i + 1, EndLine: i + 1},
			})
		}
		// Classes
		if matches := jsClassRe.FindStringSubmatch(line); len(matches) >= 2 {
			symbols = append(symbols, Symbol{
				Name:       matches[1],
				Kind:       SymbolKindClass,
				Visibility: VisibilityPublic,
				Location:   Location{StartLine: i + 1, EndLine: i + 1},
			})
		}
	}

	return symbols
}

func extractPythonImports(content []byte) []Import {
	var imports []Import
	matches := pyImportRe.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			path := string(m[1])
			if path == "" {
				path = strings.TrimSpace(string(m[2]))
			}
			imports = append(imports, Import{
				Path:    path,
				IsLocal: !strings.Contains(path, "."),
			})
		}
	}
	return imports
}

func extractPythonSymbols(content []byte) []Symbol {
	var symbols []Symbol
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		// Functions
		if matches := pyFuncRe.FindStringSubmatch(line); len(matches) >= 2 {
			name := matches[1]
			vis := VisibilityPublic
			if strings.HasPrefix(name, "_") {
				vis = VisibilityPrivate
			}
			symbols = append(symbols, Symbol{
				Name:       name,
				Kind:       SymbolKindFunction,
				Visibility: vis,
				Location:   Location{StartLine: i + 1, EndLine: i + 1},
			})
		}
		// Classes
		if matches := pyClassRe.FindStringSubmatch(line); len(matches) >= 2 {
			symbols = append(symbols, Symbol{
				Name:       matches[1],
				Kind:       SymbolKindClass,
				Visibility: VisibilityPublic,
				Location:   Location{StartLine: i + 1, EndLine: i + 1},
			})
		}
	}

	return symbols
}
