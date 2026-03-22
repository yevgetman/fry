package continuerun

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			name: "resume",
			output: `## Decision
<verdict>RESUME</verdict>
<sprint>4</sprint>
<reason>Sprint 4 code work is complete but Docker was not running.</reason>`,
			totalSprints: 7,
			wantVerdict:  VerdictResume,
			wantSprint:   4,
			wantReason:   "Sprint 4 code work is complete but Docker was not running.",
		},
		{
			name: "resume fresh",
			output: `<verdict>RESUME_FRESH</verdict>
<sprint>2</sprint>
<reason>No work exists for sprint 2.</reason>`,
			totalSprints: 5,
			wantVerdict:  VerdictResumeFresh,
			wantSprint:   2,
			wantReason:   "No work exists for sprint 2.",
		},
		{
			name: "all complete",
			output: `<verdict>ALL_COMPLETE</verdict>
<sprint>0</sprint>
<reason>All 5 sprints passed.</reason>`,
			totalSprints: 5,
			wantVerdict:  VerdictAllComplete,
			wantSprint:   0,
			wantReason:   "All 5 sprints passed.",
		},
		{
			name: "blocked",
			output: `<verdict>BLOCKED</verdict>
<sprint>3</sprint>
<reason>Docker daemon is not running.</reason>

## Pre-conditions
- [ ] Start Docker Desktop`,
			totalSprints: 5,
			wantVerdict:  VerdictBlocked,
			wantSprint:   3,
			wantReason:   "Docker daemon is not running.",
		},
		{
			name: "continue next",
			output: `<verdict>CONTINUE_NEXT</verdict>
<sprint>4</sprint>
<reason>Sprint 3 work is complete, continue to next.</reason>`,
			totalSprints: 7,
			wantVerdict:  VerdictContinueNext,
			wantSprint:   4,
			wantReason:   "Sprint 3 work is complete, continue to next.",
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
			name: "sprint out of range ignored",
			output: `<verdict>RESUME</verdict>
<sprint>99</sprint>
<reason>Bad sprint number.</reason>`,
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
			output:   "## Decision\n<verdict>RESUME</verdict>",
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
			name: "all sprints complete",
			state: &BuildState{
				TotalSprints: 3,
				CompletedSprints: []CompletedSprint{
					{Number: 1}, {Number: 2}, {Number: 3},
				},
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
	}
	decision := HeuristicAnalyze(state)
	assert.Equal(t, VerdictAllComplete, decision.Verdict)
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
	assert.Contains(t, prompt, "<verdict>")
	assert.Contains(t, prompt, "<sprint>")
	assert.Contains(t, prompt, "<reason>")
	assert.Contains(t, prompt, "continue-decision.txt")
	assert.Contains(t, prompt, "TestEpic")
}
