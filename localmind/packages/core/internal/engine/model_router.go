package engine

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/localmind/core/internal/ollama"
	"github.com/localmind/core/pkg/protocol"
)

// =============================================================================
// Capability Definitions
// =============================================================================

// Capability represents a specific skill a model can have
type Capability string

const (
	CapCodeCompletion Capability = "code_completion" // Fast fill-in-middle, autocomplete
	CapCodeGeneration Capability = "code_generation" // Generating new code blocks
	CapReasoning      Capability = "reasoning"       // Multi-step planning, logic
	CapRefactoring    Capability = "refactoring"     // Understanding + rewriting code
	CapExplanation    Capability = "explanation"     // Explaining code in natural language
	CapInstruction    Capability = "instruction"     // Following complex instructions
)

// CapabilityScore is 0.0–1.0 indicating how well a model handles a capability
type CapabilityScore map[Capability]float64

// ModelProfile describes a model's known capabilities and characteristics
type ModelProfile struct {
	Name         string
	Family       string // e.g. "qwen2.5-coder", "deepseek-coder", "codellama"
	ParamSize    string // e.g. "1.5b", "7b", "14b"
	SizeBytes    int64
	Capabilities CapabilityScore
	SpeedTier    int // 1=fastest, 3=slowest (relative)
}

// RoleRequirement defines what capabilities a role needs, with priority weights
type RoleRequirement struct {
	Role        protocol.ModelRole
	Required    []Capability // Must-have capabilities
	Preferred   []Capability // Nice-to-have capabilities
	SpeedWeight float64      // 0.0–1.0: how much speed matters (1.0 = speed critical)
}

// =============================================================================
// Known Model Profiles (capability database)
// =============================================================================

// knownProfiles maps model family patterns to their capability scores.
// This is the "brain" that lets us assign tasks intelligently.
var knownProfiles = map[string]CapabilityScore{
	// Qwen 2.5 Coder family — excellent code models
	"qwen2.5-coder:0.5b": {
		CapCodeCompletion: 0.7, CapCodeGeneration: 0.4, CapReasoning: 0.2,
		CapRefactoring: 0.3, CapExplanation: 0.2, CapInstruction: 0.2,
	},
	"qwen2.5-coder:1.5b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.6, CapReasoning: 0.4,
		CapRefactoring: 0.5, CapExplanation: 0.4, CapInstruction: 0.4,
	},
	"qwen2.5-coder:3b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.7, CapReasoning: 0.5,
		CapRefactoring: 0.6, CapExplanation: 0.6, CapInstruction: 0.6,
	},
	"qwen2.5-coder:7b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.85, CapReasoning: 0.7,
		CapRefactoring: 0.8, CapExplanation: 0.75, CapInstruction: 0.75,
	},
	"qwen2.5-coder:14b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.9, CapReasoning: 0.8,
		CapRefactoring: 0.85, CapExplanation: 0.85, CapInstruction: 0.85,
	},
	"qwen2.5-coder:32b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.95, CapReasoning: 0.9,
		CapRefactoring: 0.9, CapExplanation: 0.9, CapInstruction: 0.9,
	},

	// DeepSeek Coder family
	"deepseek-coder:1.3b": {
		CapCodeCompletion: 0.8, CapCodeGeneration: 0.5, CapReasoning: 0.3,
		CapRefactoring: 0.4, CapExplanation: 0.3, CapInstruction: 0.3,
	},
	"deepseek-coder:6.7b": {
		CapCodeCompletion: 0.85, CapCodeGeneration: 0.75, CapReasoning: 0.6,
		CapRefactoring: 0.7, CapExplanation: 0.65, CapInstruction: 0.65,
	},
	"deepseek-coder:33b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.9, CapReasoning: 0.8,
		CapRefactoring: 0.85, CapExplanation: 0.85, CapInstruction: 0.85,
	},

	// Code Llama family
	"codellama:7b": {
		CapCodeCompletion: 0.85, CapCodeGeneration: 0.75, CapReasoning: 0.5,
		CapRefactoring: 0.65, CapExplanation: 0.6, CapInstruction: 0.55,
	},
	"codellama:13b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.8, CapReasoning: 0.65,
		CapRefactoring: 0.75, CapExplanation: 0.7, CapInstruction: 0.65,
	},
	"codellama:34b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.85, CapReasoning: 0.75,
		CapRefactoring: 0.8, CapExplanation: 0.8, CapInstruction: 0.75,
	},

	// StarCoder2 family
	"starcoder2:3b": {
		CapCodeCompletion: 0.85, CapCodeGeneration: 0.6, CapReasoning: 0.3,
		CapRefactoring: 0.45, CapExplanation: 0.35, CapInstruction: 0.3,
	},
	"starcoder2:7b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.75, CapReasoning: 0.5,
		CapRefactoring: 0.65, CapExplanation: 0.55, CapInstruction: 0.5,
	},
	"starcoder2:15b": {
		CapCodeCompletion: 0.9, CapCodeGeneration: 0.85, CapReasoning: 0.65,
		CapRefactoring: 0.75, CapExplanation: 0.7, CapInstruction: 0.65,
	},

	// General-purpose models (weaker at code but good at reasoning)
	"llama3:8b": {
		CapCodeCompletion: 0.5, CapCodeGeneration: 0.5, CapReasoning: 0.7,
		CapRefactoring: 0.5, CapExplanation: 0.8, CapInstruction: 0.75,
	},
	"llama3:70b": {
		CapCodeCompletion: 0.6, CapCodeGeneration: 0.7, CapReasoning: 0.9,
		CapRefactoring: 0.7, CapExplanation: 0.9, CapInstruction: 0.9,
	},
	"mistral:7b": {
		CapCodeCompletion: 0.5, CapCodeGeneration: 0.55, CapReasoning: 0.65,
		CapRefactoring: 0.5, CapExplanation: 0.7, CapInstruction: 0.7,
	},
	"phi3:mini": {
		CapCodeCompletion: 0.6, CapCodeGeneration: 0.5, CapReasoning: 0.6,
		CapRefactoring: 0.45, CapExplanation: 0.6, CapInstruction: 0.6,
	},
}

