package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEngine(t *testing.T) {
	t.Parallel()

	name, err := ResolveEngine("claude", "codex", "codex")
	require.NoError(t, err)
	assert.Equal(t, "claude", name)

	name, err = ResolveEngine("", "claude", "codex")
	require.NoError(t, err)
	assert.Equal(t, "claude", name)

	name, err = ResolveEngine("", "", "claude")
	require.NoError(t, err)
	assert.Equal(t, "claude", name)

	name, err = ResolveEngine("", "", "")
	require.NoError(t, err)
	assert.Equal(t, "codex", name)
}

func TestResolveEngineInvalid(t *testing.T) {
	t.Parallel()

	_, err := ResolveEngine("", "", "gpt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported engine")
}

func TestNewEngine(t *testing.T) {
	t.Parallel()

	eng, err := NewEngine("codex")
	require.NoError(t, err)
	_, ok := eng.(*CodexEngine)
	assert.True(t, ok)

	eng, err = NewEngine("claude")
	require.NoError(t, err)
	_, ok = eng.(*ClaudeEngine)
	assert.True(t, ok)
}

func TestCodexCommandConstruction(t *testing.T) {
	t.Parallel()

	args := codexArgs("build it", RunOpts{
		Model:      "gpt-5",
		ExtraFlags: []string{"--json", "--foo=bar"},
	})

	assert.Equal(t, []string{
		"exec",
		"--dangerously-bypass-approvals-and-sandbox",
		"--model", "gpt-5",
		"--json", "--foo=bar",
		"build it",
	}, args)
}

func TestClaudeCommandConstruction(t *testing.T) {
	t.Parallel()

	args := claudeArgs("build it", RunOpts{
		Model:      "sonnet",
		ExtraFlags: []string{"--output-format", "json"},
	})

	assert.Equal(t, []string{
		"-p",
		"--dangerously-skip-permissions",
		"--model", "sonnet",
		"--output-format", "json",
		"build it",
	}, args)
}
