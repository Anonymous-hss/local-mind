//go:build cgo && !nocgo

// Package ast provides AST parsing and code intelligence using Tree-sitter.
// This file contains the CGO-dependent implementation using Tree-sitter.
package ast

import (
	"context"
	"fmt"
	"sync"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Parser provides multi-language AST parsing using Tree-sitter
type Parser struct {
	mu        sync.RWMutex
	parsers   map[Language]*sitter.Parser
	analyzers map[Language]Analyzer
}

// NewParser creates a new multi-language parser
func NewParser() *Parser {
	p := &Parser{
		parsers:   make(map[Language]*sitter.Parser),
		analyzers: make(map[Language]Analyzer),
	}

	// Register default analyzers
	p.RegisterAnalyzer(LanguageGo, &GoAnalyzer{})
	p.RegisterAnalyzer(LanguageTypeScript, &TypeScriptAnalyzer{})
	p.RegisterAnalyzer(LanguageJavaScript, &JavaScriptAnalyzer{})
	p.RegisterAnalyzer(LanguagePython, &PythonAnalyzer{})

	return p
}

// RegisterAnalyzer registers a language-specific analyzer
func (p *Parser) RegisterAnalyzer(lang Language, analyzer Analyzer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.analyzers[lang] = analyzer
}

// getParser returns or creates a parser for the given language
func (p *Parser) getParser(lang Language) (*sitter.Parser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if parser, ok := p.parsers[lang]; ok {
		return parser, nil
	}

	parser := sitter.NewParser()

	switch lang {
	case LanguageGo:
		parser.SetLanguage(golang.GetLanguage())
	case LanguageTypeScript:
		parser.SetLanguage(typescript.GetLanguage())
	case LanguageJavaScript:
		parser.SetLanguage(javascript.GetLanguage())
	case LanguagePython:
		parser.SetLanguage(python.GetLanguage())
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}

	p.parsers[lang] = parser
	return parser, nil
}

// Parse parses source code and returns a ParseResult
func (p *Parser) Parse(ctx context.Context, filename string, content []byte) (*ParseResult, error) {
	startTime := time.Now()

	lang := LanguageFromFilename(filename)
	if lang == LanguageUnknown {
		return nil, fmt.Errorf("unsupported file type: %s", filename)
	}

	parser, err := p.getParser(lang)
	if err != nil {
		return nil, err
	}

	// Parse the content
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()

	// Get analyzer for this language
	p.mu.RLock()
	analyzer, ok := p.analyzers[lang]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no analyzer for language: %s", lang)
	}

	// Extract symbols, imports, exports
	result := &ParseResult{
		File:     filename,
		Language: string(lang),
	}

	// Extract package name (language-specific)
	result.Package = analyzer.ExtractPackage(rootNode, content)

	// Extract imports
	result.Imports = analyzer.ExtractImports(rootNode, content)

	// Extract symbols
	result.Symbols = analyzer.ExtractSymbols(rootNode, content)

	// Extract exports
	result.Exports = analyzer.ExtractExports(rootNode, content, result.Symbols)

	// Check for parse errors
	if rootNode.HasError() {
		result.Errors = extractParseErrors(rootNode, filename)
	}

	result.ParseTimeMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// ParseFile parses a file from disk
func (p *Parser) ParseFile(ctx context.Context, filepath string) (*ParseResult, error) {
	// Read file content
	content, err := readFile(filepath)
	if err != nil {
		return nil, err
	}
	return p.Parse(ctx, filepath, content)
}

// extractParseErrors extracts error nodes from the AST
func extractParseErrors(node *sitter.Node, filename string) []ParseError {
	var errors []ParseError

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.IsError() || n.IsMissing() {
			errors = append(errors, ParseError{
				Message:  "syntax error",
				Severity: "error",
				Location: Location{
					File:      filename,
					StartLine: int(n.StartPoint().Row) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					StartCol:  int(n.StartPoint().Column),
					EndCol:    int(n.EndPoint().Column),
				},
			})
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return errors
}

// readFile reads a file's contents
func readFile(filepath string) ([]byte, error) {
	// Use os.ReadFile for simplicity
	return nil, fmt.Errorf("not implemented - use Parse with content directly")
}

// Analyzer is the interface for language-specific analysis
type Analyzer interface {
	ExtractPackage(node *sitter.Node, content []byte) string
	ExtractImports(node *sitter.Node, content []byte) []Import
	ExtractSymbols(node *sitter.Node, content []byte) []Symbol
	ExtractExports(node *sitter.Node, content []byte, symbols []Symbol) []Export
}

// IsCGOEnabled returns true when CGO is enabled
func IsCGOEnabled() bool {
	return true
}
