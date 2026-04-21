//go:build cgo && !nocgo

package ast

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// TypeScriptAnalyzer provides TypeScript-specific AST analysis
type TypeScriptAnalyzer struct{}

// ExtractPackage extracts module name (from package.json or file path)
func (a *TypeScriptAnalyzer) ExtractPackage(node *sitter.Node, content []byte) string {
	// TypeScript doesn't have package declarations like Go
	// Return empty, to be determined from file path
	return ""
}

// ExtractImports extracts import statements from TypeScript source
func (a *TypeScriptAnalyzer) ExtractImports(node *sitter.Node, content []byte) []Import {
	var imports []Import

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		switch n.Type() {
		case "import_statement":
			if imp := a.parseImportStatement(n, content); imp != nil {
				imports = append(imports, *imp)
			}
		case "import_require_clause":
			if imp := a.parseRequire(n, content); imp != nil {
				imports = append(imports, *imp)
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return imports
}

func (a *TypeScriptAnalyzer) parseImportStatement(node *sitter.Node, content []byte) *Import {
	imp := &Import{
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "string":
			path := string(content[child.StartByte():child.EndByte()])
			imp.Path = strings.Trim(path, `"'`)
		case "import_clause":
			imp.Names = a.parseImportClause(child, content)
		case "identifier":
			imp.Alias = string(content[child.StartByte():child.EndByte()])
		}
	}

	// Check if local import
	imp.IsLocal = strings.HasPrefix(imp.Path, ".") || strings.HasPrefix(imp.Path, "@/")

	return imp
}

func (a *TypeScriptAnalyzer) parseImportClause(node *sitter.Node, content []byte) []string {
	var names []string

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "identifier" {
			names = append(names, string(content[n.StartByte():n.EndByte()]))
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return names
}

func (a *TypeScriptAnalyzer) parseRequire(node *sitter.Node, content []byte) *Import {
	// Handle require() calls
	imp := &Import{
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "string" {
			path := string(content[child.StartByte():child.EndByte()])
			imp.Path = strings.Trim(path, `"'`)
		}
	}

	imp.IsLocal = strings.HasPrefix(imp.Path, ".")
	return imp
}

// ExtractSymbols extracts all symbols from TypeScript source
func (a *TypeScriptAnalyzer) ExtractSymbols(node *sitter.Node, content []byte) []Symbol {
	var symbols []Symbol

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		switch n.Type() {
		case "function_declaration":
			if sym := a.parseFunction(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "class_declaration":
			if sym := a.parseClass(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "interface_declaration":
			if sym := a.parseInterface(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "type_alias_declaration":
			if sym := a.parseTypeAlias(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "lexical_declaration", "variable_declaration":
			symbols = append(symbols, a.parseVariables(n, content)...)
		case "enum_declaration":
			if sym := a.parseEnum(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return symbols
}

func (a *TypeScriptAnalyzer) parseFunction(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind:       SymbolKindFunction,
		Visibility: VisibilityPrivate,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			sym.Name = string(content[child.StartByte():child.EndByte()])
		case "formal_parameters":
			sym.Signature = string(content[child.StartByte():child.EndByte()])
		case "export":
			sym.Visibility = VisibilityPublic
		}
	}

	return sym
}

func (a *TypeScriptAnalyzer) parseClass(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind:       SymbolKindClass,
		Visibility: VisibilityPrivate,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "type_identifier", "identifier":
			sym.Name = string(content[child.StartByte():child.EndByte()])
		}
	}

	// Check parent for export
	if node.Parent() != nil && node.Parent().Type() == "export_statement" {
		sym.Visibility = VisibilityPublic
	}

	return sym
}

func (a *TypeScriptAnalyzer) parseInterface(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind:       SymbolKindInterface,
		Visibility: VisibilityPrivate,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			sym.Name = string(content[child.StartByte():child.EndByte()])
		}
	}

	if node.Parent() != nil && node.Parent().Type() == "export_statement" {
		sym.Visibility = VisibilityPublic
	}

	return sym
}

func (a *TypeScriptAnalyzer) parseTypeAlias(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind:       SymbolKindType,
		Visibility: VisibilityPrivate,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			sym.Name = string(content[child.StartByte():child.EndByte()])
		}
	}

	if node.Parent() != nil && node.Parent().Type() == "export_statement" {
		sym.Visibility = VisibilityPublic
	}

	return sym
}

