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

func TestAssemblePrompt_WritingMode_MaxEffort(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   dir,
		SprintPrompt: "Write the introduction.",
		Promise:      "DONE",
		EffortLevel:  epic.EffortMax,
		Mode:         "writing",
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "QUALITY DIRECTIVE")
	assert.Contains(t, prompt, "editorial rigor")
	assert.Contains(t, prompt, "audience engagement")
	assert.NotContains(t, prompt, "defensive code")
}

func TestAssemblePrompt_WritingMode_PlanReference(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   dir,
		SprintPrompt: "Write chapter one.",
		Promise:      "DONE",
		Mode:         "writing",
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "content plan")
	assert.Contains(t, prompt, "chapters/sections")
	assert.NotContains(t, prompt, "project architecture")
}
