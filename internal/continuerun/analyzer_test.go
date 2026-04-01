package continuerun

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/steering"
)

type stubEngine struct {
	output     string
	exitCode   int
	err        error
	decisionFn func(workDir string)
}

func (s *stubEngine) Run(_ context.Context, _ string, opts engine.RunOpts) (string, int, error) {
	if s.decisionFn != nil {
		s.decisionFn(opts.WorkDir)
	}
	return s.output, s.exitCode, s.err
}

func (s *stubEngine) Name() string { return "stub" }

func TestParseDecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		output       string
		totalSprints int
		wantVerdict  ContinueVerdict
		wantSprint   int
		wantReason   string
	}{
		{
			name:         "resume",
			output:       "## Decision\n" + `{"verdict":"RESUME","sprint":4,"reason":"Sprint 4 code work is complete but Docker was not running."}`,
			totalSprints: 7,
			wantVerdict:  VerdictResume,
			wantSprint:   4,
			wantReason:   "Sprint 4 code work is complete but Docker was not running.",
		},
		{
			name:         "resume fresh",
			output:       `{"verdict":"RESUME_FRESH","sprint":2,"reason":"No work exists for sprint 2."}`,
			totalSprints: 5,
			wantVerdict:  VerdictResumeFresh,
			wantSprint:   2,
			wantReason:   "No work exists for sprint 2.",
		},
		{
			name:         "all complete",
			output:       `{"verdict":"ALL_COMPLETE","sprint":0,"reason":"All 5 sprints passed."}`,
			totalSprints: 5,
			wantVerdict:  VerdictAllComplete,
			wantSprint:   0,
			wantReason:   "All 5 sprints passed.",
		},
		{
			name:         "blocked",
			output:       "{\"verdict\":\"BLOCKED\",\"sprint\":3,\"reason\":\"Docker daemon is not running.\"}\n\n## Pre-conditions\n- [ ] Start Docker Desktop",
			totalSprints: 5,
			wantVerdict:  VerdictBlocked,
			wantSprint:   3,
			wantReason:   "Docker daemon is not running.",
		},
		{
			name:         "continue next",
			output:       `{"verdict":"CONTINUE_NEXT","sprint":4,"reason":"Sprint 3 work is complete, continue to next."}`,
			totalSprints: 7,
			wantVerdict:  VerdictContinueNext,
			wantSprint:   4,
			wantReason:   "Sprint 3 work is complete, continue to next.",
		},
		{
			name:         "audit incomplete",
			output:       `{"verdict":"AUDIT_INCOMPLETE","sprint":3,"reason":"All sprints passed but build audit sentinel is absent."}`,
			totalSprints: 3,
			wantVerdict:  VerdictAuditIncomplete,
			wantSprint:   3,
			wantReason:   "All sprints passed but build audit sentinel is absent.",
		},
		{
			name:         "unparseable output",
			output:       "I don't know what happened.",
			totalSprints: 5,
			wantVerdict:  VerdictBlocked,
			wantSprint:   0,
			wantReason:   "could not parse agent decision",
		},
		{
			name:         "empty output",
			output:       "",
			totalSprints: 5,
			wantVerdict:  VerdictBlocked,
			wantSprint:   0,
		},
		{
			name:         "sprint out of range ignored",
			output:       `{"verdict":"RESUME","sprint":99,"reason":"Bad sprint number."}`,
			totalSprints: 5,
			wantVerdict:  VerdictResume,
			wantSprint:   0, // invalid sprint is rejected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			decision := ParseDecision(tt.output, tt.totalSprints)
			assert.Equal(t, tt.wantVerdict, decision.Verdict)
			assert.Equal(t, tt.wantSprint, decision.StartSprint)
			if tt.wantReason != "" {
				assert.Equal(t, tt.wantReason, decision.Reason)
			}
		})
	}
}

func TestParsePreconditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "no preconditions",
			output:   "## Decision\n" + `{"verdict":"RESUME","sprint":1,"reason":"Resuming."}`,
			expected: nil,
		},
		{
			name: "one precondition",
			output: `## Pre-conditions
- [ ] Start Docker Desktop`,
			expected: []string{"Start Docker Desktop"},
		},
		{
			name: "multiple preconditions",
			output: `## Pre-conditions
- [ ] Start Docker Desktop
- [ ] Install pnpm
- [ ] Run npm install`,
			expected: []string{"Start Docker Desktop", "Install pnpm", "Run npm install"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, parsePreconditions(tt.output))
		})
	}
}

