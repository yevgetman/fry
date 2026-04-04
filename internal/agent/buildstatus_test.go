package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestWriteBuildStatus_AtomicWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	status := &BuildStatus{
		Version: 1,
		Build: BuildInfo{
			Epic:         "Test Epic",
			Effort:       "high",
			Engine:       "claude",
			Mode:         "software",
			TotalSprints: 3,
			Status:       "running",
			StartedAt:    time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC),
		},
		Sprints: []SprintStatus{},
	}

	err := WriteBuildStatus(dir, status)
	require.NoError(t, err)

	// Verify file exists at expected path
	path := filepath.Join(dir, config.BuildStatusFile)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Verify no temp file remains
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "temp file should not remain after write")

	// Verify valid JSON
	var parsed BuildStatus
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, 1, parsed.Version)
	assert.Equal(t, "Test Epic", parsed.Build.Epic)
	assert.Equal(t, "high", parsed.Build.Effort)
	assert.Equal(t, "claude", parsed.Build.Engine)
	assert.Equal(t, "software", parsed.Build.Mode)
	assert.Equal(t, 3, parsed.Build.TotalSprints)
	assert.Equal(t, "running", parsed.Build.Status)
	assert.False(t, parsed.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestWriteBuildStatus_Overwrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	status := &BuildStatus{
		Version: 1,
		Build:   BuildInfo{Status: "running", Epic: "First"},
		Sprints: []SprintStatus{},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	status.Build.Status = "completed"
	status.Build.Epic = "Updated"
	require.NoError(t, WriteBuildStatus(dir, status))

	read, err := ReadBuildStatus(dir)
	require.NoError(t, err)
	assert.Equal(t, "completed", read.Build.Status)
	assert.Equal(t, "Updated", read.Build.Epic)
}

func TestReadBuildStatus_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result, err := ReadBuildStatus(dir)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestReadBuildStatus_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path := filepath.Join(dir, config.BuildStatusFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("{invalid"), 0o644))

	_, err := ReadBuildStatus(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse build status")
}

