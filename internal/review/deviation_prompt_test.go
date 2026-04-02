package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRelevantDeviationsAndGuidance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, AppendDeviationLog(dir, DeviationLogEntry{
		SprintNum:       1,
		SprintName:      "Forecast",
		Verdict:         VerdictDeviate,
		Trigger:         "Pricing document remains authoritative over summary copy.",
		Impact:          "- Keep the pricing appendix values as-is.\n",
		RiskAssessment:  "Low risk if the summary adds a reconciliation note.",
		AffectedSprints: []int{3},
	}))

	relevant := LoadRelevantDeviations(dir, 3, 10_000)
	guidance := LoadActiveDeviationGuidance(dir, 3, 10_000)

	assert.Contains(t, relevant, "Review after Sprint 1")
	assert.Contains(t, relevant, "Affected sprints")
	assert.Contains(t, guidance, "Pricing document remains authoritative")
	assert.Contains(t, guidance, "reconciliation note")
}

func TestLoadRelevantDeviationsMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assert.Empty(t, LoadRelevantDeviations(dir, 2, 1_000))
	assert.Empty(t, LoadActiveDeviationGuidance(dir, 2, 1_000))
}