func TestHeuristicAnalyze(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		state       *BuildState
		wantVerdict ContinueVerdict
		wantSprint  int
	}{
		{
			name: "all sprints complete with audit done",
			state: &BuildState{
				TotalSprints: 3,
				CompletedSprints: []CompletedSprint{
					{Number: 1}, {Number: 2}, {Number: 3},
				},
				BuildAuditComplete: true,
			},
			wantVerdict: VerdictAllComplete,
			wantSprint:  3,
		},
		{
			name: "all sprints complete without audit",
			state: &BuildState{
				TotalSprints: 3,
				CompletedSprints: []CompletedSprint{
					{Number: 1}, {Number: 2}, {Number: 3},
				},
				BuildAuditComplete: false,
				AuditConfigured:    true,
			},
			wantVerdict: VerdictAuditIncomplete,
			wantSprint:  3,
		},
		{
			name: "all sprints complete audit not configured",
			state: &BuildState{
				TotalSprints: 3,
				CompletedSprints: []CompletedSprint{
					{Number: 1}, {Number: 2}, {Number: 3},
				},
				BuildAuditComplete: false,
				AuditConfigured:    false,
			},
			wantVerdict: VerdictAllComplete,
			wantSprint:  3,
		},
		{
			name: "no sprints complete returns sprint 1",
			state: &BuildState{
				TotalSprints:     3,
				CompletedSprints: nil,
			},
			wantVerdict: VerdictResume,
			wantSprint:  1,
		},
		{
			name: "middle sprint missing returns that sprint",
			state: &BuildState{
				TotalSprints: 4,
				CompletedSprints: []CompletedSprint{
					{Number: 1}, {Number: 2},
				},
			},
			wantVerdict: VerdictResume,
			wantSprint:  3,
		},
		{
			name:        "nil state returns all complete",
			state:       nil,
			wantVerdict: VerdictAllComplete,
			wantSprint:  0,
		},
		{
			name:        "zero total sprints returns all complete",
			state:       &BuildState{TotalSprints: 0},
			wantVerdict: VerdictAllComplete,
			wantSprint:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			decision := HeuristicAnalyze(tt.state)
			assert.Equal(t, tt.wantVerdict, decision.Verdict)
			assert.Equal(t, tt.wantSprint, decision.StartSprint)
		})
	}
}

func TestHeuristicAnalyzeResumePoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		point        *steering.ResumePoint
		totalSprints int
		wantVerdict  ContinueVerdict
		wantSprint   int
	}{
		{
			name: "resume sprint",
			point: &steering.ResumePoint{
				Verdict: steering.ResumeVerdictResume,
				Sprint:  6,
				Reason:  "after sprint audit",
			},
			totalSprints: 8,
			wantVerdict:  VerdictResume,
			wantSprint:   6,
		},
		{
			name: "continue next",
			point: &steering.ResumePoint{
				Verdict: steering.ResumeVerdictContinueNext,
				Sprint:  3,
				Reason:  "after sprint compaction",
			},
			totalSprints: 8,
			wantVerdict:  VerdictContinueNext,
			wantSprint:   4,
		},
		{
			name: "build audit incomplete",
			point: &steering.ResumePoint{
				Verdict: steering.ResumeVerdictAuditIncomplete,
				Sprint:  8,
				Reason:  "before build audit",
			},
			totalSprints: 8,
			wantVerdict:  VerdictAuditIncomplete,
			wantSprint:   8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			decision := HeuristicAnalyze(&BuildState{
				TotalSprints: tt.totalSprints,
				ResumePoint:  tt.point,
			})
			assert.Equal(t, tt.wantVerdict, decision.Verdict)
			assert.Equal(t, tt.wantSprint, decision.StartSprint)
			assert.Equal(t, tt.point.Reason, decision.Reason)
		})
	}
}

func TestHeuristicAnalyzeResumeFromFirst(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		TotalSprints:     3,
		CompletedSprints: nil, // none completed
	}
	decision := HeuristicAnalyze(state)
	assert.Equal(t, VerdictResume, decision.Verdict)
	assert.Equal(t, 1, decision.StartSprint)
}

func TestHeuristicAnalyzeResumeFromMiddle(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		TotalSprints: 3,
		CompletedSprints: []CompletedSprint{
			{Number: 1},
		},
	}
	decision := HeuristicAnalyze(state)
	assert.Equal(t, VerdictResume, decision.Verdict)
	assert.Equal(t, 2, decision.StartSprint)
}

func TestHeuristicAnalyzeAllComplete(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		TotalSprints: 2,
		CompletedSprints: []CompletedSprint{
			{Number: 1}, {Number: 2},
		},
		BuildAuditComplete: true,
	}
	decision := HeuristicAnalyze(state)
	assert.Equal(t, VerdictAllComplete, decision.Verdict)
	assert.Equal(t, 2, decision.StartSprint)
}

func TestHeuristicAnalyzeAuditIncomplete(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		TotalSprints: 2,
		CompletedSprints: []CompletedSprint{
			{Number: 1}, {Number: 2},
		},
		BuildAuditComplete: false,
		AuditConfigured:    true,
	}
	decision := HeuristicAnalyze(state)
	assert.Equal(t, VerdictAuditIncomplete, decision.Verdict)
	assert.Equal(t, 2, decision.StartSprint)
}