func TestWriteBuildStatus_FullLifecycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	now := time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC)

	// Phase 1: Build start
	status := &BuildStatus{
		Version: 1,
		Build: BuildInfo{
			Epic:         "My Feature",
			Effort:       "high",
			Engine:       "claude",
			Mode:         "software",
			GitBranch:    "fry/my-feature",
			TotalSprints: 3,
			Status:       "running",
			StartedAt:    now,
		},
		Sprints: []SprintStatus{},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	// Phase 2: Sprint start
	sprintStart := now.Add(10 * time.Second)
	status.Build.CurrentSprint = 1
	status.Sprints = append(status.Sprints, SprintStatus{
		Number:    1,
		Name:      "Scaffolding",
		Status:    "running",
		StartedAt: &sprintStart,
	})
	require.NoError(t, WriteBuildStatus(dir, status))

	// Phase 3: Sprint complete with checks and alignment
	sprintEnd := now.Add(60 * time.Second)
	status.Sprints[0].Status = "PASS (aligned)"
	status.Sprints[0].FinishedAt = &sprintEnd
	status.Sprints[0].DurationSec = 50.0
	status.Sprints[0].SanityChecks = &SanityCheckStatus{
		Passed: 3,
		Total:  4,
		Results: []CheckResultEntry{
			{Type: "FILE", Target: "main.go", Passed: true},
			{Type: "CMD", Target: "go build ./...", Passed: true},
			{Type: "TEST", Target: "go test ./...", Passed: true},
			{Type: "FILE_CONTAINS", Target: "go.mod", Passed: false, Output: "pattern not found"},
		},
	}
	status.Sprints[0].Alignment = &AlignmentStatus{
		Attempts: 2,
		Outcome:  "healed",
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	// Phase 4: Audit
	status.Sprints[0].Audit = &AuditStatus{
		Cycles:         2,
		Findings:       map[string]int{"HIGH": 0, "MODERATE": 1, "LOW": 2},
		Outcome:        "advisory",
		CurrentCycle:   2,
		MaxCycles:      5,
		TargetIssues:   1,
		IssueHeadlines: []string{"internal/api/server.go: missing timeout"},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	// Phase 5: Review
	status.Sprints[0].Review = &ReviewStatus{Verdict: "CONTINUE"}
	require.NoError(t, WriteBuildStatus(dir, status))

	// Phase 6: Build audit
	status.BuildAudit = &BuildAuditStatus{
		Ran:      true,
		Passed:   true,
		Blocking: false,
		Findings: map[string]int{"LOW": 1},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	// Phase 7: Build complete
	status.Build.Status = "completed"
	require.NoError(t, WriteBuildStatus(dir, status))

	// Verify final state round-trips correctly
	read, err := ReadBuildStatus(dir)
	require.NoError(t, err)

	assert.Equal(t, 1, read.Version)
	assert.Equal(t, "completed", read.Build.Status)
	assert.Equal(t, "My Feature", read.Build.Epic)
	assert.Equal(t, "fry/my-feature", read.Build.GitBranch)
	assert.Equal(t, 1, read.Build.CurrentSprint)

	require.Len(t, read.Sprints, 1)
	s := read.Sprints[0]
	assert.Equal(t, 1, s.Number)
	assert.Equal(t, "Scaffolding", s.Name)
	assert.Equal(t, "PASS (aligned)", s.Status)
	assert.Equal(t, 50.0, s.DurationSec)

	require.NotNil(t, s.SanityChecks)
	assert.Equal(t, 3, s.SanityChecks.Passed)
	assert.Equal(t, 4, s.SanityChecks.Total)
	assert.Len(t, s.SanityChecks.Results, 4)
	assert.True(t, s.SanityChecks.Results[0].Passed)
	assert.False(t, s.SanityChecks.Results[3].Passed)
	assert.Equal(t, "pattern not found", s.SanityChecks.Results[3].Output)

	require.NotNil(t, s.Alignment)
	assert.Equal(t, 2, s.Alignment.Attempts)
	assert.Equal(t, "healed", s.Alignment.Outcome)

	require.NotNil(t, s.Audit)
	assert.Equal(t, 2, s.Audit.Cycles)
	assert.Equal(t, "advisory", s.Audit.Outcome)
	assert.Equal(t, 1, s.Audit.Findings["MODERATE"])
	assert.Equal(t, 2, s.Audit.CurrentCycle)
	assert.Equal(t, 5, s.Audit.MaxCycles)
	assert.Equal(t, 1, s.Audit.TargetIssues)
	assert.Equal(t, []string{"internal/api/server.go: missing timeout"}, s.Audit.IssueHeadlines)

	require.NotNil(t, s.Review)
	assert.Equal(t, "CONTINUE", s.Review.Verdict)

	require.NotNil(t, read.BuildAudit)
	assert.True(t, read.BuildAudit.Ran)
	assert.True(t, read.BuildAudit.Passed)
	assert.Equal(t, 1, read.BuildAudit.Findings["LOW"])
}

func TestWriteBuildStatus_OmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	status := &BuildStatus{
		Version: 1,
		Build: BuildInfo{
			Epic:         "Minimal",
			TotalSprints: 1,
			Status:       "running",
		},
		Sprints: []SprintStatus{
			{Number: 1, Name: "Solo", Status: "PASS"},
		},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	path := filepath.Join(dir, config.BuildStatusFile)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	raw := string(data)
	// Optional fields with omitempty should not appear
	assert.NotContains(t, raw, "build_audit")
	assert.NotContains(t, raw, "sanity_checks")
	assert.NotContains(t, raw, "alignment")
	assert.NotContains(t, raw, "audit")
	assert.NotContains(t, raw, "review")
	assert.NotContains(t, raw, "git_branch")
}

func TestWriteBuildStatus_CreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// .fry/ does not exist yet
	status := &BuildStatus{
		Version: 1,
		Build:   BuildInfo{Status: "running"},
		Sprints: []SprintStatus{},
	}
	err := WriteBuildStatus(dir, status)
	require.NoError(t, err)

	// Verify the directory was created
	info, err := os.Stat(filepath.Join(dir, ".fry"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestGenerateRunID(t *testing.T) {
	t.Parallel()

	id := GenerateRunID()
	assert.True(t, len(id) > len(config.RunPrefix), "run ID should have a timestamp suffix")
	assert.Equal(t, config.RunPrefix, id[:len(config.RunPrefix)])
}

func TestWriteBuildStatus_PerRunSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	runID := "run-20260402-100000"
	status := &BuildStatus{
		Version: 1,
		Run: &RunMeta{
			RunID:   runID,
			RunType: RunTypeFresh,
		},
		Build: BuildInfo{
			Epic:   "Snapshot Test",
			Status: "running",
		},
		Sprints: []SprintStatus{},
	}

	require.NoError(t, WriteBuildStatus(dir, status))

	// Verify top-level file
	topLevel, err := ReadBuildStatus(dir)
	require.NoError(t, err)
	require.NotNil(t, topLevel)
	assert.Equal(t, "Snapshot Test", topLevel.Build.Epic)
	require.NotNil(t, topLevel.Run)
	assert.Equal(t, runID, topLevel.Run.RunID)

	// Verify per-run snapshot
	runStatus, err := ReadRunStatus(dir, runID)
	require.NoError(t, err)
	require.NotNil(t, runStatus)
	assert.Equal(t, "Snapshot Test", runStatus.Build.Epic)
	assert.Equal(t, runID, runStatus.Run.RunID)
	assert.Equal(t, RunTypeFresh, runStatus.Run.RunType)
}

func TestWriteBuildStatus_RunSnapshotUpdatesInPlace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	runID := "run-20260402-100000"
	status := &BuildStatus{
		Version: 1,
		Run:     &RunMeta{RunID: runID, RunType: RunTypeFresh},
		Build:   BuildInfo{Epic: "Update Test", Status: "running"},
		Sprints: []SprintStatus{},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	// Update status
	status.Build.Status = "completed"
	status.Sprints = append(status.Sprints, SprintStatus{Number: 1, Name: "S1", Status: "PASS"})
	require.NoError(t, WriteBuildStatus(dir, status))

	// Verify per-run snapshot reflects latest update
	runStatus, err := ReadRunStatus(dir, runID)
	require.NoError(t, err)
	assert.Equal(t, "completed", runStatus.Build.Status)
	require.Len(t, runStatus.Sprints, 1)
}

func TestWriteBuildStatus_LaterRunDoesNotEraseEarlier(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// First run completes
	run1 := &BuildStatus{
		Version: 1,
		Run:     &RunMeta{RunID: "run-20260402-100000", RunType: RunTypeFresh},
		Build: BuildInfo{
			Epic:         "My App",
			Status:       "completed",
			TotalSprints: 10,
		},
		Sprints: []SprintStatus{
			{Number: 1, Name: "S1", Status: "PASS"},
			{Number: 2, Name: "S2", Status: "PASS"},
		},
	}
	require.NoError(t, WriteBuildStatus(dir, run1))

	// Second run (retry) overwrites top-level, starts from sprint 1
	run2 := &BuildStatus{
		Version: 1,
		Run:     &RunMeta{RunID: "run-20260403-060000", RunType: RunTypeRetry, ParentRunID: "run-20260402-100000"},
		Build: BuildInfo{
			Epic:         "My App",
			Status:       "failed",
			TotalSprints: 10,
		},
		Sprints: []SprintStatus{
			{Number: 1, Name: "S1", Status: "running"},
		},
	}
	require.NoError(t, WriteBuildStatus(dir, run2))

	// Top-level shows the later run
	topLevel, err := ReadBuildStatus(dir)
	require.NoError(t, err)
	assert.Equal(t, "failed", topLevel.Build.Status)
	assert.Equal(t, "run-20260403-060000", topLevel.Run.RunID)

	// But the first run's snapshot is preserved
	firstRun, err := ReadRunStatus(dir, "run-20260402-100000")
	require.NoError(t, err)
	require.NotNil(t, firstRun, "first run's snapshot must survive the later retry")
	assert.Equal(t, "completed", firstRun.Build.Status)
	assert.Len(t, firstRun.Sprints, 2)
}

func TestReadRunStatus_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result, err := ReadRunStatus(dir, "run-nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestScanRuns_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	runs, err := ScanRuns(dir)
	assert.NoError(t, err)
	assert.Nil(t, runs)
}

func TestScanRuns_MultipleRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create two run snapshots
	for _, r := range []struct {
		id      string
		runType RunType
		status  string
		started time.Time
	}{
		{"run-20260402-100000", RunTypeFresh, "completed", time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)},
		{"run-20260403-060000", RunTypeContinue, "failed", time.Date(2026, 4, 3, 6, 0, 0, 0, time.UTC)},
	} {
		status := &BuildStatus{
			Version: 1,
			Run:     &RunMeta{RunID: r.id, RunType: r.runType},
			Build: BuildInfo{
				Epic:      "Test",
				Status:    r.status,
				StartedAt: r.started,
			},
			Sprints: []SprintStatus{{Number: 1, Name: "S1", Status: "PASS"}},
		}
		runDir := filepath.Join(dir, config.RunsDir, r.id)
		require.NoError(t, os.MkdirAll(runDir, 0o755))
		data, err := json.MarshalIndent(status, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(runDir, "build-status.json"), data, 0o644))
	}

	runs, err := ScanRuns(dir)
	require.NoError(t, err)
	require.Len(t, runs, 2)

	// Sorted newest-first
	assert.Equal(t, "run-20260403-060000", runs[0].RunID)
	assert.Equal(t, RunTypeContinue, runs[0].RunType)
	assert.Equal(t, "failed", runs[0].Status)

	assert.Equal(t, "run-20260402-100000", runs[1].RunID)
	assert.Equal(t, RunTypeFresh, runs[1].RunType)
	assert.Equal(t, "completed", runs[1].Status)
	assert.Equal(t, 1, runs[1].Sprints)
}

func TestScanRuns_SkipsNonRunDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	runsRoot := filepath.Join(dir, config.RunsDir)
	require.NoError(t, os.MkdirAll(runsRoot, 0o755))

	// Create a non-run directory
	require.NoError(t, os.MkdirAll(filepath.Join(runsRoot, "not-a-run"), 0o755))
	// Create a file
	require.NoError(t, os.WriteFile(filepath.Join(runsRoot, "stray-file.txt"), []byte("hi"), 0o644))

	runs, err := ScanRuns(dir)
	require.NoError(t, err)
	assert.Empty(t, runs)
}

