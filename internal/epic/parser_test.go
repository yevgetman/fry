package epic

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
)

func TestParseBasicEpic(t *testing.T) {
	t.Parallel()

	ep := parseTempEpic(t, `
@epic Basic Epic
@sprint 1
@name Setup
@max_iterations 2
@promise BASIC_DONE
@prompt
Ship the first slice.
`)

	assert.Equal(t, "Basic Epic", ep.Name)
	assert.Equal(t, config.DefaultVerificationFile, ep.VerificationFile)
	assert.Equal(t, config.DefaultMaxHealAttempts, ep.MaxHealAttempts)
	assert.Equal(t, config.DefaultDockerReadyTimeout, ep.DockerReadyTimeout)
	assert.Equal(t, config.DefaultMaxDeviationScope, ep.MaxDeviationScope)
	require.Len(t, ep.Sprints, 1)
	assert.Equal(t, 1, ep.Sprints[0].Number)
	assert.Equal(t, "Setup", ep.Sprints[0].Name)
	assert.Equal(t, 2, ep.Sprints[0].MaxIterations)
	assert.Equal(t, "BASIC_DONE", ep.Sprints[0].Promise)
	assert.Equal(t, "Ship the first slice.", ep.Sprints[0].Prompt)
	assert.Equal(t, 1, ep.TotalSprints)
}

func TestParseMultiSprintEpic(t *testing.T) {
	t.Parallel()

	ep := parseTempEpic(t, `
@epic Multi
@sprint 1
@name One
@max_iterations 1
@promise ONE
@prompt
Prompt one.
@sprint 2
@name Two
@max_iterations 2
@promise TWO
@prompt
Prompt two.
With another line.
@sprint 3
@name Three
@max_iterations 3
@promise THREE
@prompt
Prompt three.
`)

	require.Len(t, ep.Sprints, 3)
	assert.Equal(t, []int{1, 2, 3}, []int{ep.Sprints[0].Number, ep.Sprints[1].Number, ep.Sprints[2].Number})
	assert.Equal(t, "Prompt one.", ep.Sprints[0].Prompt)
	assert.Equal(t, "Prompt two.\nWith another line.", ep.Sprints[1].Prompt)
	assert.Equal(t, "Prompt three.", ep.Sprints[2].Prompt)
}

func TestParseAllGlobalDirectives(t *testing.T) {
	t.Parallel()

	ep := parseTempEpic(t, `
@epic Full Epic
@engine claude
@docker_from_sprint 2
@docker_ready_cmd docker compose ps
@docker_ready_timeout 45
@require_tool go
@require_tool git
@preflight_cmd test -f go.mod
@preflight_cmd go test ./...
@pre_sprint ./scripts/pre-sprint.sh
@pre_iteration ./scripts/pre-iteration.sh
@model sonnet
@engine_flags --json --danger
@verification custom-verification.md
@max_heal_attempts 7
@compact_with_agent
@review_between_sprints
@review_engine claude
@review_model reviewer-x
@max_deviation_scope 5
@sprint 1
@name One
@max_iterations 2
@promise OK
@prompt
Do it.
`)

	assert.Equal(t, "Full Epic", ep.Name)
	assert.Equal(t, "claude", ep.Engine)
	assert.Equal(t, 2, ep.DockerFromSprint)
	assert.Equal(t, "docker compose ps", ep.DockerReadyCmd)
	assert.Equal(t, 45, ep.DockerReadyTimeout)
	assert.Equal(t, []string{"go", "git"}, ep.RequiredTools)
	assert.Equal(t, []string{"test -f go.mod", "go test ./..."}, ep.PreflightCmds)
	assert.Equal(t, "./scripts/pre-sprint.sh", ep.PreSprintCmd)
	assert.Equal(t, "./scripts/pre-iteration.sh", ep.PreIterationCmd)
	assert.Equal(t, "sonnet", ep.AgentModel)
	assert.Equal(t, "--json --danger", ep.AgentFlags)
	assert.Equal(t, "custom-verification.md", ep.VerificationFile)
	assert.Equal(t, 7, ep.MaxHealAttempts)
	assert.True(t, ep.CompactWithAgent)
	assert.True(t, ep.ReviewBetweenSprints)
	assert.Equal(t, "claude", ep.ReviewEngine)
	assert.Equal(t, "reviewer-x", ep.ReviewModel)
	assert.Equal(t, 5, ep.MaxDeviationScope)
}