// TestHeuristicAnalyzeAuditComplete is a named standalone required by acceptance criteria.
// It mirrors the table case "all sprints complete with audit done" with a different sprint count.
func TestHeuristicAnalyzeAuditComplete(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		TotalSprints: 3,
		CompletedSprints: []CompletedSprint{
			{Number: 1}, {Number: 2}, {Number: 3},
		},
		BuildAuditComplete: true,
	}
	decision := HeuristicAnalyze(state)
	assert.Equal(t, VerdictAllComplete, decision.Verdict)
	assert.Equal(t, 3, decision.StartSprint)
}

// TestHeuristicAnalyzeBackwardCompatNilSentinel is a named standalone required by acceptance criteria.
// It validates the safe-default sentinel-absent path: when BuildAuditComplete is its zero value
// (false), the analyzer returns AUDIT_INCOMPLETE regardless of whether the build predates the
// sentinel feature or simply hasn't completed its audit yet.
func TestHeuristicAnalyzeBackwardCompatNilSentinel(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		TotalSprints: 2,
		CompletedSprints: []CompletedSprint{
			{Number: 1}, {Number: 2},
		},
		AuditConfigured: true,
	}
	decision := HeuristicAnalyze(state)
	assert.Equal(t, VerdictAuditIncomplete, decision.Verdict)
	assert.Equal(t, 2, decision.StartSprint)
}

func TestHeuristicAnalyzeNoSprints(t *testing.T) {
	t.Parallel()

	decision := HeuristicAnalyze(&BuildState{TotalSprints: 0})
	assert.Equal(t, VerdictAllComplete, decision.Verdict)
}

func TestBuildAnalysisPrompt(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 3,
		Engine:       "codex",
		EffortLevel:  "high",
		SprintNames:  []string{"Setup", "Auth", "API"},
	}

	report := FormatReport(state)
	prompt := buildAnalysisPrompt(state, report)

	assert.Contains(t, prompt, "Continue Analysis")
	assert.Contains(t, prompt, "build analyst")
	assert.Contains(t, prompt, "RESUME")
	assert.Contains(t, prompt, "RESUME_FRESH")
	assert.Contains(t, prompt, "CONTINUE_NEXT")
	assert.Contains(t, prompt, "ALL_COMPLETE")
	assert.Contains(t, prompt, "BLOCKED")
	assert.Contains(t, prompt, "AUDIT_INCOMPLETE")
	assert.Contains(t, prompt, `"verdict"`)
	assert.Contains(t, prompt, `"sprint"`)
	assert.Contains(t, prompt, `"reason"`)
	assert.Contains(t, prompt, "continue-decision.txt")
	assert.Contains(t, prompt, "TestEpic")
}

func TestAnalyzeSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry", "build-logs"), 0o755))

	state := BuildState{TotalSprints: 2, EpicName: "Test"}
	stub := stubEngine{
		output: `{"verdict":"RESUME","sprint":1,"reason":"Sprint 1 incomplete."}`,
	}

	decision, err := Analyze(context.Background(), AnalyzeOpts{
		ProjectDir: dir,
		State:      &state,
		Engine:     &stub,
		Model:      "test-model",
	})

	require.NoError(t, err)
	assert.Equal(t, VerdictResume, decision.Verdict)
	assert.Equal(t, 1, decision.StartSprint)
	assert.FileExists(t, filepath.Join(dir, config.ContinueReportFile))
	assert.FileExists(t, filepath.Join(dir, config.ContinuePromptFile))
}

func TestAnalyzeReadsDecisionFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry", "build-logs"), 0o755))

	state := BuildState{TotalSprints: 2, EpicName: "Test"}
	stub := stubEngine{
		output: `{"verdict":"BLOCKED"}`,
		decisionFn: func(workDir string) {
			content := `{"verdict":"ALL_COMPLETE","sprint":2,"reason":"Done."}`
			_ = os.WriteFile(filepath.Join(workDir, config.ContinueDecisionFile), []byte(content), 0o644)
		},
	}

	decision, err := Analyze(context.Background(), AnalyzeOpts{
		ProjectDir: dir,
		State:      &state,
		Engine:     &stub,
		Model:      "test-model",
	})

	require.NoError(t, err)
	assert.Equal(t, VerdictAllComplete, decision.Verdict)
}

func TestAnalyzeEngineNil(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	state := BuildState{TotalSprints: 2, EpicName: "Test"}

	_, err := Analyze(context.Background(), AnalyzeOpts{
		ProjectDir: dir,
		State:      &state,
		Engine:     nil,
		Model:      "test-model",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

func TestAnalyzeContextCancelled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := BuildState{TotalSprints: 2, EpicName: "Test"}
	stub := stubEngine{
		err: ctx.Err(),
	}

	_, err := Analyze(ctx, AnalyzeOpts{
		ProjectDir: dir,
		State:      &state,
		Engine:     &stub,
		Model:      "test-model",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
