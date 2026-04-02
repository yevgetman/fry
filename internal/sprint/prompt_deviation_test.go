package sprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/review"
)

func TestAssemblePromptIncludesActiveDeviationGuidance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, review.AppendDeviationLog(dir, review.DeviationLogEntry{
		SprintNum:       1,
		SprintName:      "Pricing",
		Verdict:         review.VerdictDeviate,
		Trigger:         "The appendix remains authoritative over the summary.",
		Impact:          "- Preserve appendix numbers.\n",
		RiskAssessment:  "Low risk with a reconciliation note.",
		AffectedSprints: []int{3},
	}))

	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   dir,
		SprintNumber: 3,
		SprintPrompt: "Write the executive summary.",
		Promise:      "DONE",
	})
	require.NoError(t, err)

	assert.Contains(t, prompt, "ACTIVE INTENTIONAL DIVERGENCES")
	assert.Contains(t, prompt, "appendix remains authoritative")
	assert.Contains(t, prompt, "reconciliation note")
}