func TestLatestRunID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// No status file yet
	assert.Equal(t, "", LatestRunID(dir))

	// Status without RunMeta
	status := &BuildStatus{
		Version: 1,
		Build:   BuildInfo{Status: "running"},
		Sprints: []SprintStatus{},
	}
	require.NoError(t, WriteBuildStatus(dir, status))
	assert.Equal(t, "", LatestRunID(dir))

	// Status with RunMeta
	status.Run = &RunMeta{RunID: "run-20260402-100000", RunType: RunTypeFresh}
	require.NoError(t, WriteBuildStatus(dir, status))
	assert.Equal(t, "run-20260402-100000", LatestRunID(dir))
}

func TestRunMeta_ContinueLineage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Fresh run
	status := &BuildStatus{
		Version: 1,
		Run:     &RunMeta{RunID: "run-20260401-100000", RunType: RunTypeFresh},
		Build:   BuildInfo{Epic: "Lineage Test", Status: "completed"},
		Sprints: []SprintStatus{},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	// Continue run references the parent
	status.Run = &RunMeta{
		RunID:       "run-20260401-120000",
		RunType:     RunTypeContinue,
		ParentRunID: "run-20260401-100000",
	}
	status.Build.Status = "running"
	require.NoError(t, WriteBuildStatus(dir, status))

	// Verify lineage in the continue run's snapshot
	continueStatus, err := ReadRunStatus(dir, "run-20260401-120000")
	require.NoError(t, err)
	assert.Equal(t, RunTypeContinue, continueStatus.Run.RunType)
	assert.Equal(t, "run-20260401-100000", continueStatus.Run.ParentRunID)

	// Verify original run is still accessible
	freshStatus, err := ReadRunStatus(dir, "run-20260401-100000")
	require.NoError(t, err)
	assert.Equal(t, "completed", freshStatus.Build.Status)
}

