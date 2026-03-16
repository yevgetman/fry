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
		ActiveSprint: &ActiveSprintState{
			Number:         3,
			Name:           "API",
			IterationCount: 2,
			AuditCount:     1,
			LastLogTail:    "Error: docker up: exit status 1",
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
	assert.Contains(t, report, "Partial Work Detected for Sprint 3")
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
