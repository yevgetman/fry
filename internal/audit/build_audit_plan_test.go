package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/review"
)

func TestBuildAuditPromptUsesAdversarialScopeAndDeferredAnalysis(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{name: "claude"}
	opts := makeBuildOpts(t, eng)
	opts.DeferredFailures = `# Deferred Sanity Check Failures

## Sprint 2: Pricing

- DEFERRED: Command failed: scripts/check-pricing.sh

## Sprint 4: Pricing Review

- DEFERRED: Command output mismatch: scripts/check-pricing.sh (expected pattern: PASS)
`
	require.NoError(t, review.AppendDeviationLog(opts.ProjectDir, review.DeviationLogEntry{
		SprintNum:       2,
		SprintName:      "Pricing",
		Verdict:         review.VerdictDeviate,
		Trigger:         "The appendix remains authoritative over the narrative summary.",
		Impact:          "- Preserve appendix values.\n",
		RiskAssessment:  "Low risk with reconciliation notes.",
		AffectedSprints: []int{2},
	}))

	prompt := buildBuildAuditPrompt(opts)
	assert.Contains(t, prompt, "reviewing this document corpus for the first time")
	assert.Contains(t, prompt, "## Build Scope")
	assert.NotContains(t, prompt, "| Sprint | Name | Status | Duration |")
	assert.Contains(t, prompt, "## Cross-Document Integrity")
	assert.Contains(t, prompt, "## Deferred Failure Analysis")
	assert.Contains(t, prompt, "## Intentional Divergences Log")
	assert.Contains(t, prompt, "re-read each document from the beginning")
}