func TestParsePerSprintHealAttempts(t *testing.T) {
	t.Parallel()

	ep := parseTempEpic(t, `
@epic Heal
@max_heal_attempts 3
@sprint 1
@name One
@max_iterations 1
@promise ONE
@max_heal_attempts 9
@prompt
Prompt.
`)

	require.NotNil(t, ep.Sprints[0].MaxHealAttempts)
	assert.Equal(t, 9, *ep.Sprints[0].MaxHealAttempts)
	assert.Equal(t, 3, ep.MaxHealAttempts)
}

func TestParsePromptBleedStripping(t *testing.T) {
	t.Parallel()

	ep := parseTempEpic(t, `
@epic Bleed
@sprint 1
@name One
@max_iterations 1
@promise ONE
@prompt
Keep this.

# =====
## ====

`)

	assert.Equal(t, "Keep this.", ep.Sprints[0].Prompt)
}

func TestParseUnknownDirectiveWarning(t *testing.T) {
	t.Parallel()

	output := captureStderr(t, func() {
		parseTempEpic(t, `
@epic Warn
@mystery value
@sprint 1
@name One
@bogus nope
@max_iterations 1
@promise ONE
@prompt
Prompt.
`)
	})

	assert.Contains(t, output, "warning: unrecognized directive: @mystery value")
	assert.Contains(t, output, "warning: unrecognized directive: @bogus nope")
}

func TestParseBooleanFlags(t *testing.T) {
	t.Parallel()

	ep := parseTempEpic(t, `
@epic Flags
@compact_with_agent
@review_between_sprints
@sprint 1
@name One
@max_iterations 1
@promise ONE
@prompt
Prompt.
`)

	assert.True(t, ep.CompactWithAgent)
	assert.True(t, ep.ReviewBetweenSprints)
}

func TestParseModelAliases(t *testing.T) {
	t.Parallel()

	ep := parseTempEpic(t, `
@epic Aliases
@codex_model gpt-5
@codex_flags --profile fast
@sprint 1
@name One
@max_iterations 1
@promise ONE
@prompt
Prompt.
`)

	assert.Equal(t, "gpt-5", ep.AgentModel)
	assert.Equal(t, "--profile fast", ep.AgentFlags)
}

func TestParseEndDirective(t *testing.T) {
	t.Parallel()

	ep := parseTempEpic(t, `
@epic End
@sprint 1
@name One
@max_iterations 1
@promise ONE
@prompt
Prompt.
@end
@review_engine claude
`)

	require.Len(t, ep.Sprints, 1)
	assert.Equal(t, "claude", ep.ReviewEngine)
}

func TestValidateEpic(t *testing.T) {
	t.Parallel()

	valid := &Epic{
		Sprints: []Sprint{{
			Number:        1,
			Name:          "One",
			MaxIterations: 1,
			Promise:       "ONE",
			Prompt:        "Prompt.",
		}},
	}
	assert.NoError(t, ValidateEpic(valid))

	err := ValidateEpic(&Epic{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one sprint")

	err = ValidateEpic(&Epic{Sprints: []Sprint{
		{Number: 1, Name: "One", MaxIterations: 1, Promise: "ONE", Prompt: "Prompt."},
		{Number: 3, Name: "Three", MaxIterations: 1, Promise: "THREE", Prompt: "Prompt."},
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected sprint 2, got 3")

	err = ValidateEpic(&Epic{Sprints: []Sprint{{Number: 1, MaxIterations: 1, Promise: "ONE", Prompt: "Prompt."}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing @name")

	err = ValidateEpic(&Epic{Sprints: []Sprint{{Number: 1, Name: "One", MaxIterations: 1, Promise: "ONE"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing @prompt")
}

func TestParseEpicBadSprintNumber(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "epic.md")
	require.NoError(t, os.WriteFile(path, []byte("@epic Bad\n@sprint abc\n"), 0o600))

	_, err := ParseEpic(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an integer")
}

func TestParseEpicBadMaxIterations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "epic.md")
	content := "@epic Bad\n@sprint 1\n@name One\n@max_iterations xyz\n@promise ONE\n@prompt\nDo it.\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := ParseEpic(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an integer")
}

func TestParseEpicFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := ParseEpic("/nonexistent/path/epic.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open epic file")
}

func parseTempEpic(t *testing.T, contents string) *Epic {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "epic.md")
	err := os.WriteFile(path, []byte(strings.TrimLeft(contents, "\n")), 0o600)
	require.NoError(t, err)

	ep, err := ParseEpic(path)
	require.NoError(t, err)
	return ep
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	fn()

	require.NoError(t, w.Close())
	os.Stderr = old

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	return string(data)
}
