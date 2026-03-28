package summary

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/audit"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/sprint"
)

// --- stub engine ---

type stubSummaryEngine struct {
	output string
	err    error
}

func (s *stubSummaryEngine) Run(_ context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(s.output))
	}
	return s.output, 0, s.err
}

func (s *stubSummaryEngine) Name() string { return "stub" }

// --- tests ---

func TestBuildSummaryPrompt_IncludesEpicName(t *testing.T) {
	t.Parallel()

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: t.TempDir(),
		EpicName:   "My Epic",
	})

	assert.Contains(t, prompt, "# BUILD SUMMARY GENERATION")
	assert.Contains(t, prompt, "**My Epic**")
	assert.Contains(t, prompt, "## Your Role")
	assert.Contains(t, prompt, "## Output Instructions")
	assert.Contains(t, prompt, config.SummaryFile)
}

func TestBuildSummaryPrompt_IncludesSprintResults(t *testing.T) {
	t.Parallel()

	results := []sprint.SprintResult{
		{Number: 1, Name: "Setup", Status: "PASS", Duration: 30 * time.Second},
		{Number: 2, Name: "Build", Status: "FAIL (alignment exhausted)", Duration: 2 * time.Minute, AuditWarning: "MODERATE issues"},
	}
	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: t.TempDir(),
		EpicName:   "Test",
		Results:    results,
	})

	assert.Contains(t, prompt, "| 1 | Setup | PASS |")
	assert.Contains(t, prompt, "| 2 | Build | FAIL (alignment exhausted) |")
	assert.Contains(t, prompt, "MODERATE issues")
}

func TestBuildSummaryPrompt_IncludesEpicFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	epicPath := filepath.Join(projectDir, config.FryDir, "epic.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(epicPath), 0o755))
	require.NoError(t, os.WriteFile(epicPath, []byte("@epic TestEpic\n@sprint 1\n"), 0o644))

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "TestEpic",
	})

	assert.Contains(t, prompt, "## Epic Definition")
	assert.Contains(t, prompt, "@epic TestEpic")
}

func TestBuildSummaryPrompt_TruncatesLargeEpicFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	epicPath := filepath.Join(projectDir, config.FryDir, "epic.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(epicPath), 0o755))
	largeContent := strings.Repeat("x", maxEpicFileBytes+1000)
	require.NoError(t, os.WriteFile(epicPath, []byte(largeContent), 0o644))

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Big",
	})

	assert.Contains(t, prompt, "...(truncated)")
}

func TestBuildSummaryPrompt_IncludesEpicProgress(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(projectDir, config.EpicProgressFile)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.EpicProgressFile), []byte("Sprint 1 complete\n"), 0o644))

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
	})

	assert.Contains(t, prompt, "## Epic Progress")
	assert.Contains(t, prompt, "Sprint 1 complete")
}

func TestBuildSummaryPrompt_IncludesDeferredFailures(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(projectDir, config.DeferredFailuresFile)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.DeferredFailuresFile), []byte("check_file missing.txt FAIL\n"), 0o644))

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
	})

	assert.Contains(t, prompt, "## Deferred Sanity Check Failures")
	assert.Contains(t, prompt, "check_file missing.txt FAIL")
}

func TestBuildSummaryPrompt_IncludesDeviationLog(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(projectDir, config.DeviationLogFile)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.DeviationLogFile), []byte("deviation at sprint 2\n"), 0o644))

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
	})

	assert.Contains(t, prompt, "## Deviation Log")
	assert.Contains(t, prompt, "deviation at sprint 2")
}

func TestBuildSummaryPrompt_IncludesBuildLogs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	logsDir := filepath.Join(projectDir, config.BuildLogsDir)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint1_20260318_100000.log"), []byte("log content A\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint2_20260318_110000.log"), []byte("log content B\n"), 0o644))

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
	})

	assert.Contains(t, prompt, "## Build Logs")
	assert.Contains(t, prompt, "sprint1_20260318_100000.log")
	assert.Contains(t, prompt, "log content A")
	assert.Contains(t, prompt, "sprint2_20260318_110000.log")
	assert.Contains(t, prompt, "log content B")
}