func TestWriteBuildStatus_NoRunMeta_SkipsSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	status := &BuildStatus{
		Version: 1,
		Build:   BuildInfo{Status: "running"},
		Sprints: []SprintStatus{},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	// Runs directory should not be created
	_, err := os.Stat(filepath.Join(dir, config.RunsDir))
	assert.True(t, os.IsNotExist(err), "runs dir should not exist when no RunMeta")
}

func TestBuildStatus_ReportingFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	status := &BuildStatus{
		Version: 1,
		Build: BuildInfo{
			Epic:   "Reporting Test",
			Status: "completed_with_reporting_failure",
			Phase:  "complete",
		},
		Sprints: []SprintStatus{
			{Number: 1, Name: "S1", Status: "PASS"},
		},
		ReportingFailure: &ReportingFailure{
			Stage:   "build_audit",
			Message: "quota exceeded",
		},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	read, err := ReadBuildStatus(dir)
	require.NoError(t, err)
	assert.Equal(t, "completed_with_reporting_failure", read.Build.Status)
	require.NotNil(t, read.ReportingFailure)
	assert.Equal(t, "build_audit", read.ReportingFailure.Stage)
	assert.Equal(t, "quota exceeded", read.ReportingFailure.Message)
}

func TestBuildStatus_ReportingFailure_BothStages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	status := &BuildStatus{
		Version: 1,
		Build: BuildInfo{
			Epic:   "Both Failures",
			Status: "completed_with_reporting_failure",
			Phase:  "complete",
		},
		Sprints: []SprintStatus{},
		ReportingFailure: &ReportingFailure{
			Stage:   "build_audit+summary",
			Message: "audit: quota exceeded; summary: quota exceeded",
		},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	read, err := ReadBuildStatus(dir)
	require.NoError(t, err)
	assert.Equal(t, "build_audit+summary", read.ReportingFailure.Stage)
	assert.Contains(t, read.ReportingFailure.Message, "audit:")
	assert.Contains(t, read.ReportingFailure.Message, "summary:")
}

