package suggestion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/localmind/core/pkg/ollama"
)

// Generator creates code suggestions using LLM
type Generator struct {
	client   *ollama.Client
	model    string
	analyzer *Analyzer
}

// NewGenerator creates a new suggestion generator
func NewGenerator(client *ollama.Client, model string) *Generator {
	return &Generator{
		client:   client,
		model:    model,
		analyzer: NewAnalyzer(nil),
	}
}

// GenerateFromSmell generates a fix suggestion for a code smell
func (g *Generator) GenerateFromSmell(ctx context.Context, smell CodeSmell, code string) (*Suggestion, error) {
	// Build prompt based on smell type
	prompt := g.buildSmellPrompt(smell, code)

	// Get suggestion from LLM
	response, err := g.queryLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse response into suggestion
	suggestion := g.parseResponse(response, smell)
	suggestion.Type = SuggestionTypeSmell
	suggestion.SourceFile = smell.File
	suggestion.StartLine = smell.StartLine
	suggestion.EndLine = smell.EndLine

	return suggestion, nil
}

// GenerateRefactor generates a refactoring suggestion
func (g *Generator) GenerateRefactor(ctx context.Context, req *SuggestionRequest) (*Suggestion, error) {
	startTime := time.Now()

	prompt := g.buildRefactorPrompt(req)

	response, err := g.queryLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}

	suggestion := g.parseRefactorResponse(response, req)
	suggestion.Type = SuggestionTypeRefactor
	suggestion.SourceFile = req.File
	suggestion.StartLine = req.StartLine
	suggestion.EndLine = req.EndLine
	suggestion.CreatedAt = startTime

	return suggestion, nil
}

// buildSmellPrompt creates the prompt for smell fixes
func (g *Generator) buildSmellPrompt(smell CodeSmell, code string) string {
	var prompt strings.Builder

	prompt.WriteString("You are a code refactoring expert. ")
	prompt.WriteString("Provide a fix for the following code smell.\n\n")

	prompt.WriteString("SMELL TYPE: " + string(smell.Type) + "\n")
	prompt.WriteString("DESCRIPTION: " + smell.Description + "\n\n")

	prompt.WriteString("ORIGINAL CODE:\n```\n")
	prompt.WriteString(code)
	prompt.WriteString("\n```\n\n")

	prompt.WriteString("Respond with:\n")
	prompt.WriteString("1. EXPLANATION: Why this fix improves the code (1-2 sentences)\n")
	prompt.WriteString("2. FIXED CODE: The improved code block\n")
	prompt.WriteString("3. RISKS: Any potential risks (or 'none')\n")

	return prompt.String()
}

// buildRefactorPrompt creates the prompt for general refactoring
func (g *Generator) buildRefactorPrompt(req *SuggestionRequest) string {
	var prompt strings.Builder

	prompt.WriteString("You are a code refactoring expert. ")
	prompt.WriteString("Suggest an improvement for the following code.\n\n")

	if req.Intent != "" {
		prompt.WriteString("USER INTENT: " + req.Intent + "\n\n")
	}

	prompt.WriteString("CODE:\n```\n")
	prompt.WriteString(req.Content)
	prompt.WriteString("\n```\n\n")

	if req.Context != "" {
		prompt.WriteString("CONTEXT:\n```\n")
		prompt.WriteString(req.Context)
		prompt.WriteString("\n```\n\n")
	}

	prompt.WriteString("Respond with:\n")
	prompt.WriteString("1. TITLE: A short title for this refactor\n")
	prompt.WriteString("2. EXPLANATION: Why this improves the code (2-3 sentences)\n")
	prompt.WriteString("3. REFACTORED CODE: The improved code block\n")
	prompt.WriteString("4. RISKS: Any potential risks (or 'none')\n")

	return prompt.String()
}

