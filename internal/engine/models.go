package engine

import "fmt"

// Model represents a valid model identifier with its capability ranking.
// Rank 1 is the most capable; higher numbers are less capable.
// Models with the same rank are considered equivalent in capability.
type Model struct {
	ID   string
	Rank int
}

// ClaudeModels lists valid model identifiers for the Claude engine, ordered by capability.
var ClaudeModels = []Model{
	// Rank 1 — Frontier (opus-class, current generation)
	{ID: "claude-opus-4-6", Rank: 1},
	{ID: "opus", Rank: 1},
	{ID: "opus[1m]", Rank: 1},

	// Rank 2 — Strong (sonnet-class, current generation)
	{ID: "claude-sonnet-4-6", Rank: 2},
	{ID: "sonnet", Rank: 2},
	{ID: "sonnet[1m]", Rank: 2},
	{ID: "default", Rank: 2},
	{ID: "opusplan", Rank: 2},

	// Rank 3 — Previous-gen frontier
	{ID: "claude-3-opus-20240229", Rank: 3},

	// Rank 4 — Previous-gen strong
	{ID: "claude-3-5-sonnet-20241022", Rank: 4},

	// Rank 5 — Fast (haiku-class, current generation)
	{ID: "claude-haiku-4-5-20251001", Rank: 5},
	{ID: "haiku", Rank: 5},

	// Rank 6 — Previous-gen fast
	{ID: "claude-3-5-haiku-20241022", Rank: 6},

	// Rank 7 — Legacy
	{ID: "claude-3-sonnet-20240229", Rank: 7},
	{ID: "claude-3-haiku-20240307", Rank: 8},
}

// CodexModels lists valid model identifiers for the Codex engine, ordered by capability.
var CodexModels = []Model{
	// Rank 1 — Latest frontier
	{ID: "gpt-5.4", Rank: 1},

	// Rank 2 — Frontier codex-optimized
	{ID: "gpt-5.3-codex", Rank: 2},

	// Rank 3 — Previous frontier
	{ID: "gpt-5.2-codex", Rank: 3},
	{ID: "gpt-5.2", Rank: 4},

	// Rank 5 — Deep reasoning
	{ID: "gpt-5.1-codex-max", Rank: 5},
	{ID: "gpt-5.1-codex", Rank: 6},
	{ID: "gpt-5.1", Rank: 7},

	// Rank 8 — Older generation
	{ID: "gpt-5-codex", Rank: 8},
	{ID: "gpt-5", Rank: 9},

	// Rank 10 — Mini (speed/cost optimized)
	{ID: "gpt-5.1-codex-mini", Rank: 10},
	{ID: "gpt-5-codex-mini", Rank: 11},
}

// claudeModelSet and codexModelSet are lookup maps for validation.
var (
	claudeModelSet = buildModelSet(ClaudeModels)
	codexModelSet  = buildModelSet(CodexModels)
)

func buildModelSet(models []Model) map[string]int {
	m := make(map[string]int, len(models))
	for _, model := range models {
		m[model.ID] = model.Rank
	}
	return m
}

// ModelsForEngine returns the ranked model list for the given engine.
func ModelsForEngine(engineName string) ([]Model, error) {
	switch engineName {
	case "claude":
		return ClaudeModels, nil
	case "codex":
		return CodexModels, nil
	default:
		return nil, fmt.Errorf("unsupported engine: %s", engineName)
	}
}

// ModelRank returns the capability rank of a model for the given engine.
// Rank 1 is the most capable. Returns 0 if the model is not found.
func ModelRank(engineName, model string) int {
	switch engineName {
	case "claude":
		return claudeModelSet[model]
	case "codex":
		return codexModelSet[model]
	default:
		return 0
	}
}

// ValidateModel checks whether the given model string is valid for the specified engine.
// An empty model is always valid (the engine CLI will use its own default).
func ValidateModel(engineName, model string) error {
	if model == "" {
		return nil
	}
	switch engineName {
	case "claude":
		if _, ok := claudeModelSet[model]; !ok {
			return fmt.Errorf("invalid model %q for engine %s; valid models: %s", model, engineName, modelIDs(ClaudeModels))
		}
	case "codex":
		if _, ok := codexModelSet[model]; !ok {
			return fmt.Errorf("invalid model %q for engine %s; valid models: %s", model, engineName, modelIDs(CodexModels))
		}
	default:
		return fmt.Errorf("unsupported engine: %s", engineName)
	}
	return nil
}

func modelIDs(models []Model) string {
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	return fmt.Sprintf("%v", ids)
}
