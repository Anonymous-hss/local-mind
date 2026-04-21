package suggestion

import (
	"context"
	"strings"

	"github.com/localmind/core/internal/ast"
)

// SmellConfig configures smell detection thresholds
type SmellConfig struct {
	MaxFunctionLines int
	MaxNestingDepth  int
	MaxParamCount    int
	MaxClassLines    int
	MaxComplexity    int
}

// DefaultSmellConfig returns sensible defaults
func DefaultSmellConfig() *SmellConfig {
	return &SmellConfig{
		MaxFunctionLines: 50,
		MaxNestingDepth:  4,
		MaxParamCount:    5,
		MaxClassLines:    300,
		MaxComplexity:    10,
	}
}

// Analyzer detects code smells and suggests improvements
type Analyzer struct {
	parser *ast.Parser
	config *SmellConfig
}

// NewAnalyzer creates a new code analyzer
func NewAnalyzer(config *SmellConfig) *Analyzer {
	if config == nil {
		config = DefaultSmellConfig()
	}
	return &Analyzer{
		parser: ast.NewParser(),
		config: config,
	}
}

// AnalyzeFile analyzes a file for code smells
func (a *Analyzer) AnalyzeFile(filename string, content []byte) ([]CodeSmell, error) {
	var smells []CodeSmell

	// Analyze by line-based heuristics first (fast)
	smells = append(smells, a.detectLineBasedSmells(filename, content)...)

	// Then use AST for deeper analysis
	astSmells, err := a.detectASTSmells(filename, content)
	if err == nil {
		smells = append(smells, astSmells...)
	}

	return smells, nil
}

// detectLineBasedSmells uses simple line analysis for quick detection
func (a *Analyzer) detectLineBasedSmells(filename string, content []byte) []CodeSmell {
	var smells []CodeSmell
	lines := strings.Split(string(content), "\n")

	// Track nesting depth
	nestingDepth := 0
	maxNestingLine := 0
	maxNesting := 0

	// Track function boundaries for length detection
	type funcInfo struct {
		name       string
		startLine  int
		braceDepth int
	}
	var currentFunc *funcInfo

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect function declarations (simple heuristic)
		if strings.HasPrefix(trimmed, "func ") && strings.Contains(line, "{") {
			// Extract function name
			name := trimmed[5:] // after "func "
			if idx := strings.IndexByte(name, '('); idx > 0 {
				name = name[:idx]
			}
			currentFunc = &funcInfo{
				name:       name,
				startLine:  lineNum,
				braceDepth: 1, // the opening brace on this line
			}
		} else if currentFunc != nil {
			// Track braces within the current function
			openBraces := strings.Count(line, "{")
			closeBraces := strings.Count(line, "}")
			currentFunc.braceDepth += openBraces - closeBraces

			if currentFunc.braceDepth <= 0 {
				// Function ended
				funcLines := lineNum - currentFunc.startLine + 1
				if funcLines > a.config.MaxFunctionLines {
					smells = append(smells, CodeSmell{
						Type:         SmellLongFunction,
						File:         filename,
						StartLine:    currentFunc.startLine,
						EndLine:      lineNum,
						Description:  "Function is too long (" + itoa(funcLines) + " lines) - consider extracting logic",
						Severity:     "warning",
						FixAvailable: true,
					})
				}
				currentFunc = nil
			}
		}

		// Count braces for nesting (global)
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")
		nestingDepth += openBraces - closeBraces

		if nestingDepth > maxNesting {
			maxNesting = nestingDepth
			maxNestingLine = lineNum
		}

		// Detect magic numbers
		if a.hasMagicNumber(line) {
			smells = append(smells, CodeSmell{
				Type:         SmellMagicNumber,
				File:         filename,
				StartLine:    lineNum,
				EndLine:      lineNum,
				Description:  "Magic number detected - consider using a named constant",
				Severity:     "info",
				FixAvailable: true,
			})
		}

		// Detect long lines
		if len(line) > 120 {
			smells = append(smells, CodeSmell{
				Type:         SmellType("long_line"),
				File:         filename,
				StartLine:    lineNum,
				EndLine:      lineNum,
				Description:  "Line exceeds 120 characters",
				Severity:     "info",
				FixAvailable: false,
			})
		}
	}

	// Report deep nesting
	if maxNesting > a.config.MaxNestingDepth {
		smells = append(smells, CodeSmell{
			Type:         SmellDeepNesting,
			File:         filename,
			StartLine:    maxNestingLine,
			EndLine:      maxNestingLine,
			Description:  "Deep nesting detected - consider extracting to a function",
			Severity:     "warning",
			FixAvailable: true,
		})
	}

	return smells
}

