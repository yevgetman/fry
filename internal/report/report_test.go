package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteCreatesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "build-report.json")

	start := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 21, 10, 5, 0, 0, time.UTC)

	r := BuildReport{
		EpicName:  "Test Epic",
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
		Sprints: []SprintResult{
			{
				SprintNum: 1,
				Name:      "Setup",
				StartTime: start,
				EndTime:   end,
				Passed:    true,
			},
		},
	}

	err := Write(path, r)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, json.Valid(data), "output must be valid JSON")
}

func TestWriteUnmarshalsCorrectly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "build-report.json")

	start := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 21, 10, 5, 30, 0, time.UTC)

	original := BuildReport{
		EpicName:  "My Epic",
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
		Sprints: []SprintResult{
			{
				SprintNum:    1,
				Name:         "Sprint One",
				StartTime:    start,
				EndTime:      end,
				Passed:       true,
				HealAttempts: 2,
				Verification: &VerificationResult{
					TotalChecks:  5,
					PassedChecks: 4,
					FailedChecks: 1,
				},
			},
		},
	}

	require.NoError(t, Write(path, original))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var decoded BuildReport
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "My Epic", decoded.EpicName)
	require.Len(t, decoded.Sprints, 1)
	assert.Equal(t, 1, decoded.Sprints[0].SprintNum)
	assert.Equal(t, "Sprint One", decoded.Sprints[0].Name)
	assert.True(t, decoded.Sprints[0].Passed)
	assert.Equal(t, 2, decoded.Sprints[0].HealAttempts)
	require.NotNil(t, decoded.Sprints[0].Verification)
	assert.Equal(t, 5, decoded.Sprints[0].Verification.TotalChecks)
	assert.Equal(t, 4, decoded.Sprints[0].Verification.PassedChecks)
	assert.Equal(t, 1, decoded.Sprints[0].Verification.FailedChecks)
}

func TestWriteErrorOnUnwritablePath(t *testing.T) {
	t.Parallel()

	// Use a path where the directory is a file (cannot create as directory)
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blockingFile, []byte("x"), 0o644))

	path := filepath.Join(blockingFile, "build-report.json")
	err := Write(path, BuildReport{})
	assert.Error(t, err)
}

func TestWriteFilePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "build-report.json")

	require.NoError(t, Write(path, BuildReport{EpicName: "Perms Test"}))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestWriteAtomicRename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "build-report.json")

	r := BuildReport{EpicName: "Atomic Test"}
	require.NoError(t, Write(path, r))

	// Final file must exist and be valid JSON.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, json.Valid(data))

	// No temp files should remain in the directory.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".build-report-"), "unexpected temp file left: %s", e.Name())
	}
}

func TestWriteZeroReport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "build-report.json")

	err := Write(path, BuildReport{})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var decoded BuildReport
	require.NoError(t, json.Unmarshal(data, &decoded))
}
