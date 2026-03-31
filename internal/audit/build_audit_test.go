package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/epic"
)

func makeBuildOpts(t *testing.T, eng *stubEngine) BuildAuditOpts {
	t.Helper()
	projectDir := t.TempDir()
	return BuildAuditOpts{
		ProjectDir: projectDir,
		Epic: &epic.Epic{
			Name:             "TestEpic",
			TotalSprints:     2,
			AuditAfterSprint: true,
			EffortLevel:      epic.EffortHigh,
		},
		Engine:  eng,
		Verbose: false,
		Model:   "sonnet",
		Mode:    "software",
	}
}

func TestRunBuildAuditNilEpic(t *testing.T) {
	t.Parallel()

	_, err := RunBuildAudit(context.Background(), BuildAuditOpts{
		Engine: &stubEngine{name: "claude"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "epic is required")
}

func TestRunBuildAuditNilEngine(t *testing.T) {
	t.Parallel()

	_, err := RunBuildAudit(context.Background(), BuildAuditOpts{
		Epic: &epic.Epic{Name: "Test"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

func TestRunBuildAuditReturnsPassOnCleanReport(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "claude",
		sideEffect: func(projectDir string, callIndex int) {
			// Agent writes a clean audit report
			path := filepath.Join(projectDir, config.BuildAuditFile)
			_ = os.WriteFile(path, []byte(cleanAudit), 0o644)
		},
	}
	opts := makeBuildOpts(t, eng)

	result, err := RunBuildAudit(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed)
	assert.False(t, result.Blocking)
}

func TestRunBuildAuditReturnsFailOnCritical(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "claude",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.BuildAuditFile)
			_ = os.WriteFile(path, []byte(criticalFindings), 0o644)
		},
	}
	opts := makeBuildOpts(t, eng)

	result, err := RunBuildAudit(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, "CRITICAL", result.MaxSeverity)
	assert.Equal(t, 1, result.SeverityCounts["CRITICAL"])
}

func TestRunBuildAuditReturnsFailOnHigh(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "claude",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.BuildAuditFile)
			_ = os.WriteFile(path, []byte(highFindings), 0o644)
		},
	}
	opts := makeBuildOpts(t, eng)

	result, err := RunBuildAudit(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, "HIGH", result.MaxSeverity)
}

func TestRunBuildAuditReturnsAdvisoryOnModerate(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "claude",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.BuildAuditFile)
			_ = os.WriteFile(path, []byte(moderateFindings), 0o644)
		},
	}
	opts := makeBuildOpts(t, eng)

	result, err := RunBuildAudit(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.False(t, result.Blocking)
	assert.Equal(t, "MODERATE", result.MaxSeverity)
}

func TestRunBuildAuditHandlesNoFile(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "claude",
		// Agent does not write a report file
	}
	opts := makeBuildOpts(t, eng)

	_, err := RunBuildAudit(context.Background(), opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not write")
}

func TestRunBuildAuditPopulatesSeverityCounts(t *testing.T) {
	t.Parallel()

	multiFindings := "## Findings\n" +
		"- **Location:** a.go:1\n- **Description:** Issue A\n- **Severity:** HIGH\n" +
		"- **Location:** b.go:2\n- **Description:** Issue B\n- **Severity:** MODERATE\n" +
		"- **Location:** c.go:3\n- **Description:** Issue C\n- **Severity:** MODERATE\n" +
		"- **Location:** d.go:4\n- **Description:** Issue D\n- **Severity:** LOW\n"

	eng := &stubEngine{
		name: "claude",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.BuildAuditFile)
			_ = os.WriteFile(path, []byte(multiFindings), 0o644)
		},
	}
	opts := makeBuildOpts(t, eng)

	result, err := RunBuildAudit(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.SeverityCounts["HIGH"])
	assert.Equal(t, 2, result.SeverityCounts["MODERATE"])
	assert.Equal(t, 1, result.SeverityCounts["LOW"])
}

func TestRunBuildAuditPopulatesFindings(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "claude",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.BuildAuditFile)
			_ = os.WriteFile(path, []byte(criticalFindings), 0o644)
		},
	}
	opts := makeBuildOpts(t, eng)

	result, err := RunBuildAudit(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.UnresolvedFindings, 1)
	assert.Equal(t, "Null pointer dereference", result.UnresolvedFindings[0].Description)
	assert.Equal(t, "CRITICAL", result.UnresolvedFindings[0].Severity)
	assert.Equal(t, "src/main.go:10", result.UnresolvedFindings[0].Location)
}

func TestRunBuildAuditContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	eng := &stubEngine{
		name: "claude",
	}
	opts := BuildAuditOpts{
		ProjectDir: t.TempDir(),
		Epic:       &epic.Epic{Name: "Test", TotalSprints: 1},
		Engine:     eng,
		Model:      "sonnet",
	}

	_, err := RunBuildAudit(ctx, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunBuildAuditPassOnLowOnly(t *testing.T) {
	t.Parallel()

	lowFindings := "## Findings\n- **Description:** Minor naming issue\n- **Severity:** LOW\n"

	eng := &stubEngine{
		name: "claude",
		sideEffect: func(projectDir string, callIndex int) {
			path := filepath.Join(projectDir, config.BuildAuditFile)
			_ = os.WriteFile(path, []byte(lowFindings), 0o644)
		},
	}
	opts := makeBuildOpts(t, eng)

	result, err := RunBuildAudit(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed)
	assert.False(t, result.Blocking)
	assert.Equal(t, "LOW", result.MaxSeverity)
}

func TestRunBuildAuditPropagatesAgentError(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "claude",
		errs: []error{context.DeadlineExceeded},
	}
	opts := makeBuildOpts(t, eng)

	_, err := RunBuildAudit(context.Background(), opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent run")
}

func TestBuildAuditPromptIncludesCodebaseContext(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{name: "claude"}
	opts := makeBuildOpts(t, eng)
	require.NoError(t, os.MkdirAll(filepath.Join(opts.ProjectDir, config.CodebaseMemoriesDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(opts.ProjectDir, config.CodebaseFile), []byte("# Codebase: Fry\n\nPersistent architecture details."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(opts.ProjectDir, config.CodebaseMemoriesDir, "001-memory.md"), []byte(`---
confidence: high
source: build-1
sprint: 1
date: 2026-03-31
reinforced: 0
---
Build audit findings should be reconciled with established package boundaries.`), 0o644))

	prompt := buildBuildAuditPrompt(opts)

	assert.Contains(t, prompt, "## Codebase Context")
	assert.Contains(t, prompt, "Persistent architecture details.")
	assert.Contains(t, prompt, "## Codebase Memories")
	assert.Contains(t, prompt, "established package boundaries")
}
