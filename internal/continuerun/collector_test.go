package continuerun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/epic"
)

func TestParseCompletedSprints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected []CompletedSprint
	}{
		{
			name:     "empty file",
			content:  "",
			expected: nil,
		},
		{
			name:     "header only",
			content:  "# Epic Progress — TestEpic\n\n",
			expected: nil,
		},
		{
			name: "one completed sprint",
			content: "# Epic Progress — TestEpic\n\n" +
				"## Sprint 1: Setup — PASS\n\nDid the setup.\n",
			expected: []CompletedSprint{
				{Number: 1, Name: "Setup", Status: "PASS"},
			},
		},
		{
			name: "multiple completed sprints",
			content: "# Epic Progress — TestEpic\n\n" +
				"## Sprint 1: Setup — PASS\n\nDone.\n\n" +
				"## Sprint 2: Auth — PASS (healed)\n\nFixed.\n\n" +
				"## Sprint 3: API — PASS (deferred failures)\n\nMostly done.\n",
			expected: []CompletedSprint{
				{Number: 1, Name: "Setup", Status: "PASS"},
				{Number: 2, Name: "Auth", Status: "PASS (healed)"},
				{Number: 3, Name: "API", Status: "PASS (deferred failures)"},
			},
		},
		{
			name: "ignores non-PASS lines",
			content: "## Sprint 1: Setup — PASS\n\n" +
				"## Sprint 2: Auth — FAIL\n\n",
			expected: []CompletedSprint{
				{Number: 1, Name: "Setup", Status: "PASS"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			fryDir := filepath.Join(dir, config.FryDir)
			require.NoError(t, os.MkdirAll(fryDir, 0o755))

			if tt.content != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, config.EpicProgressFile), []byte(tt.content), 0o644))
			}

			result := parseCompletedSprints(dir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindNextSprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		completed    []CompletedSprint
		totalSprints int
		expected     int
	}{
		{
			name:         "no completions",
			completed:    nil,
			totalSprints: 5,
			expected:     1,
		},
		{
			name: "first three done",
			completed: []CompletedSprint{
				{Number: 1}, {Number: 2}, {Number: 3},
			},
			totalSprints: 5,
			expected:     4,
		},
		{
			name: "gap at sprint 2",
			completed: []CompletedSprint{
				{Number: 1}, {Number: 3},
			},
			totalSprints: 5,
			expected:     2,
		},
		{
			name: "all complete",
			completed: []CompletedSprint{
				{Number: 1}, {Number: 2}, {Number: 3},
			},
			totalSprints: 3,
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, findNextSprint(tt.completed, tt.totalSprints))
		})
	}
}

func TestCollectActiveSprintState(t *testing.T) {
	t.Parallel()

	ep := &epic.Epic{
		TotalSprints: 3,
		Sprints: []epic.Sprint{
			{Number: 1, Name: "Setup"},
			{Number: 2, Name: "Auth"},
			{Number: 3, Name: "API"},
		},
	}

	t.Run("no evidence", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, config.BuildLogsDir), 0o755))

		result := collectActiveSprintState(dir, 2, ep)
		assert.Nil(t, result)
	})

	t.Run("has build logs", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		logsDir := filepath.Join(dir, config.BuildLogsDir)
		require.NoError(t, os.MkdirAll(logsDir, 0o755))

		// Create iteration logs
		require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint2_iter1_20260316_100000.log"), []byte("log1"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint2_iter2_20260316_100100.log"), []byte("log2\nfinal line"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint2_audit1_20260316_100200.log"), []byte("audit"), 0o644))

		result := collectActiveSprintState(dir, 2, ep)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Number)
		assert.Equal(t, "Auth", result.Name)
		assert.Equal(t, 2, result.IterationCount)
		assert.Equal(t, 1, result.AuditCount)
		assert.Equal(t, 0, result.HealCount)
		assert.False(t, result.HasRetryLog)
	})

	t.Run("has sprint progress mention", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, config.BuildLogsDir), 0o755))

		progress := "# Sprint 2: Auth — Progress\n\nDid some work.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte(progress), 0o644))

		result := collectActiveSprintState(dir, 2, ep)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Number)
		assert.Contains(t, result.ProgressExcerpt, "Sprint 2")
	})

	t.Run("has retry log", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		logsDir := filepath.Join(dir, config.BuildLogsDir)
		require.NoError(t, os.MkdirAll(logsDir, 0o755))

		require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint2_retry_20260316_100000.log"), []byte("retry"), 0o644))

		result := collectActiveSprintState(dir, 2, ep)
		require.NotNil(t, result)
		assert.True(t, result.HasRetryLog)
	})
}

