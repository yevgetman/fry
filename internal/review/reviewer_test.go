package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
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

func TestExtractSprintPrompt(t *testing.T) {
	t.Parallel()

	epicContent := `@epic Demo
@sprint 1
@name One
@max_iterations 1
@promise ONE
@prompt
First prompt line.
Second prompt line.
@sprint 2
@name Two
@max_iterations 1
@promise TWO
@prompt
Second sprint prompt.
@end
@sprint 3
@name Three
@max_iterations 1
@promise THREE
@prompt
Third sprint.`

	tests := []struct {
		name      string
		sprintNum int
		want      string
	}{
		{"first sprint multi-line", 1, "First prompt line.\nSecond prompt line."},
		{"second sprint with end directive", 2, "Second sprint prompt."},
		{"third sprint at end of file", 3, "Third sprint."},
		{"missing sprint", 99, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ExtractSprintPrompt(epicContent, tt.sprintNum))
		})
	}
}

func TestAssembleReviewPrompt(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	prompt, err := AssembleReviewPrompt(ReviewPromptOpts{
		ProjectDir:             projectDir,
		SprintNum:              3,
		TotalSprints:           5,
		SprintName:             "Build API",
		RemainingSprintPrompts: []string{"### Sprint 4: Tests\n\nWrite tests.", "### Sprint 5: Deploy\n\nDeploy."},
		EpicProgressContent:    "Sprint 1 done. Sprint 2 done.",
		SprintProgressContent:  "Iteration 1: built endpoints.",
		DeviationLogContent:    "",
	})
	require.NoError(t, err)

	assert.Contains(t, prompt, "# Sprint Review — After Sprint 3: Build API")
	assert.Contains(t, prompt, "Sprint 4 through 5")
	assert.Contains(t, prompt, "## Bias: CONTINUE")
	assert.Contains(t, prompt, "Sprint 1 done. Sprint 2 done.")
	assert.Contains(t, prompt, "Iteration 1: built endpoints.")
	assert.Contains(t, prompt, "### Sprint 4: Tests")
	assert.Contains(t, prompt, "### Sprint 5: Deploy")
	assert.Contains(t, prompt, "None — this is the first review.")
	assert.Contains(t, prompt, "<verdict>CONTINUE</verdict> or <verdict>DEVIATE</verdict>")

	_, err = os.Stat(filepath.Join(projectDir, config.ReviewPromptFile))
	assert.NoError(t, err)
}

func TestRunReplanEndToEnd(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	epicContent := "@epic Demo\n@review_between_sprints\n@review_engine claude\n" +
		"@sprint 1\n@name One\n@max_iterations 3\n@promise ONE\n@prompt\nFirst prompt.\n" +
		"@sprint 2\n@name Two\n@max_iterations 3\n@promise TWO\n@prompt\nSecond prompt.\n"

	epicPath := filepath.Join(projectDir, config.FryDir, "epic.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(epicPath), 0o755))
	require.NoError(t, os.WriteFile(epicPath, []byte(epicContent), 0o644))

	planPath := filepath.Join(projectDir, config.PlanFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(planPath), 0o755))
	require.NoError(t, os.WriteFile(planPath, []byte("Plan content\n"), 0o644))

	modifiedEpic := strings.Replace(epicContent, "Second prompt.", "Second prompt updated.", 1)
	mockEngine := &stubReplanEngine{output: modifiedEpic}

	err := RunReplan(context.Background(), ReplanOpts{
		ProjectDir: projectDir,
		EpicPath:   epicPath,
		DeviationSpec: &DeviationSpec{
			Trigger:         "Test trigger",
			AffectedSprints: []int{2},
			RiskAssessment:  "Low",
			RawText:         "Test deviation",
		},
		CompletedSprint: 1,
		MaxScope:        2,
		Engine:          mockEngine,
	})
	require.NoError(t, err)

	updated, err := os.ReadFile(epicPath)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "Second prompt updated.")
	assert.Contains(t, string(updated), "First prompt.")

	backups, err := filepath.Glob(filepath.Join(projectDir, config.BuildLogsDir, "epic.md.bak.*"))
	require.NoError(t, err)
	assert.Len(t, backups, 1)
}

type stubReplanEngine struct {
	output string
}

func (s *stubReplanEngine) Run(_ context.Context, _ string, opts engine.RunOpts) (string, int, error) {
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(s.output))
	}
	return s.output, 0, nil
}

func (s *stubReplanEngine) Name() string {
	return "stub"
}

func TestAssembleReviewPromptDefaults(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	prompt, err := AssembleReviewPrompt(ReviewPromptOpts{
		ProjectDir:   projectDir,
		SprintNum:    1,
		TotalSprints: 1,
		SprintName:   "Solo",
	})
	require.NoError(t, err)

	assert.Contains(t, prompt, "(No epic progress recorded yet.)")
	assert.Contains(t, prompt, "(No sprint progress recorded.)")
	assert.Contains(t, prompt, "(No remaining sprints.)")
}
