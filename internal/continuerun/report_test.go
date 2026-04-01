package continuerun

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yevgetman/fry/internal/archive"
	"github.com/yevgetman/fry/internal/steering"
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
			{Number: 2, Name: "Auth", Status: "PASS (aligned)"},
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
	assert.Contains(t, report, "PASS (aligned)")
	assert.Contains(t, report, "Next Sprint: 3")
	assert.Contains(t, report, "Partial Work Detected (1 incomplete sprint(s))")
	assert.Contains(t, report, "### Sprint 3: API")
	assert.Contains(t, report, "2 iterations completed")
	assert.Contains(t, report, "docker up")
	assert.Contains(t, report, "NOT RUNNING")
}

func TestFormatReport_FailedSprint(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 3,
		Engine:       "claude",
		EffortLevel:  "max",
		FailedSprints: []FailedSprint{
			{Number: 1, Name: "Setup", Status: "FAIL (audit: HIGH)"},
		},
		SprintNames: []string{"Setup", "Auth", "API"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "## Failed Sprints")
	assert.Contains(t, report, "FAIL (audit: HIGH)")
	assert.Contains(t, report, "Completed Sprints\nNone")
	assert.Contains(t, report, "Next Sprint: 1")
}

func TestFormatReport_MixedPassAndFail(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 5,
		Engine:       "claude",
		EffortLevel:  "max",
		CompletedSprints: []CompletedSprint{
			{Number: 1, Name: "Setup", Status: "PASS"},
		},
		HighestCompleted: 1,
		FailedSprints: []FailedSprint{
			{Number: 2, Name: "Auth", Status: "FAIL"},
		},
		SprintNames: []string{"Setup", "Auth", "API", "UI", "Tests"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "## Completed Sprints")
	assert.Contains(t, report, "## Failed Sprints")
	assert.Contains(t, report, "Next Sprint: 2")
}

func TestFormatReport_ExitReason(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 9,
		Engine:       "claude",
		EffortLevel:  "max",
		CompletedSprints: []CompletedSprint{
			{Number: 1, Name: "Setup", Status: "PASS (aligned)"},
			{Number: 2, Name: "Models", Status: "PASS (aligned)"},
		},
		HighestCompleted: 2,
		ExitReason:       "After sprint 2: sprint 6 is outside deviation scope (max: sprint 5)",
		SprintNames:      []string{"Setup", "Models", "Services", "ViewModels", "Today UI", "Detail UI", "Tomorrow", "Polish", "Final"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "## Last Run Stopped")
	assert.Contains(t, report, "outside deviation scope")
	assert.Contains(t, report, "Next Sprint: 3")
}

func TestFormatReport_ResumePoint(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "Checkout",
		TotalSprints: 6,
		Engine:       "codex",
		EffortLevel:  "high",
		ResumePoint: &steering.ResumePoint{
			Phase:              "sprint_audit",
			Verdict:            steering.ResumeVerdictResume,
			Reason:             "after audit verify 1 in cycle 2",
			Sprint:             6,
			SprintName:         "Polish auth",
			RecommendedCommand: "fry run --resume --sprint 6",
		},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "## Resume Point")
	assert.Contains(t, report, "Verdict: RESUME")
	assert.Contains(t, report, "Sprint: 6 (Polish auth)")
	assert.Contains(t, report, "Recommended command: `fry run --resume --sprint 6`")
}

func TestFormatReport_AllComplete(t *testing.T) {
	t.Parallel()

	state := &BuildState{
		EpicName:     "TestEpic",
		TotalSprints: 2,
		Engine:       "codex",
		EffortLevel:  "fast",
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
			{Number: 4, Name: "Pages", Status: "PASS (aligned)"},
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
		GitClean:    true,
		GitBranch:   "master",
		SprintNames: []string{"Setup", "Auth"},
	}

	report := FormatReport(state)
	assert.Contains(t, report, "node (ok)")
	assert.Contains(t, report, "docker (MISSING)")
	assert.Contains(t, report, "clean")
	assert.Contains(t, report, "master")
}