func TestBuildStatus_NoReportingFailure_OmittedFromJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	status := &BuildStatus{
		Version: 1,
		Build:   BuildInfo{Status: "completed"},
		Sprints: []SprintStatus{},
	}
	require.NoError(t, WriteBuildStatus(dir, status))

	data, err := os.ReadFile(filepath.Join(dir, config.BuildStatusFile))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "reporting_failure")
}

func TestWriteRollingResults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	results := []RollingSprintResult{
		{Number: 1, Name: "Setup", Status: "PASS", DurationSec: 42.5},
		{Number: 2, Name: "Core", Status: "PASS (aligned)", DurationSec: 120.0},
	}
	require.NoError(t, WriteRollingResults(dir, results))

	read, err := ReadRollingResults(dir)
	require.NoError(t, err)
	require.Len(t, read, 2)
	assert.Equal(t, 1, read[0].Number)
	assert.Equal(t, "Setup", read[0].Name)
	assert.Equal(t, "PASS", read[0].Status)
	assert.Equal(t, 42.5, read[0].DurationSec)
	assert.Equal(t, "PASS (aligned)", read[1].Status)
}

func TestReadRollingResults_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	results, err := ReadRollingResults(dir)
	assert.NoError(t, err)
	assert.Nil(t, results)
}

func TestWriteRollingResults_Overwrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, WriteRollingResults(dir, []RollingSprintResult{
		{Number: 1, Name: "S1", Status: "PASS"},
	}))
	require.NoError(t, WriteRollingResults(dir, []RollingSprintResult{
		{Number: 1, Name: "S1", Status: "PASS"},
		{Number: 2, Name: "S2", Status: "FAIL"},
	}))

	read, err := ReadRollingResults(dir)
	require.NoError(t, err)
	require.Len(t, read, 2)
	assert.Equal(t, "FAIL", read[1].Status)
}
