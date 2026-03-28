package steering

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

// --- Directive tests ---

func TestReadDirective_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content, err := ReadDirective(dir)
	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestReadDirective_WithFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.AgentDirectiveFile), []byte("focus on API"), 0o644))

	content, err := ReadDirective(dir)
	require.NoError(t, err)
	assert.Equal(t, "focus on API", content)
}

func TestClearDirective_RemovesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.AgentDirectiveFile), []byte("test"), 0o644))

	require.NoError(t, ClearDirective(dir))

	_, err := os.Stat(filepath.Join(dir, config.AgentDirectiveFile))
	assert.True(t, os.IsNotExist(err))
}

func TestClearDirective_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, ClearDirective(dir))
}

// --- Hold tests ---

func TestIsHoldRequested_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assert.False(t, IsHoldRequested(dir))
}

func TestIsHoldRequested_WithFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.AgentHoldFile), nil, 0o644))

	assert.True(t, IsHoldRequested(dir))
}

func TestClearHold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.AgentHoldFile), nil, 0o644))

	require.NoError(t, ClearHold(dir))
	assert.False(t, IsHoldRequested(dir))
}

// --- Decision tests ---

func TestWriteDecisionNeeded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	require.NoError(t, WriteDecisionNeeded(dir, "What should I do?"))

	data, err := os.ReadFile(filepath.Join(dir, config.DecisionNeededFile))
	require.NoError(t, err)
	assert.Equal(t, "What should I do?", string(data))
}

func TestClearDecisionNeeded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.DecisionNeededFile), []byte("test"), 0o644))

	require.NoError(t, ClearDecisionNeeded(dir))
	_, err := os.Stat(filepath.Join(dir, config.DecisionNeededFile))
	assert.True(t, os.IsNotExist(err))
}

func TestWaitForDecision_FileAppears(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	// Write the directive after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(dir, config.AgentDirectiveFile), []byte("continue"), 0o644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	decision, err := WaitForDecision(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, "continue", decision)

	// Directive should be cleared
	content, err := ReadDirective(dir)
	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestWaitForDecision_ContextCanceled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := WaitForDecision(ctx, dir)
	assert.Error(t, err)
}

// --- Pause tests ---

func TestIsPaused_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assert.False(t, IsPaused(dir))
}

func TestIsPaused_WithFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.AgentPauseFile), nil, 0o644))

	assert.True(t, IsPaused(dir))
}

func TestClearPause(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.AgentPauseFile), nil, 0o644))

	require.NoError(t, ClearPause(dir))
	assert.False(t, IsPaused(dir))
}
