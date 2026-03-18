package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateModelClaude(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateModel("claude", "claude-opus-4-6"))
	require.NoError(t, ValidateModel("claude", "claude-sonnet-4-6"))
	require.NoError(t, ValidateModel("claude", "sonnet"))
	require.NoError(t, ValidateModel("claude", "opus[1m]"))
	require.NoError(t, ValidateModel("claude", ""))

	err := ValidateModel("claude", "gpt-5.4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model")
	assert.Contains(t, err.Error(), "claude")

	err = ValidateModel("claude", "nonexistent")
	require.Error(t, err)
}

func TestValidateModelCodex(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateModel("codex", "gpt-5.4"))
	require.NoError(t, ValidateModel("codex", "gpt-5.1-codex"))
	require.NoError(t, ValidateModel("codex", "gpt-5-codex-mini"))
	require.NoError(t, ValidateModel("codex", ""))

	err := ValidateModel("codex", "claude-opus-4-6")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model")
	assert.Contains(t, err.Error(), "codex")
}

func TestValidateModelUnsupportedEngine(t *testing.T) {
	t.Parallel()

	err := ValidateModel("gemini", "some-model")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported engine")
}

func TestModelsForEngine(t *testing.T) {
	t.Parallel()

	models, err := ModelsForEngine("claude")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
	assert.Equal(t, "claude-opus-4-6", models[0].ID)
	assert.Equal(t, 1, models[0].Rank)

	models, err = ModelsForEngine("codex")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
	assert.Equal(t, "gpt-5.4", models[0].ID)
	assert.Equal(t, 1, models[0].Rank)

	_, err = ModelsForEngine("unknown")
	require.Error(t, err)
}

func TestModelRank(t *testing.T) {
	t.Parallel()

	// Claude: opus is smartest (rank 1), haiku is fast (rank 5)
	assert.Equal(t, 1, ModelRank("claude", "claude-opus-4-6"))
	assert.Equal(t, 1, ModelRank("claude", "opus"))
	assert.Equal(t, 2, ModelRank("claude", "claude-sonnet-4-6"))
	assert.Equal(t, 2, ModelRank("claude", "sonnet"))
	assert.Equal(t, 5, ModelRank("claude", "claude-haiku-4-5-20251001"))
	assert.Equal(t, 5, ModelRank("claude", "haiku"))

	// Aliases share rank with their full model ID
	assert.Equal(t, ModelRank("claude", "claude-opus-4-6"), ModelRank("claude", "opus[1m]"))
	assert.Equal(t, ModelRank("claude", "claude-sonnet-4-6"), ModelRank("claude", "sonnet[1m]"))

	// Codex: gpt-5.4 is smartest, mini is cheapest
	assert.Equal(t, 1, ModelRank("codex", "gpt-5.4"))
	assert.Equal(t, 11, ModelRank("codex", "gpt-5-codex-mini"))

	// Unknown model returns 0
	assert.Equal(t, 0, ModelRank("claude", "nonexistent"))
	assert.Equal(t, 0, ModelRank("unknown-engine", "gpt-5.4"))
}

func TestModelRankOrdering(t *testing.T) {
	t.Parallel()

	// Claude: opus < sonnet < haiku (lower rank = smarter)
	assert.Less(t, ModelRank("claude", "claude-opus-4-6"), ModelRank("claude", "claude-sonnet-4-6"))
	assert.Less(t, ModelRank("claude", "claude-sonnet-4-6"), ModelRank("claude", "claude-haiku-4-5-20251001"))

	// Codex: gpt-5.4 < gpt-5 < gpt-5-codex-mini
	assert.Less(t, ModelRank("codex", "gpt-5.4"), ModelRank("codex", "gpt-5"))
	assert.Less(t, ModelRank("codex", "gpt-5"), ModelRank("codex", "gpt-5-codex-mini"))
}

func TestModelSetsNoDuplicates(t *testing.T) {
	t.Parallel()

	assert.Equal(t, len(ClaudeModels), len(claudeModelSet), "duplicate in ClaudeModels")
	assert.Equal(t, len(CodexModels), len(codexModelSet), "duplicate in CodexModels")
}
