package engine

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEngine(t *testing.T) {
	t.Parallel()

	name, err := ResolveEngine("claude", "codex", "codex", "")
	require.NoError(t, err)
	assert.Equal(t, "claude", name)

	name, err = ResolveEngine("", "claude", "codex", "")
	require.NoError(t, err)
	assert.Equal(t, "claude", name)

	name, err = ResolveEngine("", "", "claude", "")
	require.NoError(t, err)
	assert.Equal(t, "claude", name)

	name, err = ResolveEngine("", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "claude", name)
}

func TestResolveEngineCustomDefault(t *testing.T) {
	t.Parallel()

	// Custom default is used when nothing else is specified.
	name, err := ResolveEngine("", "", "", "claude")
	require.NoError(t, err)
	assert.Equal(t, "claude", name)

	// CLI flag overrides the custom default.
	name, err = ResolveEngine("codex", "", "", "claude")
	require.NoError(t, err)
	assert.Equal(t, "codex", name)

	// Epic directive overrides the custom default.
	name, err = ResolveEngine("", "codex", "", "claude")
	require.NoError(t, err)
	assert.Equal(t, "codex", name)
}

func TestResolveEngineInvalid(t *testing.T) {
	t.Parallel()

	_, err := ResolveEngine("", "", "gpt", "")
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

	args := codexArgs(RunOpts{
		Model:      "gpt-5",
		ExtraFlags: []string{"--json", "--foo=bar"},
	})

	assert.Equal(t, []string{
		"exec",
		"--dangerously-bypass-approvals-and-sandbox",
		"--model", "gpt-5",
		"--json", "--foo=bar",
	}, args)
}

func TestClaudeCommandConstruction(t *testing.T) {
	t.Parallel()

	args := claudeArgs(RunOpts{
		Model:      "sonnet",
		ExtraFlags: []string{"--output-format", "json"},
	})

	assert.Equal(t, []string{
		"-p",
		"--dangerously-skip-permissions",
		"--model", "sonnet",
		"--output-format", "json",
	}, args)
}

func TestCodexRunBinaryNotFound(t *testing.T) {
	// t.Parallel() intentionally omitted: t.Setenv panics after t.Parallel() (Go 1.17+).
	t.Setenv("PATH", t.TempDir())
	eng := &CodexEngine{}
	_, exitCode, err := eng.Run(context.Background(), "hello", RunOpts{})
	require.Error(t, err)
	assert.Equal(t, -1, exitCode)
}

func TestClaudeRunBinaryNotFound(t *testing.T) {
	// t.Parallel() intentionally omitted: t.Setenv panics after t.Parallel() (Go 1.17+).
	t.Setenv("PATH", t.TempDir())
	eng := &ClaudeEngine{}
	_, exitCode, err := eng.Run(context.Background(), "hello", RunOpts{})
	require.Error(t, err)
	assert.Equal(t, -1, exitCode)
}

func TestExitCodeFromError_Nil(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, exitCodeFromError(nil))
}

func TestExitCodeFromError_NonExitError(t *testing.T) {
	t.Parallel()

	assert.Equal(t, -1, exitCodeFromError(fmt.Errorf("something")))
}

func TestCodexRunContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eng := &CodexEngine{}
	_, _, err := eng.Run(ctx, "hello", RunOpts{})
	require.Error(t, err)
}

func TestClaudeRunContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eng := &ClaudeEngine{}
	_, _, err := eng.Run(ctx, "hello", RunOpts{})
	require.Error(t, err)
}

func TestCombinedWriter(t *testing.T) {
	t.Parallel()

	var bufA, bufB bytes.Buffer
	w := combinedWriter(&bufA, &bufB)
	_, err := w.Write([]byte("test-data"))
	require.NoError(t, err)
	assert.Equal(t, "test-data", bufA.String())
	assert.Equal(t, "test-data", bufB.String())
}
