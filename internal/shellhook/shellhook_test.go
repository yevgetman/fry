package shellhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_EmptyCommand(t *testing.T) {
	t.Parallel()

	require.NoError(t, Run(context.Background(), t.TempDir(), ""))
	require.NoError(t, Run(context.Background(), t.TempDir(), "   "))
	require.NoError(t, Run(context.Background(), t.TempDir(), "\t\n"))
}

func TestRun_SuccessfulCommand(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), t.TempDir(), "echo hello")
	require.NoError(t, err)
}

func TestRun_FailingCommand(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), t.TempDir(), "exit 1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `run "exit 1"`)
}

func TestRun_CommandWithOutput(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), t.TempDir(), "echo 'error msg' >&2 && exit 1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error msg")
}

func TestRun_UsesProjectDir(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	err := Run(context.Background(), projectDir, "test \"$(pwd)\" = \""+projectDir+"\"")
	require.NoError(t, err)
}

func TestRun_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, t.TempDir(), "sleep 10")
	require.Error(t, err)
}
