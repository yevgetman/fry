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

func TestWaitForDecision_ExitRequested(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))

	go func() {
		time.Sleep(500 * time.Millisecond)
		_, _ = RequestExit(dir)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := WaitForDecision(ctx, dir)
	require.Error(t, err)

	var exitReq *ExitRequestError
	require.ErrorAs(t, err, &exitReq)
	assert.Equal(t, "sprint_boundary", exitReq.Phase)
	assert.Contains(t, exitReq.Detail, "steering decision")
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

func TestRequestExitAndReadExitRequest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	created, err := RequestExit(dir)
	require.NoError(t, err)
	assert.True(t, created)

	req, err := ReadExitRequest(dir)
	require.NoError(t, err)
	require.NotNil(t, req)
	assert.Equal(t, 1, req.Version)
	assert.False(t, req.RequestedAt.IsZero())
}

func TestRequestExitAlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	created, err := RequestExit(dir)
	require.NoError(t, err)
	assert.True(t, created)

	created, err = RequestExit(dir)
	require.NoError(t, err)
	assert.False(t, created)
}

func TestReadStopRequestPrefersExitRequest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.AgentPauseFile), nil, 0o644))
	created, err := RequestExit(dir)
	require.NoError(t, err)
	require.True(t, created)

	req, err := ReadStopRequest(dir)
	require.NoError(t, err)
	require.NotNil(t, req)
	assert.Equal(t, "exit", req.Source)
	assert.False(t, req.RequestedAt.IsZero())
}

func TestWriteResumePointRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	point := ResumePoint{
		Phase:              "sprint_audit",
		Verdict:            ResumeVerdictResume,
		Reason:             "after audit cycle 2",
		Sprint:             6,
		SprintName:         "Polish auth",
		RecommendedCommand: "fry run --resume --sprint 6",
	}

	require.NoError(t, WriteResumePoint(dir, point))

	read, err := ReadResumePoint(dir)
	require.NoError(t, err)
	require.NotNil(t, read)
	assert.Equal(t, point.Phase, read.Phase)
	assert.Equal(t, point.Verdict, read.Verdict)
	assert.Equal(t, point.Sprint, read.Sprint)
	assert.Equal(t, point.SprintName, read.SprintName)
	assert.Equal(t, point.RecommendedCommand, read.RecommendedCommand)
	assert.False(t, read.SettledAt.IsZero())
}

func TestCleanupAllRemovesExitArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, WriteResumePoint(dir, ResumePoint{
		Phase:   "sprint_boundary",
		Verdict: ResumeVerdictContinueNext,
		Reason:  "after sprint compaction",
	}))
	created, err := RequestExit(dir)
	require.NoError(t, err)
	require.True(t, created)

	CleanupAll(dir)

	req, err := ReadExitRequest(dir)
	require.NoError(t, err)
	assert.Nil(t, req)
	point, err := ReadResumePoint(dir)
	require.NoError(t, err)
	assert.Nil(t, point)
}