func (a *TypeScriptAnalyzer) parseVariables(node *sitter.Node, content []byte) []Symbol {
	var symbols []Symbol

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "variable_declarator" {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					sym := Symbol{
						Name:       string(content[child.StartByte():child.EndByte()]),
						Kind:       SymbolKindVariable,
						Visibility: VisibilityPrivate,
						Location: Location{
							StartLine: int(child.StartPoint().Row) + 1,
							EndLine:   int(child.EndPoint().Row) + 1,
						},
					}
					symbols = append(symbols, sym)
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return symbols
}

func (a *TypeScriptAnalyzer) parseEnum(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind:       SymbolKindEnum,
		Visibility: VisibilityPrivate,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			sym.Name = string(content[child.StartByte():child.EndByte()])
		}
	}

	if node.Parent() != nil && node.Parent().Type() == "export_statement" {
		sym.Visibility = VisibilityPublic
	}

	return sym
}

// ExtractExports extracts exported symbols from TypeScript source
func (a *TypeScriptAnalyzer) ExtractExports(node *sitter.Node, content []byte, symbols []Symbol) []Export {
	var exports []Export

	// First, add all public symbols
	for _, sym := range symbols {
		if sym.Visibility == VisibilityPublic {
			exports = append(exports, Export{
				Name:       sym.Name,
				SymbolKind: sym.Kind,
			})
		}
	}

	// Also look for export statements
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "export_statement" {
			// Check for default export
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					exports = append(exports, Export{
						Name:      string(content[child.StartByte():child.EndByte()]),
						IsDefault: true,
					})
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return exports
}

// JavaScriptAnalyzer reuses TypeScript analyzer (subset)
type JavaScriptAnalyzer struct {
	TypeScriptAnalyzer
}

// PythonAnalyzer provides Python-specific AST analysis
type PythonAnalyzer struct{}

// ExtractPackage extracts module name
func (a *PythonAnalyzer) ExtractPackage(node *sitter.Node, content []byte) string {
	return ""
}

// ExtractImports extracts import statements
func (a *PythonAnalyzer) ExtractImports(node *sitter.Node, content []byte) []Import {
	var imports []Import

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		switch n.Type() {
		case "import_statement", "import_from_statement":
			imports = append(imports, a.parseImport(n, content)...)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return imports
}

func (a *PythonAnalyzer) parseImport(node *sitter.Node, content []byte) []Import {
	var imports []Import

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "dotted_name" {
			path := string(content[n.StartByte():n.EndByte()])
			imports = append(imports, Import{
				Path: path,
				Location: Location{
					StartLine: int(n.StartPoint().Row) + 1,
				},
				IsLocal: strings.HasPrefix(path, "."),
			})
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return imports
}

// ExtractSymbols extracts all symbols
func (a *PythonAnalyzer) ExtractSymbols(node *sitter.Node, content []byte) []Symbol {
	var symbols []Symbol

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		switch n.Type() {
		case "function_definition":
			if sym := a.parseFunction(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "class_definition":
			if sym := a.parseClass(n, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return symbols
}

func (a *PythonAnalyzer) parseFunction(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind: SymbolKindFunction,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			sym.Name = string(content[child.StartByte():child.EndByte()])
		}
	}

	// Python visibility by convention: _ prefix = private
	if strings.HasPrefix(sym.Name, "_") {
		sym.Visibility = VisibilityPrivate
	} else {
		sym.Visibility = VisibilityPublic
	}

	return sym
}

func (a *PythonAnalyzer) parseClass(node *sitter.Node, content []byte) *Symbol {
	sym := &Symbol{
		Kind: SymbolKindClass,
		Location: Location{
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
		},
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			sym.Name = string(content[child.StartByte():child.EndByte()])
		}
	}

	if strings.HasPrefix(sym.Name, "_") {
		sym.Visibility = VisibilityPrivate
	} else {
		sym.Visibility = VisibilityPublic
	}

	return sym
}

// ExtractExports - Python exports all public symbols
func (a *PythonAnalyzer) ExtractExports(node *sitter.Node, content []byte, symbols []Symbol) []Export {
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