func TestFormatInactiveSummary_NoHistory(t *testing.T) {
	t.Parallel()

	result := FormatInactiveSummary("/tmp/myproject", nil, nil)
	assert.Contains(t, result, "No active build found in /tmp/myproject")
	assert.Contains(t, result, "Run 'fry run' to start a build.")
	assert.NotContains(t, result, "Archived")
	assert.NotContains(t, result, "Worktree")
}

func TestFormatInactiveSummary_ArchivesOnly(t *testing.T) {
	t.Parallel()

	archives := []archive.BuildSummary{
		{
			Timestamp:      time.Date(2026, 3, 27, 7, 46, 0, 0, time.Local),
			EpicName:       "My Epic",
			TotalSprints:   3,
			CompletedCount: 2,
			Mode:           "software",
		},
		{
			Timestamp:      time.Date(2026, 3, 26, 12, 0, 0, 0, time.Local),
			EpicName:       "Other Epic",
			TotalSprints:   4,
			CompletedCount: 3,
			FailedCount:    1,
			Mode:           "software",
			ExitReason:     "sprint 4 audit failed",
		},
	}

	result := FormatInactiveSummary("/tmp/proj", archives, nil)
	assert.Contains(t, result, "Archived Builds (2)")
	assert.Contains(t, result, "2026-03-27 07:46")
	assert.Contains(t, result, "My Epic")
	assert.Contains(t, result, "2/3 sprints passed")
	assert.Contains(t, result, "Other Epic")
	assert.Contains(t, result, "3/4 sprints passed, 1 failed")
	assert.Contains(t, result, "Exit: sprint 4 audit failed")
	assert.Contains(t, result, "Run 'fry run' to start a new build.")
	assert.NotContains(t, result, "Worktree")
}

func TestFormatInactiveSummary_WorktreesOnly(t *testing.T) {
	t.Parallel()

	worktrees := []archive.BuildSummary{
		{
			Dir:            ".fry-worktrees/my-slug",
			EpicName:       "WT Epic",
			TotalSprints:   6,
			CompletedCount: 0,
			FailedCount:    1,
			Mode:           "software",
			ExitReason:     "sprint 1 audit failed",
		},
	}

	result := FormatInactiveSummary("/tmp/proj", nil, worktrees)
	assert.Contains(t, result, "Worktree Builds (1)")
	assert.Contains(t, result, ".fry-worktrees/my-slug/")
	assert.Contains(t, result, "WT Epic")
	assert.Contains(t, result, "0/6 sprints passed, 1 failed")
	assert.Contains(t, result, "Exit: sprint 1 audit failed")
	assert.NotContains(t, result, "Archived")
}

func TestFormatInactiveSummary_Both(t *testing.T) {
	t.Parallel()

	archives := []archive.BuildSummary{
		{
			Timestamp:      time.Date(2026, 3, 25, 10, 0, 0, 0, time.Local),
			EpicName:       "Archived",
			TotalSprints:   2,
			CompletedCount: 2,
			Mode:           "planning",
		},
	}
	worktrees := []archive.BuildSummary{
		{
			Dir:          ".fry-worktrees/active",
			EpicName:     "Active WT",
			TotalSprints: 3,
			Mode:         "software",
		},
	}

	result := FormatInactiveSummary("/tmp/proj", archives, worktrees)
	assert.Contains(t, result, "Archived Builds (1)")
	assert.Contains(t, result, "Worktree Builds (1)")
	assert.Contains(t, result, "Archived")
	assert.Contains(t, result, "Active WT")
}

func TestFormatInactiveSummary_ManyArchives(t *testing.T) {
	t.Parallel()

	var archives []archive.BuildSummary
	for i := 15; i >= 1; i-- {
		archives = append(archives, archive.BuildSummary{
			Timestamp:    time.Date(2026, 3, i, 12, 0, 0, 0, time.Local),
			EpicName:     fmt.Sprintf("Build %d", i),
			TotalSprints: 2,
			Mode:         "software",
		})
	}

	result := FormatInactiveSummary("/tmp/proj", archives, nil)
	assert.Contains(t, result, "Archived Builds (15)")
	assert.Contains(t, result, "Build 15") // newest
	assert.Contains(t, result, "Build 6")  // 10th entry
	assert.NotContains(t, result, "Build 5")
	assert.Contains(t, result, "... and 5 more archived builds")
}
