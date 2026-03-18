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
	require.NoError(t, ValidateModel("codex", "gpt-5.4-mini"))
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

	// Codex: gpt-5.4 is smartest, gpt-5.4-mini is mid-range, legacy mini is cheapest
	assert.Equal(t, 1, ModelRank("codex", "gpt-5.4"))
	assert.Equal(t, 8, ModelRank("codex", "gpt-5.4-mini"))
	assert.Equal(t, 12, ModelRank("codex", "gpt-5-codex-mini"))

	// Unknown model returns 0
	assert.Equal(t, 0, ModelRank("claude", "nonexistent"))
	assert.Equal(t, 0, ModelRank("unknown-engine", "gpt-5.4"))
}

func TestModelRankOrdering(t *testing.T) {
	t.Parallel()

	// Claude: opus < sonnet < haiku (lower rank = smarter)
	assert.Less(t, ModelRank("claude", "claude-opus-4-6"), ModelRank("claude", "claude-sonnet-4-6"))
	assert.Less(t, ModelRank("claude", "claude-sonnet-4-6"), ModelRank("claude", "claude-haiku-4-5-20251001"))

	// Codex: gpt-5.4 < gpt-5.4-mini < gpt-5 < gpt-5-codex-mini
	assert.Less(t, ModelRank("codex", "gpt-5.4"), ModelRank("codex", "gpt-5.4-mini"))
	assert.Less(t, ModelRank("codex", "gpt-5.4-mini"), ModelRank("codex", "gpt-5"))
	assert.Less(t, ModelRank("codex", "gpt-5"), ModelRank("codex", "gpt-5-codex-mini"))
}

func TestModelSetsNoDuplicates(t *testing.T) {
	t.Parallel()

	assert.Equal(t, len(ClaudeModels), len(claudeModelSet), "duplicate in ClaudeModels")
	assert.Equal(t, len(CodexModels), len(codexModelSet), "duplicate in CodexModels")
}

// --- Tier system tests ---

func TestTierModel(t *testing.T) {
	t.Parallel()

	// Claude tier mappings
	assert.Equal(t, "opus[1m]", TierModel("claude", TierFrontier))
	assert.Equal(t, "sonnet", TierModel("claude", TierStandard))
	assert.Equal(t, "haiku", TierModel("claude", TierMini))
	assert.Equal(t, "haiku", TierModel("claude", TierLabor))

	// Codex tier mappings
	assert.Equal(t, "gpt-5.4", TierModel("codex", TierFrontier))
	assert.Equal(t, "gpt-5.3-codex", TierModel("codex", TierStandard))
	assert.Equal(t, "gpt-5.4-mini", TierModel("codex", TierMini))
	assert.Equal(t, "gpt-5-codex-mini", TierModel("codex", TierLabor))

	// Unknown engine returns empty
	assert.Equal(t, "", TierModel("gemini", TierFrontier))
}

func TestTierMappingsAreValidModels(t *testing.T) {
	t.Parallel()

	for tier, model := range claudeTierModels {
		require.NoError(t, ValidateModel("claude", model), "claude tier %s maps to invalid model %q", tier, model)
	}
	for tier, model := range codexTierModels {
		require.NoError(t, ValidateModel("codex", model), "codex tier %s maps to invalid model %q", tier, model)
	}
}

func TestTierForSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		engine  string
		effort  string
		session SessionType
		want    ModelTier
	}{
		// Sprint: Standard for low/medium, Frontier for high/max
		{"sprint-claude-low", "claude", "low", SessionSprint, TierStandard},
		{"sprint-claude-medium", "claude", "medium", SessionSprint, TierStandard},
		{"sprint-claude-high", "claude", "high", SessionSprint, TierFrontier},
		{"sprint-claude-max", "claude", "max", SessionSprint, TierFrontier},
		{"sprint-codex-low", "codex", "low", SessionSprint, TierStandard},
		{"sprint-codex-high", "codex", "high", SessionSprint, TierFrontier},

		// Heal: same as sprint
		{"heal-claude-low", "claude", "low", SessionHeal, TierStandard},
		{"heal-claude-max", "claude", "max", SessionHeal, TierFrontier},

		// AuditFix: Standard for low/medium, Frontier for high/max
		{"auditfix-claude-low", "claude", "low", SessionAuditFix, TierStandard},
		{"auditfix-claude-medium", "claude", "medium", SessionAuditFix, TierStandard},
		{"auditfix-claude-high", "claude", "high", SessionAuditFix, TierFrontier},
		{"auditfix-claude-max", "claude", "max", SessionAuditFix, TierFrontier},
		{"auditfix-codex-low", "codex", "low", SessionAuditFix, TierStandard},
		{"auditfix-codex-high", "codex", "high", SessionAuditFix, TierFrontier},

		// Audit (Claude): Standard for low/medium/high, Frontier for max
		{"audit-claude-low", "claude", "low", SessionAudit, TierStandard},
		{"audit-claude-medium", "claude", "medium", SessionAudit, TierStandard},
		{"audit-claude-high", "claude", "high", SessionAudit, TierStandard},
		{"audit-claude-max", "claude", "max", SessionAudit, TierFrontier},

		// Audit (Codex): Mini for low, Standard for medium, Frontier for high/max
		{"audit-codex-low", "codex", "low", SessionAudit, TierMini},
		{"audit-codex-medium", "codex", "medium", SessionAudit, TierStandard},
		{"audit-codex-high", "codex", "high", SessionAudit, TierFrontier},
		{"audit-codex-max", "codex", "max", SessionAudit, TierFrontier},

		// AuditVerify: same as Audit
		{"auditverify-claude-high", "claude", "high", SessionAuditVerify, TierStandard},
		{"auditverify-claude-max", "claude", "max", SessionAuditVerify, TierFrontier},
		{"auditverify-codex-low", "codex", "low", SessionAuditVerify, TierMini},
		{"auditverify-codex-high", "codex", "high", SessionAuditVerify, TierFrontier},

		// BuildAudit: same as Audit
		{"buildaudit-claude-high", "claude", "high", SessionBuildAudit, TierStandard},
		{"buildaudit-claude-max", "claude", "max", SessionBuildAudit, TierFrontier},
		{"buildaudit-codex-low", "codex", "low", SessionBuildAudit, TierMini},
		{"buildaudit-codex-max", "codex", "max", SessionBuildAudit, TierFrontier},

		// Review: Standard for low/medium, Frontier for high/max
		{"review-claude-low", "claude", "low", SessionReview, TierStandard},
		{"review-claude-high", "claude", "high", SessionReview, TierFrontier},
		{"review-codex-medium", "codex", "medium", SessionReview, TierStandard},
		{"review-codex-max", "codex", "max", SessionReview, TierFrontier},

		// Replan: same as Review
		{"replan-claude-low", "claude", "low", SessionReplan, TierStandard},
		{"replan-claude-max", "claude", "max", SessionReplan, TierFrontier},

		// BuildSummary: Mini for low/medium, Standard for high/max
		{"summary-claude-low", "claude", "low", SessionBuildSummary, TierMini},
		{"summary-claude-medium", "claude", "medium", SessionBuildSummary, TierMini},
		{"summary-claude-high", "claude", "high", SessionBuildSummary, TierStandard},
		{"summary-claude-max", "claude", "max", SessionBuildSummary, TierStandard},

		// Compaction: Labor for low/medium/high, Mini for max
		{"compact-claude-low", "claude", "low", SessionCompaction, TierLabor},
		{"compact-claude-high", "claude", "high", SessionCompaction, TierLabor},
		{"compact-claude-max", "claude", "max", SessionCompaction, TierMini},

		// Continue: Mini for low/medium, Standard for high/max
		{"continue-claude-low", "claude", "low", SessionContinue, TierMini},
		{"continue-claude-medium", "claude", "medium", SessionContinue, TierMini},
		{"continue-claude-high", "claude", "high", SessionContinue, TierStandard},
		{"continue-claude-max", "claude", "max", SessionContinue, TierStandard},

		// SanityCheck: Labor always
		{"sanity-claude-low", "claude", "low", SessionSanityCheck, TierLabor},
		{"sanity-claude-max", "claude", "max", SessionSanityCheck, TierLabor},
		{"sanity-codex-high", "codex", "high", SessionSanityCheck, TierLabor},

		// Prepare: Standard always
		{"prepare-claude-low", "claude", "low", SessionPrepare, TierStandard},
		{"prepare-claude-max", "claude", "max", SessionPrepare, TierStandard},
		{"prepare-codex-high", "codex", "high", SessionPrepare, TierStandard},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TierForSession(tc.engine, tc.effort, tc.session)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTierForSessionEmptyEffort(t *testing.T) {
	t.Parallel()

	// Empty effort normalizes to "medium"
	assert.Equal(t, TierForSession("claude", "medium", SessionSprint), TierForSession("claude", "", SessionSprint))
	assert.Equal(t, TierForSession("codex", "medium", SessionAudit), TierForSession("codex", "", SessionAudit))
}

func TestResolveModelForSession(t *testing.T) {
	t.Parallel()

	// Claude sprint at high effort → frontier → opus[1m]
	assert.Equal(t, "opus[1m]", ResolveModelForSession("claude", "high", SessionSprint))
	// Claude sprint at low effort → standard → sonnet
	assert.Equal(t, "sonnet", ResolveModelForSession("claude", "low", SessionSprint))
	// Codex audit at low effort → mini → gpt-5.4-mini
	assert.Equal(t, "gpt-5.4-mini", ResolveModelForSession("codex", "low", SessionAudit))
	// Codex sprint at max effort → frontier → gpt-5.4
	assert.Equal(t, "gpt-5.4", ResolveModelForSession("codex", "max", SessionSprint))
	// Claude sanity check → labor → haiku
	assert.Equal(t, "haiku", ResolveModelForSession("claude", "max", SessionSanityCheck))
	// Codex compaction at max → mini → gpt-5.4-mini
	assert.Equal(t, "gpt-5.4-mini", ResolveModelForSession("codex", "max", SessionCompaction))
}

func TestResolveModel(t *testing.T) {
	t.Parallel()

	// Override takes precedence
	assert.Equal(t, "my-custom-model", ResolveModel("my-custom-model", "claude", "high", SessionSprint))

	// Empty override falls through to tier system
	assert.Equal(t, "opus[1m]", ResolveModel("", "claude", "high", SessionSprint))
	assert.Equal(t, "sonnet", ResolveModel("", "claude", "low", SessionSprint))
}