func TestCollectBuildState_NoPreviousBuild(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ep := &epic.Epic{TotalSprints: 3}
	_, err := CollectBuildState(context.Background(), dir, ep)
	assert.ErrorIs(t, err, ErrNoPreviousBuild)
}

func TestCollectBuildState_FreshBuild(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	ep := &epic.Epic{
		Name:         "TestEpic",
		TotalSprints: 3,
		Engine:       "codex",
		EffortLevel:  epic.EffortHigh,
		Sprints: []epic.Sprint{
			{Number: 1, Name: "Setup"},
			{Number: 2, Name: "Auth"},
			{Number: 3, Name: "API"},
		},
	}

	state, err := CollectBuildState(context.Background(), dir, ep)
	require.NoError(t, err)
	assert.Equal(t, "TestEpic", state.EpicName)
	assert.Equal(t, 3, state.TotalSprints)
	assert.Empty(t, state.CompletedSprints)
	assert.Equal(t, 0, state.HighestCompleted)
	assert.Nil(t, state.ActiveSprint)
}

func TestCollectBuildState_PartialBuild(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fryDir := filepath.Join(dir, config.FryDir)
	logsDir := filepath.Join(dir, config.BuildLogsDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Write epic progress with sprint 1 done
	epicProgress := "# Epic Progress — TestEpic\n\n## Sprint 1: Setup — PASS\n\nCompleted setup.\n\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.EpicProgressFile), []byte(epicProgress), 0o644))

	// Write sprint 2 iteration logs (partial work)
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint2_iter1_20260316_100000.log"), []byte("working..."), 0o644))

	ep := &epic.Epic{
		Name:         "TestEpic",
		TotalSprints: 3,
		Engine:       "codex",
		EffortLevel:  epic.EffortHigh,
		Sprints: []epic.Sprint{
			{Number: 1, Name: "Setup"},
			{Number: 2, Name: "Auth"},
			{Number: 3, Name: "API"},
		},
	}

	state, err := CollectBuildState(context.Background(), dir, ep)
	require.NoError(t, err)
	assert.Len(t, state.CompletedSprints, 1)
	assert.Equal(t, 1, state.HighestCompleted)
	require.NotNil(t, state.ActiveSprint)
	assert.Equal(t, 2, state.ActiveSprint.Number)
	assert.Equal(t, "Auth", state.ActiveSprint.Name)
	assert.Equal(t, 1, state.ActiveSprint.IterationCount)
}

func TestCollectDeferredFailures(t *testing.T) {
	t.Parallel()

	t.Run("no file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		assert.Nil(t, collectDeferredFailures(dir))
	})

	t.Run("with entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))

		content := "# Deferred Verification Failures\n\n" +
			"## Sprint 2: Auth\n\n" +
			"- DEFERRED: File missing or empty: src/lib/auth.ts\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.DeferredFailuresFile), []byte(content), 0o644))

		lines := collectDeferredFailures(dir)
		assert.Len(t, lines, 2)
		assert.Equal(t, "## Sprint 2: Auth", lines[0])
		assert.Contains(t, lines[1], "DEFERRED:")
	})
}

func TestExtractMaxSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"empty", "", ""},
		{"low only", "**Severity:** LOW", "LOW"},
		{"mixed", "**Severity:** LOW\n**Severity:** HIGH\n**Severity:** MODERATE", "HIGH"},
		{"critical", "Severity: CRITICAL", "CRITICAL"},
		{"no label no match", "This is HIGH priority work", ""},
		{"label required", "Severity: MODERATE\nHIGH effort task", "MODERATE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, extractMaxSeverity(tt.content))
		})
	}
}

func TestReadTail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	var lines []string
	for i := 1; i <= 200; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("%s\n", joinLines(lines))), 0o644))

	tail := readTail(path, 10)
	assert.Contains(t, tail, "line 200")
	assert.Contains(t, tail, "line 195")
	assert.NotContains(t, tail, "line 100")
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