// roleRequirements defines what each role needs from a model
var roleRequirements = map[protocol.ModelRole]RoleRequirement{
	protocol.ModelRoleCompletion: {
		Role:        protocol.ModelRoleCompletion,
		Required:    []Capability{CapCodeCompletion},
		Preferred:   []Capability{CapCodeGeneration},
		SpeedWeight: 1.0, // Speed is everything for autocomplete
	},
	protocol.ModelRoleSuggestion: {
		Role:        protocol.ModelRoleSuggestion,
		Required:    []Capability{CapRefactoring, CapExplanation},
		Preferred:   []Capability{CapCodeGeneration, CapInstruction},
		SpeedWeight: 0.4, // Quality > speed, but don't be too slow
	},
	protocol.ModelRoleAgent: {
		Role:        protocol.ModelRoleAgent,
		Required:    []Capability{CapReasoning, CapInstruction},
		Preferred:   []Capability{CapCodeGeneration, CapRefactoring},
		SpeedWeight: 0.1, // Quality is paramount, speed doesn't matter much
	},
}

// =============================================================================
// ModelRouter
// =============================================================================

// ModelRouter auto-discovers installed Ollama models and assigns each to a role
// based on a capability profiling framework.
//
// Each model is scored against known capability profiles. Each role has required
// and preferred capabilities with speed/quality trade-off weights. The router
// finds the best model for each role using a weighted scoring system.
type ModelRouter struct {
	client *ollama.Client
	logger *log.Logger

	// Discovered models (populated by Discover())
	models   []ollama.ModelInfo
	profiles []ModelProfile

	// Active role → model name assignments
	assignments map[string]string
}

// NewModelRouter creates a new model router
func NewModelRouter(client *ollama.Client, logger *log.Logger) *ModelRouter {
	if logger == nil {
		logger = log.Default()
	}
	return &ModelRouter{
		client:      client,
		logger:      logger,
		assignments: make(map[string]string),
	}
}