// queryLLM sends prompt to Ollama
func (g *Generator) queryLLM(ctx context.Context, prompt string) (string, error) {
	req := &ollama.GenerateRequest{
		Model:  g.model,
		Prompt: prompt,
		Options: &ollama.ModelOptions{
			NumPredict:  500,
			Temperature: 0.3, // More deterministic for refactoring
		},
	}

	var response strings.Builder
	_, err := g.client.Generate(ctx, req, func(token string, done bool) error {
		response.WriteString(token)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("LLM query failed: %w", err)
	}

	return response.String(), nil
}

// parseResponse parses LLM response into a suggestion
func (g *Generator) parseResponse(response string, smell CodeSmell) *Suggestion {
	suggestion := &Suggestion{
		ID:         generateSuggestionID(),
		CreatedAt:  time.Now(),
		Confidence: ConfidenceMedium,
		Risk:       RiskLow,
	}

	// Parse explanation
	if idx := strings.Index(response, "EXPLANATION:"); idx != -1 {
		end := strings.Index(response[idx:], "\n2.")
		if end == -1 {
			end = len(response) - idx
		}
		suggestion.Explanation = strings.TrimSpace(response[idx+12 : idx+end])
	}

	// Parse fixed code
	if idx := strings.Index(response, "FIXED CODE:"); idx != -1 {
		// Find code block
		startCode := strings.Index(response[idx:], "```")
		if startCode != -1 {
			startCode += idx + 3
			// Skip language identifier
			if nl := strings.Index(response[startCode:], "\n"); nl != -1 {
				startCode += nl + 1
			}
			endCode := strings.Index(response[startCode:], "```")
			if endCode != -1 {
				fixedCode := response[startCode : startCode+endCode]
				
				// Create diff
				suggestion.Diff = &Diff{
					File: smell.File,
					Hunks: []Hunk{
						{
							StartLineOld: smell.StartLine,
							EndLineOld:   smell.EndLine,
							StartLineNew: smell.StartLine,
							EndLineNew:   smell.StartLine + strings.Count(fixedCode, "\n"),
							Before:       "", // Would need original code
							After:        strings.TrimSpace(fixedCode),
						},
					},
				}
			}
		}
	}

	// Parse risks
	if idx := strings.Index(response, "RISKS:"); idx != -1 {
		riskText := strings.TrimSpace(response[idx+6:])
		if strings.HasPrefix(strings.ToLower(riskText), "none") {
			suggestion.Risk = RiskLow
		} else {
			suggestion.Risk = RiskMedium
			suggestion.RiskDetails = []string{riskText}
		}
	}

	// Set title based on smell type
	suggestion.Title = smellToTitle(smell.Type)

	return suggestion
}

// parseRefactorResponse parses refactor response
func (g *Generator) parseRefactorResponse(response string, req *SuggestionRequest) *Suggestion {
	suggestion := &Suggestion{
		ID:         generateSuggestionID(),
		CreatedAt:  time.Now(),
		Confidence: ConfidenceMedium,
		Risk:       RiskLow,
	}

	// Parse title
	if idx := strings.Index(response, "TITLE:"); idx != -1 {
		end := strings.Index(response[idx:], "\n")
		if end == -1 {
			end = len(response) - idx
		}
		suggestion.Title = strings.TrimSpace(response[idx+6 : idx+end])
	}

	// Parse explanation
	if idx := strings.Index(response, "EXPLANATION:"); idx != -1 {
		end := strings.Index(response[idx:], "\n3.")
		if end == -1 {
			end = strings.Index(response[idx:], "REFACTORED")
		}
		if end == -1 {
			end = len(response) - idx
		}
		suggestion.Explanation = strings.TrimSpace(response[idx+12 : idx+end])
	}

	// Parse refactored code
	if idx := strings.Index(response, "REFACTORED CODE:"); idx != -1 {
		startCode := strings.Index(response[idx:], "```")
		if startCode != -1 {
			startCode += idx + 3
			if nl := strings.Index(response[startCode:], "\n"); nl != -1 {
				startCode += nl + 1
			}
			endCode := strings.Index(response[startCode:], "```")
			if endCode != -1 {
				newCode := response[startCode : startCode+endCode]

				suggestion.Diff = &Diff{
					File: req.File,
					Hunks: []Hunk{
						{
							StartLineOld: req.StartLine,
							EndLineOld:   req.EndLine,
							StartLineNew: req.StartLine,
							EndLineNew:   req.StartLine + strings.Count(newCode, "\n"),
							Before:       req.Content,
							After:        strings.TrimSpace(newCode),
						},
					},
				}
			}
		}
	}

	return suggestion
}

// generateSuggestionID creates a unique ID
func generateSuggestionID() string {
	return "sugg-" + time.Now().Format("20060102150405")
}

// smellToTitle converts a smell type to a human-readable title
func smellToTitle(smell SmellType) string {
	titles := map[SmellType]string{
		SmellLongFunction:    "Extract Long Function",
		SmellDeepNesting:     "Reduce Nesting Depth",
		SmellDuplicateCode:   "Remove Duplicate Code",
		SmellLargeClass:      "Split Large Class",
		SmellLongParamList:   "Simplify Parameters",
		SmellComplexCondition: "Simplify Condition",
		SmellMagicNumber:     "Extract Constant",
		SmellDeadCode:        "Remove Dead Code",
	}

	if title, ok := titles[smell]; ok {
		return title
	}
	return "Code Improvement"
}

// GenerateResult represents the result of generating multiple suggestions
type GenerateResult struct {
	Suggestions []Suggestion
	Errors      []string
	TimeMs      int64
}

// GenerateMultiple generates suggestions for multiple smells
func (g *Generator) GenerateMultiple(ctx context.Context, smells []CodeSmell, code string) *GenerateResult {
	startTime := time.Now()
	result := &GenerateResult{}

	for _, smell := range smells {
		if !smell.FixAvailable {
			continue
		}

		sugg, err := g.GenerateFromSmell(ctx, smell, code)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		result.Suggestions = append(result.Suggestions, *sugg)
	}

	result.TimeMs = time.Since(startTime).Milliseconds()
	return result
}

// SuggestionJSON is for JSON output
type SuggestionJSON struct {
	Suggestion
}

// ToJSON converts suggestion to JSON
func (s *Suggestion) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