func TestBuildSummaryPrompt_CapsLogBytes(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	logsDir := filepath.Join(projectDir, config.BuildLogsDir)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	bigLog := strings.Repeat("x", maxLogBytes+500)
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "big.log"), []byte(bigLog), 0o644))

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
	})

	assert.Contains(t, prompt, "...(truncated)")
}

func TestBuildSummaryPrompt_CapsTotalLogBytes(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	logsDir := filepath.Join(projectDir, config.BuildLogsDir)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Create enough logs to exceed total cap
	logContent := strings.Repeat("x", maxLogBytes-100)
	count := (maxTotalLogCap / (maxLogBytes - 100)) + 2
	for i := 0; i < count; i++ {
		name := filepath.Join(logsDir, fmt.Sprintf("log_%02d.log", i))
		require.NoError(t, os.WriteFile(name, []byte(logContent), 0o644))
	}

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
	})

	assert.Contains(t, prompt, "remaining logs omitted")
}

func TestCollectBuildLogs_Empty(t *testing.T) {
	t.Parallel()

	entries := collectBuildLogs(filepath.Join(t.TempDir(), "nonexistent"))
	assert.Nil(t, entries)
}

func TestCollectBuildLogs_SkipsDirectoriesAndNonLogFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a log\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sprint1.log"), []byte("log content\n"), 0o644))

	entries := collectBuildLogs(dir)
	require.Len(t, entries, 1)
	assert.Equal(t, "sprint1.log", entries[0].name)
	assert.Equal(t, "log content\n", entries[0].content)
}

func TestCollectBuildLogs_SortedByName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "z.log"), []byte("z\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.log"), []byte("a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "m.log"), []byte("m\n"), 0o644))

	entries := collectBuildLogs(dir)
	require.Len(t, entries, 3)
	assert.Equal(t, "a.log", entries[0].name)
	assert.Equal(t, "m.log", entries[1].name)
	assert.Equal(t, "z.log", entries[2].name)
}

func TestGenerateBuildSummary_NilEngine(t *testing.T) {
	t.Parallel()

	err := GenerateBuildSummary(context.Background(), SummaryOpts{
		ProjectDir: t.TempDir(),
		EpicName:   "Test",
		Engine:     nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

func TestGenerateBuildSummary_EngineWritesSummary(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	summaryPath := filepath.Join(projectDir, config.SummaryFile)

	wrapper := &writingSummaryEngine{
		projectDir:  projectDir,
		summaryText: "# Build Summary\nAll sprints passed.\n",
	}

	err := GenerateBuildSummary(context.Background(), SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
		Engine:     wrapper,
	})
	require.NoError(t, err)

	data, err := os.ReadFile(summaryPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Build Summary")
}

type writingSummaryEngine struct {
	projectDir  string
	summaryText string
}

func (w *writingSummaryEngine) Run(_ context.Context, _ string, opts engine.RunOpts) (string, int, error) {
	summaryPath := filepath.Join(w.projectDir, config.SummaryFile)
	_ = os.WriteFile(summaryPath, []byte(w.summaryText), 0o644)
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(w.summaryText))
	}
	return w.summaryText, 0, nil
}

func (w *writingSummaryEngine) Name() string { return "stub" }

func TestGenerateBuildSummary_EngineErrorNonFatal(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	eng := &stubSummaryEngine{
		output: "partial output",
		err:    errors.New("exit code 1"),
	}

	// Should not return error when engine fails but context is not cancelled
	err := GenerateBuildSummary(context.Background(), SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
		Engine:     eng,
	})
	// No summary file written → warning logged but no error
	require.NoError(t, err)
}

func TestGenerateBuildSummary_ContextCancelled(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	eng := &stubSummaryEngine{
		output: "",
		err:    ctx.Err(),
	}

	err := GenerateBuildSummary(ctx, SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
		Engine:     eng,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestBuildSummaryPrompt_NoOptionalFiles(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Empty",
		Results:    nil,
	})

	// Should still contain structure even without optional files
	assert.Contains(t, prompt, "# BUILD SUMMARY GENERATION")
	assert.Contains(t, prompt, "## Sprint Results")
	assert.Contains(t, prompt, "## Build Logs")
	assert.Contains(t, prompt, "## Output Instructions")
	// Should NOT contain optional sections
	assert.NotContains(t, prompt, "## Epic Definition")
	assert.NotContains(t, prompt, "## Epic Progress")
	assert.NotContains(t, prompt, "## Deferred Sanity Check Failures")
	assert.NotContains(t, prompt, "## Deviation Log")
	assert.NotContains(t, prompt, "## Build Audit Results")
}

