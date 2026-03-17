package continuerun

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatReport_FreshBuild(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 3,
		Engine:       "codex",
		EffortLevel:  "high",
		SprintNames:  []string{"Setup", "Auth", "API"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "TestEpic")
	assert.Contains(t, report, "3 sprints")
	assert.Contains(t, report, "mode: software")
	assert.Contains(t, report, "Completed Sprints")
	assert.Contains(t, report, "None")
	assert.Contains(t, report, "Next Sprint: 1")
}

func TestFormatReport_PartialBuild(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 5,
		Engine:       "claude",
		EffortLevel:  "max",
		CompletedSprints: []CompletedSprint{
			{Number: 1, Name: "Setup", Status: "PASS"},
			{Number: 2, Name: "Auth", Status: "PASS (healed)"},
		},
		HighestCompleted: 2,
		ActiveSprints: []ActiveSprintState{
			{
				Number:         3,
				Name:           "API",
				IterationCount: 2,
				AuditCount:     1,
				LastLogTail:    "Error: docker up: exit status 1",
			},
		},
		DockerRequired:  true,
		DockerAvailable: false,
		SprintNames:     []string{"Setup", "Auth", "API", "UI", "Tests"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "Sprint 1: Setup")
	assert.Contains(t, report, "Sprint 2: Auth")
	assert.Contains(t, report, "PASS (healed)")
	assert.Contains(t, report, "Next Sprint: 3")
	assert.Contains(t, report, "Partial Work Detected (1 incomplete sprint(s))")
	assert.Contains(t, report, "### Sprint 3: API")
	assert.Contains(t, report, "2 iterations completed")
	assert.Contains(t, report, "docker up")
	assert.Contains(t, report, "NOT RUNNING")
}

func TestFormatReport_AllComplete(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 2,
		Engine:       "codex",
		EffortLevel:  "low",
		CompletedSprints: []CompletedSprint{
			{Number: 1, Name: "Setup", Status: "PASS"},
			{Number: 2, Name: "Auth", Status: "PASS"},
		},
		HighestCompleted: 2,
		SprintNames:      []string{"Setup", "Auth"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "All sprints complete")
}

func TestFormatReport_WritingMode(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "WritingEpic",
		TotalSprints: 2,
		Engine:       "claude",
		EffortLevel:  "high",
		Mode:         "writing",
		SprintNames:  []string{"Chapter 1", "Chapter 2"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "mode: writing")
}

func TestFormatReport_MultipleActiveSprints(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 7,
		Engine:       "claude",
		EffortLevel:  "high",
		CompletedSprints: []CompletedSprint{
			{Number: 2, Name: "Services", Status: "PASS"},
			{Number: 3, Name: "Projections", Status: "PASS"},
			{Number: 4, Name: "Pages", Status: "PASS (healed)"},
			{Number: 5, Name: "Auth", Status: "PASS"},
		},
		HighestCompleted: 5,
		ActiveSprints: []ActiveSprintState{
			{
				Number:         1,
				Name:           "Scaffold",
				IterationCount: 1,
				AuditCount:     7,
				LastLogTail:    "===PROMISE: SPRINT1_DONE===",
			},
			{
				Number:          6,
				Name:            "Dashboard",
				IterationCount:  1,
				AuditCount:      3,
				AuditSeverity:   "HIGH",
				ProgressExcerpt: "Remaining: Nothing — sprint is complete.",
			},
		},
		SprintNames: []string{"Scaffold", "Services", "Projections", "Pages", "Auth", "Dashboard", "MCP"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "Partial Work Detected (2 incomplete sprint(s))")
	assert.Contains(t, report, "### Sprint 1: Scaffold")
	assert.Contains(t, report, "### Sprint 6: Dashboard")
	assert.Contains(t, report, "Last audit severity: HIGH")
	assert.Contains(t, report, "SPRINT1_DONE")
	assert.Contains(t, report, "Next Sprint: 1")
}

func TestFormatReport_Environment(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 2,
		Engine:       "codex",
		EffortLevel:  "high",
		RequiredTools: []ToolStatus{
			{Name: "node", Available: true},
			{Name: "pnpm", Available: true},
			{Name: "docker", Available: false},
		},
		GitClean:   true,
		GitBranch:  "master",
		SprintNames: []string{"Setup", "Auth"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "node (ok)")
	assert.Contains(t, report, "docker (MISSING)")
	assert.Contains(t, report, "clean")
	assert.Contains(t, report, "master")
}
