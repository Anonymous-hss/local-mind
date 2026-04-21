// Package ast provides AST parsing and code intelligence using Tree-sitter.
// All CGO dependencies are isolated within this package.
package ast

// SymbolKind represents the type of a code symbol
type SymbolKind string

const (
	SymbolKindFunction  SymbolKind = "function"
	SymbolKindMethod    SymbolKind = "method"
	SymbolKindClass     SymbolKind = "class"
	SymbolKindInterface SymbolKind = "interface"
	SymbolKindStruct    SymbolKind = "struct"
	SymbolKindVariable  SymbolKind = "variable"
	SymbolKindConstant  SymbolKind = "constant"
	SymbolKindType      SymbolKind = "type"
	SymbolKindPackage   SymbolKind = "package"
	SymbolKindModule    SymbolKind = "module"
	SymbolKindEnum      SymbolKind = "enum"
	SymbolKindProperty  SymbolKind = "property"
)

// Visibility represents the visibility of a symbol
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityPrivate  Visibility = "private"
	VisibilityProtected Visibility = "protected"
	VisibilityInternal  Visibility = "internal"
)

// Location represents a position in source code
type Location struct {
	File      string `json:"file"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	StartCol  int    `json:"startCol"`
	EndCol    int    `json:"endCol"`
}

// Symbol represents a code symbol (function, class, variable, etc.)
type Symbol struct {
	Name       string     `json:"name"`
	Kind       SymbolKind `json:"kind"`
	Location   Location   `json:"location"`
	Visibility Visibility `json:"visibility"`
	Parent     string     `json:"parent,omitempty"`     // Parent class/interface name
	Signature  string     `json:"signature,omitempty"`  // For functions/methods
	DocComment string     `json:"docComment,omitempty"` // Documentation comment
	Children   []Symbol   `json:"children,omitempty"`   // Nested symbols
}

// Import represents an import/dependency
type Import struct {
	Path     string   `json:"path"`
	Alias    string   `json:"alias,omitempty"`
	Names    []string `json:"names,omitempty"` // For named imports
	Location Location `json:"location"`
	IsLocal  bool     `json:"isLocal"` // True if importing from same project
}

// Export represents an exported symbol
type Export struct {
	Name       string `json:"name"`
	Alias      string `json:"alias,omitempty"`
	IsDefault  bool   `json:"isDefault"`
	SymbolKind SymbolKind `json:"symbolKind"`
}

// ParseResult represents the result of parsing a file
type ParseResult struct {
	File       string   `json:"file"`
	Language   string   `json:"language"`
	Package    string   `json:"package,omitempty"`
	Imports    []Import `json:"imports"`
	Exports    []Export `json:"exports"`
	Symbols    []Symbol `json:"symbols"`
	Errors     []ParseError `json:"errors,omitempty"`
	ParseTimeMs int64   `json:"parseTimeMs"`
}

// ParseError represents a parsing error
type ParseError struct {
	Message  string   `json:"message"`
	Location Location `json:"location"`
	Severity string   `json:"severity"` // "error", "warning"
}

// FileNode represents a file in the dependency graph
type FileNode struct {
	Path       string   `json:"path"`
	Package    string   `json:"package"`
	Language   string   `json:"language"`
	Imports    []Import `json:"imports"`
	Exports    []Export `json:"exports"`
	Symbols    []Symbol `json:"symbols"`
}

// Edge represents a dependency edge
type Edge struct {
	From string `json:"from"` // file path
	To   string `json:"to"`   // file path
	Kind string `json:"kind"` // "import", "inherit", "implement"
}

// DependencyGraph represents the full dependency graph
type DependencyGraph struct {
	Nodes    map[string]*FileNode `json:"nodes"`
	Edges    []Edge               `json:"edges"`
	Circular [][]string           `json:"circular,omitempty"` // Detected circular deps
}

// BoundaryType represents the type of code boundary
type BoundaryType string

const (
	BoundaryTypePackage  BoundaryType = "package"
	BoundaryTypeModule   BoundaryType = "module"
	BoundaryTypeLayer    BoundaryType = "layer"
	BoundaryTypeFeature  BoundaryType = "feature"
)

// LayerType represents common architectural layers
type LayerType string

const (
	LayerAPI        LayerType = "api"
	LayerController LayerType = "controller"
	LayerService    LayerType = "service"
	LayerRepository LayerType = "repository"
	LayerModel      LayerType = "model"
	LayerUtil       LayerType = "util"
	LayerTest       LayerType = "test"
	LayerConfig     LayerType = "config"
	LayerUnknown    LayerType = "unknown"
)

// Boundary represents a code boundary (package, module, layer)
type Boundary struct {
	Path        string       `json:"path"`
	Name        string       `json:"name"`
	Type        BoundaryType `json:"type"`
	Layer       LayerType    `json:"layer,omitempty"`
	Files       []string     `json:"files"`
	SubBoundaries []string   `json:"subBoundaries,omitempty"`
}

// Language represents a supported programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguageTypeScript Language = "typescript"
	LanguageJavaScript Language = "javascript"
	LanguagePython     Language = "python"
	LanguageUnknown    Language = "unknown"
)

// LanguageFromExtension returns the language for a file extension
func LanguageFromExtension(ext string) Language {
	switch ext {
	case ".go":
		return LanguageGo
	case ".ts", ".tsx":
		return LanguageTypeScript
	case ".js", ".jsx", ".mjs", ".cjs":
		return LanguageJavaScript
	case ".py", ".pyw":
		return LanguagePython
	default:
		return LanguageUnknown
	}
}

// LanguageFromFilename returns the language from a filename
func LanguageFromFilename(filename string) Language {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return LanguageFromExtension(filename[i:])
		}
	}
	return LanguageUnknown
}