// Discover queries Ollama for available models, profiles them, and assigns roles.
// Should be called at startup. Can be re-called to refresh.
func (r *ModelRouter) Discover(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	models, err := r.client.ListModels(ctx)
	if err != nil {
		r.logger.Printf("[ModelRouter] Failed to discover models: %v", err)
		return err
	}

	r.models = models
	r.logger.Printf("[ModelRouter] Discovered %d model(s)", len(models))

	// Profile each model
	r.profiles = make([]ModelProfile, 0, len(models))
	for _, m := range models {
		profile := r.profileModel(m)
		r.profiles = append(r.profiles, profile)
		r.logger.Printf("[ModelRouter]   %s [family=%s, params=%s, speed=%d]",
			profile.Name, profile.Family, profile.ParamSize, profile.SpeedTier)
	}

	// Assign roles based on capability matching
	r.assignRoles()
	return nil
}

// profileModel creates a capability profile for a discovered model
func (r *ModelRouter) profileModel(m ollama.ModelInfo) ModelProfile {
	name := m.Name
	family, paramSize := parseModelName(name)

	profile := ModelProfile{
		Name:      name,
		Family:    family,
		ParamSize: paramSize,
		SizeBytes: m.Size,
		SpeedTier: estimateSpeedTier(m.Size),
	}

	// Look up known capability scores
	if scores, ok := knownProfiles[name]; ok {
		profile.Capabilities = scores
	} else if scores, ok := knownProfiles[family+":"+paramSize]; ok {
		profile.Capabilities = scores
	} else {
		// Unknown model — estimate capabilities from size and family
		profile.Capabilities = estimateCapabilities(family, m.Size)
	}

	return profile
}

