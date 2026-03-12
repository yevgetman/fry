package sprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/epic"
)

func TestAssemblePrompt_MaxEffort(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   dir,
		SprintPrompt: "Build the thing.",
		Promise:      "DONE",
		EffortLevel:  epic.EffortMax,
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "QUALITY DIRECTIVE")
	assert.Contains(t, prompt, "MAX effort")
	assert.Contains(t, prompt, "heightened rigor")
}

func TestAssemblePrompt_LowEffort(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   dir,
		SprintPrompt: "Build the thing.",
		Promise:      "DONE",
		EffortLevel:  epic.EffortLow,
	})
	require.NoError(t, err)
	assert.NotContains(t, prompt, "QUALITY DIRECTIVE")
}

func TestAssemblePrompt_NoEffort(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   dir,
		SprintPrompt: "Build the thing.",
		Promise:      "DONE",
		EffortLevel:  "",
	})
	require.NoError(t, err)
	assert.NotContains(t, prompt, "QUALITY DIRECTIVE")
}

func TestAssemblePrompt_HighEffort(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   dir,
		SprintPrompt: "Build the thing.",
		Promise:      "DONE",
		EffortLevel:  epic.EffortHigh,
	})
	require.NoError(t, err)
	assert.NotContains(t, prompt, "QUALITY DIRECTIVE")
}
