package review

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/epic"
)

func TestParseVerdictContinue(t *testing.T) {
	t.Parallel()
	assert.Equal(t, VerdictContinue, ParseVerdict("<verdict>CONTINUE</verdict>"))
}

func TestParseVerdictDeviate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, VerdictDeviate, ParseVerdict("<verdict>DEVIATE</verdict>"))
}

func TestParseVerdictDefault(t *testing.T) {
	t.Parallel()
	assert.Equal(t, VerdictContinue, ParseVerdict("no verdict"))
}

func TestExtractDeviationSpec(t *testing.T) {
	t.Parallel()

	output := `### Decision
<verdict>DEVIATE</verdict>

### Deviation Spec
- **Trigger**: Auth middleware built at internal/middleware/auth instead of pkg/auth
- **Affected sprints**: 4, 5
- **Sprint 4**: Update 3 import path references from pkg/auth to internal/middleware/auth
- **Sprint 5**: Update 1 wiring reference
- **Risk assessment**: Low — purely mechanical path changes
`

	spec := ExtractDeviationSpec(output)
	require.NotNil(t, spec)
	assert.Equal(t, "Auth middleware built at internal/middleware/auth instead of pkg/auth", spec.Trigger)
	assert.Equal(t, []int{4, 5}, spec.AffectedSprints)
	assert.Contains(t, spec.Details, "Sprint 4")
	assert.Equal(t, "Low — purely mechanical path changes", spec.RiskAssessment)
}

func TestSimulationOutput(t *testing.T) {
	t.Parallel()

	continueOutput, err := simulatedReviewOutput("CONTINUE", 2, 5)
	require.NoError(t, err)
	assert.Equal(t, "### Analysis\nSprint completed as planned.\n\n### Decision\n<verdict>CONTINUE</verdict>\n", continueOutput)

	deviateOutput, err := simulatedReviewOutput("DEVIATE", 2, 5)
	require.NoError(t, err)
	assert.Contains(t, deviateOutput, "<verdict>DEVIATE</verdict>")
	assert.Contains(t, deviateOutput, "- **Affected sprints**: 3")
}

func TestValidateReplan(t *testing.T) {
	t.Parallel()

	original := mustParseEpic(t, `
@epic Demo
@review_between_sprints
@review_engine claude
@sprint 1
@name One
@max_iterations 3
@promise ONE
@prompt
Keep one.
@sprint 2
@name Two
@max_iterations 3
@promise TWO
@prompt
Keep two.
`)

	updated := mustParseEpic(t, `
@epic Demo
@review_between_sprints
@review_engine claude
@sprint 1
@name One
@max_iterations 3
@promise ONE
@prompt
Keep one.
@sprint 2
@name Two
@max_iterations 3
@promise TWO
@prompt
Update two.
`)

	require.NoError(t, ValidateReplan(original, updated, 1, 2))

	updatedBad := mustParseEpic(t, `
@epic Demo
@review_between_sprints
@review_engine claude
@sprint 1
@name One
@max_iterations 3
@promise ONE
@prompt
Changed one.
@sprint 2
@name Two
@max_iterations 3
@promise TWO
@prompt
Update two.
`)

	err := ValidateReplan(original, updatedBad, 1, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completed sprint 1")
}

func TestValidateReplanScopeCap(t *testing.T) {
	t.Parallel()

	original := mustParseEpic(t, baseEpicForValidation("Third"))
	updated := mustParseEpic(t, strings.Replace(baseEpicForValidation("Third"), "Third prompt.", "Third changed.", 1))

	err := ValidateReplan(original, updated, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside deviation scope")
}

func TestValidateReplanStructuralPreservation(t *testing.T) {
	t.Parallel()

	original := mustParseEpic(t, baseEpicForValidation("Third"))
	updated := mustParseEpic(t, strings.Replace(baseEpicForValidation("Third"), "@name Two", "@name Two Updated", 1))

	err := ValidateReplan(original, updated, 1, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "structural directives")
}

func mustParseEpic(t *testing.T, content string) *epic.Epic {
	t.Helper()

	file := t.TempDir() + "/epic.md"
	require.NoError(t, osWriteFile(file, []byte(strings.TrimSpace(content)+"\n"), 0o644))
	ep, err := epic.ParseEpic(file)
	require.NoError(t, err)
	return ep
}

func baseEpicForValidation(thirdPrompt string) string {
	return `
@epic Demo
@review_between_sprints
@review_engine claude
@sprint 1
@name One
@max_iterations 3
@promise ONE
@prompt
First prompt.
@sprint 2
@name Two
@max_iterations 3
@promise TWO
@prompt
Second prompt.
@sprint 3
@name Three
@max_iterations 3
@promise THREE
@prompt
` + thirdPrompt + ` prompt.
`
}

func osWriteFile(name string, data []byte, perm uint32) error {
	return os.WriteFile(name, data, os.FileMode(perm))
}