// parseModelName splits "qwen2.5-coder:1.5b" into ("qwen2.5-coder", "1.5b")
func parseModelName(name string) (string, string) {
	parts := strings.SplitN(name, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return name, "unknown"
}

// estimateSpeedTier guesses speed from model size (bytes)
// 1 = fastest (< 2GB), 2 = medium (2-8GB), 3 = slow (> 8GB)
func estimateSpeedTier(sizeBytes int64) int {
	gb := float64(sizeBytes) / (1024 * 1024 * 1024)
	if gb < 2.0 {
		return 1
	}
	if gb < 8.0 {
		return 2
	}
	return 3
}

// estimateCapabilities provides fallback scores for unknown models
func estimateCapabilities(family string, sizeBytes int64) CapabilityScore {
	isCode := strings.Contains(strings.ToLower(family), "code") ||
		strings.Contains(strings.ToLower(family), "coder") ||
		strings.Contains(strings.ToLower(family), "starcoder") ||
		strings.Contains(strings.ToLower(family), "deepseek")

	gb := float64(sizeBytes) / (1024 * 1024 * 1024)

	// Base scores scale with model size
	base := 0.3
	if gb > 2 {
		base = 0.5
	}
	if gb > 5 {
		base = 0.7
	}
	if gb > 15 {
		base = 0.85
	}

	scores := CapabilityScore{
		CapCodeCompletion: base,
		CapCodeGeneration: base - 0.1,
		CapReasoning:      base - 0.1,
		CapRefactoring:    base - 0.15,
		CapExplanation:    base - 0.1,
		CapInstruction:    base - 0.1,
	}

	// Code-specific models get a boost to code capabilities
	if isCode {
		scores[CapCodeCompletion] += 0.2
		scores[CapCodeGeneration] += 0.15
		scores[CapRefactoring] += 0.1
	}

	// Clamp to [0, 1]
	for k, v := range scores {
		if v > 1.0 {
			scores[k] = 1.0
		}
		if v < 0.0 {
			scores[k] = 0.0
		}
	}

	return scores
}

// =============================================================================
// Role Assignment (the core matching algorithm)
// =============================================================================

// assignRoles matches models to roles using capability scoring
func (r *ModelRouter) assignRoles() {
	if len(r.profiles) == 0 {
		r.logger.Println("[ModelRouter] No models available — all roles unassigned")
		return
	}

	// Single model → all roles
	if len(r.profiles) == 1 {
		name := r.profiles[0].Name
		r.assignments[string(protocol.ModelRoleCompletion)] = name
		r.assignments[string(protocol.ModelRoleSuggestion)] = name
		r.assignments[string(protocol.ModelRoleAgent)] = name
		r.logger.Printf("[ModelRouter] Single model '%s' → all roles", name)
		return
	}

	// Score each model for each role and pick the best
	for role, req := range roleRequirements {
		bestModel := ""
		bestScore := -1.0

		for _, profile := range r.profiles {
			score := r.scoreModelForRole(profile, req)
			if score > bestScore {
				bestScore = score
				bestModel = profile.Name
			}
		}

		if bestModel != "" {
			r.assignments[string(role)] = bestModel
		}
	}

	r.logger.Println("[ModelRouter] Capability-based role assignments:")
	for role, model := range r.assignments {
		r.logger.Printf("[ModelRouter]   %s → %s", role, model)
	}
}

// scoreModelForRole computes a fitness score (0.0–1.0) for a model against a role
func (r *ModelRouter) scoreModelForRole(profile ModelProfile, req RoleRequirement) float64 {
	caps := profile.Capabilities
	if caps == nil {
		return 0.0
	}

	var totalScore float64
	var totalWeight float64

	// Required capabilities (weight = 2.0 each)
	for _, cap := range req.Required {
		score, ok := caps[cap]
		if !ok {
			score = 0.0
		}
		totalScore += score * 2.0
		totalWeight += 2.0
	}

	// Preferred capabilities (weight = 1.0 each)
	for _, cap := range req.Preferred {
		score, ok := caps[cap]
		if !ok {
			score = 0.0
		}
		totalScore += score * 1.0
		totalWeight += 1.0
	}

	// Normalize capability score to 0–1
	capScore := 0.0
	if totalWeight > 0 {
		capScore = totalScore / totalWeight
	}

	// Speed score: 1.0 for tier 1, 0.5 for tier 2, 0.0 for tier 3
	speedScore := 0.0
	switch profile.SpeedTier {
	case 1:
		speedScore = 1.0
	case 2:
		speedScore = 0.5
	case 3:
		speedScore = 0.0
	}

	// Blend capability and speed scores using the role's speed weight
	// High SpeedWeight → prefer faster models even if less capable
	// Low SpeedWeight  → prefer more capable models even if slower
	finalScore := capScore*(1.0-req.SpeedWeight) + speedScore*req.SpeedWeight

	return finalScore
}

// =============================================================================
// Public API
// =============================================================================

// GetModelForRole returns the model assigned to a role. Empty if none.
func (r *ModelRouter) GetModelForRole(role protocol.ModelRole) string {
	return r.assignments[string(role)]
}

// GetActiveModels returns the current role → model map (copy)
func (r *ModelRouter) GetActiveModels() map[string]string {
	result := make(map[string]string, len(r.assignments))
	for k, v := range r.assignments {
		result[k] = v
	}
	return result
}

// GetModelsInfo returns protocol-friendly model info with roles annotated
func (r *ModelRouter) GetModelsInfo() []protocol.ModelInfo {
	// Build reverse map: model name → assigned roles
	roleByModel := make(map[string]string)
	for role, model := range r.assignments {
		if existing, ok := roleByModel[model]; ok {
			roleByModel[model] = existing + ", " + role
		} else {
			roleByModel[model] = role
		}
	}

	var result []protocol.ModelInfo
	for _, m := range r.models {
		result = append(result, protocol.ModelInfo{
			Name: m.Name,
			Size: m.Size,
			Role: roleByModel[m.Name],
		})
	}
	return result
}

// GetProfiles returns the full capability profiles (useful for debugging)
func (r *ModelRouter) GetProfiles() []ModelProfile {
	return r.profiles
}

// HasModels returns true if at least one model is available
func (r *ModelRouter) HasModels() bool {
	return len(r.models) > 0
}

// GetSortedModelsForCapability returns models sorted best→worst for a capability
func (r *ModelRouter) GetSortedModelsForCapability(cap Capability) []string {
	type scored struct {
		name  string
		score float64
	}
	var items []scored
	for _, p := range r.profiles {
		s := p.Capabilities[cap]
		items = append(items, scored{name: p.Name, score: s})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})
	var names []string
	for _, item := range items {
		names = append(names, item.name)
	}
	return names
}