func TestBuildSummaryPromptWithBuildAuditResult(t *testing.T) {
	t.Parallel()

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: t.TempDir(),
		EpicName:   "AuditTest",
		BuildAuditResult: &audit.AuditResult{
			Passed:      false,
			Blocking:    true,
			Iterations:  1,
			MaxSeverity: "HIGH",
			SeverityCounts: map[string]int{
				"HIGH":     2,
				"MODERATE": 1,
			},
			UnresolvedFindings: []audit.Finding{
				{Location: "main.go:10", Description: "Missing error check", Severity: "HIGH"},
				{Location: "api.go:42", Description: "SQL injection", Severity: "HIGH"},
				{Description: "Code duplication", Severity: "MODERATE"},
			},
		},
	})

	assert.Contains(t, prompt, "## Build Audit Results")
	assert.Contains(t, prompt, "**Status:** FAIL")
	assert.Contains(t, prompt, "**Max Severity:** HIGH")
	assert.Contains(t, prompt, "**Blocking:** yes")
	assert.Contains(t, prompt, "### Unresolved Findings")
	assert.Contains(t, prompt, "[main.go:10] Missing error check (**HIGH**)")
	assert.Contains(t, prompt, "[api.go:42] SQL injection (**HIGH**)")
	assert.Contains(t, prompt, "[(no location)] Code duplication (**MODERATE**)")
}

func TestBuildSummaryPromptWithPassingBuildAudit(t *testing.T) {
	t.Parallel()

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: t.TempDir(),
		EpicName:   "PassTest",
		BuildAuditResult: &audit.AuditResult{
			Passed:         true,
			Iterations:     1,
			SeverityCounts: map[string]int{},
		},
	})

	assert.Contains(t, prompt, "## Build Audit Results")
	assert.Contains(t, prompt, "**Status:** PASS")
	assert.NotContains(t, prompt, "**Blocking:**")
	assert.NotContains(t, prompt, "### Unresolved Findings")
}

func TestBuildSummaryPromptWithoutBuildAuditResult(t *testing.T) {
	t.Parallel()

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir:       t.TempDir(),
		EpicName:         "NoAudit",
		BuildAuditResult: nil,
	})

	assert.NotContains(t, prompt, "## Build Audit Results")
}

func TestGenerateBuildSummary_NonNotExistError(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	// Use a custom engine that removes read+execute permission on projectDir
	// during its Run() call. This way all earlier steps (MkdirAll, WriteFile,
	// log creation) succeed, but the os.Stat(summaryPath) call after the engine
	// returns gets EACCES (permission denied), not ENOENT.
	eng := &chmodEngine{dir: projectDir}

	err := GenerateBuildSummary(context.Background(), SummaryOpts{
		ProjectDir: projectDir,
		EpicName:   "Test",
		Engine:     eng,
	})

	// Restore permissions so t.TempDir() cleanup succeeds.
	os.Chmod(projectDir, 0o755)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "check summary file")
}

// chmodEngine removes read+execute permission on dir during Run(),
// causing subsequent os.Stat calls in the project directory to fail
// with permission denied (EACCES) rather than not-exist (ENOENT).
type chmodEngine struct {
	dir string
}

func (e *chmodEngine) Run(_ context.Context, _ string, opts engine.RunOpts) (string, int, error) {
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte("done"))
	}
	os.Chmod(e.dir, 0o000)
	return "done", 0, nil
}

func (e *chmodEngine) Name() string { return "chmod-stub" }

func TestBuildSummaryPromptIncludesBuildAuditInstructions(t *testing.T) {
	t.Parallel()

	prompt := buildSummaryPrompt(SummaryOpts{
		ProjectDir: t.TempDir(),
		EpicName:   "Test",
	})

	assert.Contains(t, prompt, "build-level audit results")
}