// hasMagicNumber detects magic numbers in code
func (a *Analyzer) hasMagicNumber(line string) bool {
	// Skip comments
	if strings.HasPrefix(strings.TrimSpace(line), "//") ||
		strings.HasPrefix(strings.TrimSpace(line), "#") ||
		strings.HasPrefix(strings.TrimSpace(line), "/*") {
		return false
	}

	// Skip common allowed numbers
	allowed := map[string]bool{
		"0": true, "1": true, "-1": true,
		"2": true, "10": true, "100": true,
		"true": true, "false": true,
	}

	// Simple regex-like check for numbers
	words := strings.Fields(line)
	for _, word := range words {
		// Check if it looks like a number
		if len(word) > 0 && (word[0] >= '0' && word[0] <= '9') {
			// Not in allowed list and not a common pattern
			if !allowed[word] && len(word) > 2 {
				return true
			}
		}
	}

	return false
}

// detectASTSmells uses AST for deeper analysis
func (a *Analyzer) detectASTSmells(filename string, content []byte) ([]CodeSmell, error) {
	var smells []CodeSmell

	// Parse the file
	result, err := a.parser.Parse(context.Background(), filename, content)
	if err != nil {
		return nil, err
	}

	// Analyze each symbol
	for _, symbol := range result.Symbols {
		switch symbol.Kind {
		case ast.SymbolKindFunction, ast.SymbolKindMethod:
			funcSmells := a.analyzeFunctionSymbol(filename, symbol)
			smells = append(smells, funcSmells...)

		case ast.SymbolKindClass, ast.SymbolKindStruct:
			classSmells := a.analyzeClassSymbol(filename, symbol)
			smells = append(smells, classSmells...)
		}
	}

	return smells, nil
}

// analyzeFunctionSymbol checks a function/method for smells
func (a *Analyzer) analyzeFunctionSymbol(filename string, symbol ast.Symbol) []CodeSmell {
	var smells []CodeSmell

	// Check function length
	lines := symbol.Location.EndLine - symbol.Location.StartLine + 1
	if lines > a.config.MaxFunctionLines {
		smells = append(smells, CodeSmell{
			Type:         SmellLongFunction,
			File:         filename,
			StartLine:    symbol.Location.StartLine,
			EndLine:      symbol.Location.EndLine,
			Description:  "Function is " + itoa(lines) + " lines - consider extracting logic",
			Severity:     "warning",
			FixAvailable: true,
		})
	}

	// Check parameter count (from signature)
	if symbol.Signature != "" {
		paramCount := countParams(symbol.Signature)
		if paramCount > a.config.MaxParamCount {
			smells = append(smells, CodeSmell{
				Type:         SmellLongParamList,
				File:         filename,
				StartLine:    symbol.Location.StartLine,
				EndLine:      symbol.Location.StartLine,
				Description:  "Function has " + itoa(paramCount) + " parameters - consider using a struct",
				Severity:     "info",
				FixAvailable: true,
			})
		}
	}

	return smells
}

// countParams counts parameters in a function signature
func countParams(signature string) int {
	if signature == "()" || signature == "" {
		return 0
	}
	// Simple comma counting
	return strings.Count(signature, ",") + 1
}

// analyzeClassSymbol checks a class/struct for smells
func (a *Analyzer) analyzeClassSymbol(filename string, symbol ast.Symbol) []CodeSmell {
	var smells []CodeSmell

	// Check class size
	lines := symbol.Location.EndLine - symbol.Location.StartLine + 1
	if lines > a.config.MaxClassLines {
		smells = append(smells, CodeSmell{
			Type:         SmellLargeClass,
			File:         filename,
			StartLine:    symbol.Location.StartLine,
			EndLine:      symbol.Location.EndLine,
			Description:  "Class is " + itoa(lines) + " lines - consider splitting",
			Severity:     "warning",
			FixAvailable: false,
		})
	}

	return smells
}

// AnalyzeSelection analyzes a specific code selection
func (a *Analyzer) AnalyzeSelection(filename string, content []byte, startLine, endLine int) ([]CodeSmell, error) {
	// Get full file smells first
	allSmells, err := a.AnalyzeFile(filename, content)
	if err != nil {
		return nil, err
	}

	// Filter to selection range
	var relevant []CodeSmell
	for _, smell := range allSmells {
		if smell.StartLine >= startLine && smell.EndLine <= endLine {
			relevant = append(relevant, smell)
		}
	}

	return relevant, nil
}
