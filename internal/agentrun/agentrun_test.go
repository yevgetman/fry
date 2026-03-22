package agentrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/engine"
)

// mockEngine implements engine.Engine for testing.
type mockEngine struct {
	output string
	err    error
}

func (m *mockEngine) Run(ctx context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	select {
	case <-ctx.Done():
		return "", 1, ctx.Err()
	default:
	}
	if opts.Stdout != nil {
		fmt.Fprint(opts.Stdout, m.output)
	}
	return m.output, 0, m.err
}

func (m *mockEngine) Name() string {
	return "mock"
}

// Test 1: silent mode, normal — both log files contain the engine output.
func TestRunWithDualLogs_SilentNormal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	iterPath := filepath.Join(dir, "iter.log")
	sprintLogPath := filepath.Join(dir, "sprint.log")

	eng := &mockEngine{output: "hello output"}
	opts := DualLogOpts{
		Engine:  eng,
		Model:   "test-model",
		WorkDir: dir,
		Verbose: false,
	}

	output, err := RunWithDualLogs(context.Background(), "test prompt", iterPath, sprintLogPath, opts)
	require.NoError(t, err)
	assert.Equal(t, "hello output", output)

	iterBytes, err := os.ReadFile(iterPath)
	require.NoError(t, err)
	assert.Equal(t, "hello output", string(iterBytes))

	sprintBytes, err := os.ReadFile(sprintLogPath)
	require.NoError(t, err)
	assert.Equal(t, "hello output", string(sprintBytes))
}

// Test 2: silent mode, non-fatal error — output returned, nil error.
func TestRunWithDualLogs_SilentNonFatalError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	iterPath := filepath.Join(dir, "iter.log")
	sprintLogPath := filepath.Join(dir, "sprint.log")

	eng := &mockEngine{output: "partial output", err: errors.New("non-fatal engine error")}
	opts := DualLogOpts{
		Engine:  eng,
		WorkDir: dir,
		Verbose: false,
	}

	output, err := RunWithDualLogs(context.Background(), "test prompt", iterPath, sprintLogPath, opts)
	require.NoError(t, err)
	assert.Equal(t, "partial output", output)
}

// Test 3: silent mode, context cancelled — returns a context error.
func TestRunWithDualLogs_ContextCancelled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	iterPath := filepath.Join(dir, "iter.log")
	sprintLogPath := filepath.Join(dir, "sprint.log")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	eng := &mockEngine{output: ""}
	opts := DualLogOpts{
		Engine:  eng,
		WorkDir: dir,
		Verbose: false,
	}

	_, err := RunWithDualLogs(ctx, "test prompt", iterPath, sprintLogPath, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// Test 4: verbose mode — output returned; both log files contain content.
func TestRunWithDualLogs_VerboseNormal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	iterPath := filepath.Join(dir, "iter.log")
	sprintLogPath := filepath.Join(dir, "sprint.log")

	eng := &mockEngine{output: "verbose output"}
	opts := DualLogOpts{
		Engine:  eng,
		WorkDir: dir,
		Verbose: true,
	}

	output, err := RunWithDualLogs(context.Background(), "test prompt", iterPath, sprintLogPath, opts)
	require.NoError(t, err)
	assert.Equal(t, "verbose output", output)

	iterBytes, err := os.ReadFile(iterPath)
	require.NoError(t, err)
	assert.Equal(t, "verbose output", string(iterBytes))

	sprintBytes, err := os.ReadFile(sprintLogPath)
	require.NoError(t, err)
	assert.Equal(t, "verbose output", string(sprintBytes))
}

// Test 5: missing iterPath directory — returns a non-nil error.
func TestRunWithDualLogs_MissingIterDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	iterPath := filepath.Join(dir, "nonexistent", "iter.log") // parent directory does not exist
	sprintLogPath := filepath.Join(dir, "sprint.log")

	eng := &mockEngine{output: ""}
	opts := DualLogOpts{
		Engine:  eng,
		WorkDir: dir,
		Verbose: false,
	}

	_, err := RunWithDualLogs(context.Background(), "test prompt", iterPath, sprintLogPath, opts)
	require.Error(t, err)
}
