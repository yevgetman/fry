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
		Cycles:   2,
		Findings: map[string]int{"HIGH": 0, "MODERATE": 1, "LOW": 2},
		Outcome:  "advisory",
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
