//go:build cgo && !nocgo

package ast

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// GoAnalyzer provides Go-specific AST analysis
type GoAnalyzer struct{}

// ExtractPackage extracts the package name from Go source
func (a *GoAnalyzer) ExtractPackage(node *sitter.Node, content []byte) string {
	// Find package_clause
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "package_clause" {
			// Get package_identifier
			for j := 0; j < int(child.ChildCount()); j++ {
				pkgId := child.Child(j)
				if pkgId.Type() == "package_identifier" {
					return string(content[pkgId.StartByte():pkgId.EndByte()])
				}
			}
		}
	}
	return ""
}

// ExtractImports extracts import statements from Go source
func (a *GoAnalyzer) ExtractImports(node *sitter.Node, content []byte) []Import {
	var imports []Import

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "import_declaration" {
			imports = append(imports, a.parseImportDecl(n, content)...)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return imports
}

func (a *GoAnalyzer) parseImportDecl(node *sitter.Node, content []byte) []Import {
	var imports []Import

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "import_spec":
			imp := a.parseImportSpec(child, content)
			if imp != nil {
				imports = append(imports, *imp)
			}
		case "import_spec_list":
			for j := 0; j < int(child.ChildCount()); j++ {
				spec := child.Child(j)
				if spec.Type() == "import_spec" {
					imp := a.parseImportSpec(spec, content)
					if imp != nil {
						imports = append(imports, *imp)
					}
				}
			}
		}
	}

	return imports
}

func (a *GoAnalyzer) parseImportSpec(node *sitter.Node, content []byte) *Import {
	imp := &Import{
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			StartCol:  int(node.StartPoint().Column),
			EndCol:    int(node.EndPoint().Column),
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "package_identifier", "blank_identifier", "dot":
			imp.Alias = string(content[child.StartByte():child.EndByte()])
		case "interpreted_string_literal":
			path := string(content[child.StartByte():child.EndByte()])
			imp.Path = strings.Trim(path, `"`)
		}
	}

	// Determine if local import
	imp.IsLocal = !strings.Contains(imp.Path, ".") && !isStdLib(imp.Path)

	return imp
}

// isStdLib checks if an import path is a Go standard library package
func isStdLib(path string) bool {
	stdLibPrefixes := []string{
		"archive", "bufio", "bytes", "compress", "container", "context",
		"crypto", "database", "debug", "embed", "encoding", "errors",
		"expvar", "flag", "fmt", "go", "hash", "html", "image", "index",
		"io", "log", "math", "mime", "net", "os", "path", "plugin",
		"reflect", "regexp", "runtime", "sort", "strconv", "strings",
		"sync", "syscall", "testing", "text", "time", "unicode", "unsafe",
	}

	for _, prefix := range stdLibPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// ExtractSymbols extracts all symbols from Go source
func (a *GoAnalyzer) ExtractSymbols(node *sitter.Node, content []byte) []Symbol {
	var symbols []Symbol

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		switch n.Type() {
		case "function_declaration":
			if sym := a.parseFunctionDecl(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "method_declaration":
			if sym := a.parseMethodDecl(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "type_declaration":
			symbols = append(symbols, a.parseTypeDecl(n, content)...)
		case "var_declaration", "const_declaration":
			symbols = append(symbols, a.parseVarDecl(n, content)...)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return symbols
}

func (a *GoAnalyzer) parseFunctionDecl(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind: SymbolKindFunction,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			StartCol:  int(node.StartPoint().Column),
			EndCol:    int(node.EndPoint().Column),
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			sym.Name = string(content[child.StartByte():child.EndByte()])
		case "parameter_list":
			sym.Signature = string(content[child.StartByte():child.EndByte()])
		}
	}

	// Determine visibility (Go: uppercase = exported)
	if len(sym.Name) > 0 && sym.Name[0] >= 'A' && sym.Name[0] <= 'Z' {
		sym.Visibility = VisibilityPublic
	} else {
		sym.Visibility = VisibilityPrivate
	}

	return sym
}

func (a *GoAnalyzer) parseMethodDecl(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind: SymbolKindMethod,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			StartCol:  int(node.StartPoint().Column),
			EndCol:    int(node.EndPoint().Column),
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "field_identifier":
			sym.Name = string(content[child.StartByte():child.EndByte()])
		case "parameter_list":
			if i == 0 { // Receiver
				// Extract receiver type
				receiverText := string(content[child.StartByte():child.EndByte()])
				sym.Parent = extractReceiverType(receiverText)
			} else {
				sym.Signature = string(content[child.StartByte():child.EndByte()])
			}
		}
	}

	// Determine visibility
	if len(sym.Name) > 0 && sym.Name[0] >= 'A' && sym.Name[0] <= 'Z' {
		sym.Visibility = VisibilityPublic
	} else {
		sym.Visibility = VisibilityPrivate
	}

	return sym
}

func extractReceiverType(receiver string) string {
	// Remove parentheses and pointer/name
	receiver = strings.Trim(receiver, "()")
	parts := strings.Fields(receiver)
	if len(parts) >= 2 {
		return strings.TrimPrefix(parts[1], "*")
	} else if len(parts) == 1 {
		return strings.TrimPrefix(parts[0], "*")
	}
	return ""
}

func (a *GoAnalyzer) parseTypeDecl(node *sitter.Node, content []byte) []Symbol {
	var symbols []Symbol

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_spec" {
			if sym := a.parseTypeSpec(child, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		}
	}

	return symbols
}

func (a *GoAnalyzer) parseTypeSpec(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			StartCol:  int(node.StartPoint().Column),
			EndCol:    int(node.EndPoint().Column),
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "type_identifier":
			sym.Name = string(content[child.StartByte():child.EndByte()])
		case "struct_type":
			sym.Kind = SymbolKindStruct
		case "interface_type":
			sym.Kind = SymbolKindInterface
		default:
			if sym.Kind == "" {
				sym.Kind = SymbolKindType
			}
		}
	}

	// Determine visibility
	if len(sym.Name) > 0 && sym.Name[0] >= 'A' && sym.Name[0] <= 'Z' {
		sym.Visibility = VisibilityPublic
	} else {
		sym.Visibility = VisibilityPrivate
	}

	return sym
}

func (a *GoAnalyzer) parseVarDecl(node *sitter.Node, content []byte) []Symbol {
	var symbols []Symbol
	kind := SymbolKindVariable
	if node.Type() == "const_declaration" {
		kind = SymbolKindConstant
	}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "identifier" {
			name := string(content[n.StartByte():n.EndByte()])
			visibility := VisibilityPrivate
			if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
				visibility = VisibilityPublic
			}
			symbols = append(symbols, Symbol{
				Name:       name,
				Kind:       kind,
				Visibility: visibility,
				Location: Location{
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

	return symbols
}

// ExtractExports extracts exported symbols (in Go, all public symbols are exports)
func (a *GoAnalyzer) ExtractExports(node *sitter.Node, content []byte, symbols []Symbol) []Export {
	var exports []Export
	for _, sym := range symbols {
		if sym.Visibility == VisibilityPublic {
			exports = append(exports, Export{
				Name:       sym.Name,
				SymbolKind: sym.Kind,
			})
		}
	}
	return exports
}
