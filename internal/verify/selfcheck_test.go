package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateHarness_AllValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a subdirectory for the file target's parent
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))

	checks := []Check{
		{Sprint: 1, Type: CheckFile, Path: "src/main.go"},
		{Sprint: 1, Type: CheckCmd, Command: "go build ./..."},
	}
	result := ValidateHarness(dir, checks)
	assert.False(t, result.HasIssues())
}

func TestValidateHarness_AbsolutePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	checks := []Check{
		{Sprint: 1, Type: CheckFile, Path: "/usr/local/bin/something"},
	}
	result := ValidateHarness(dir, checks)
	require.True(t, result.HasIssues())
	assert.Equal(t, "path_mismatch", result.Issues[0].Type)
	assert.Contains(t, result.Issues[0].Message, "absolute path")
}

func TestValidateHarness_PathTraversal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	checks := []Check{
		{Sprint: 1, Type: CheckFile, Path: "../../etc/passwd"},
	}
	result := ValidateHarness(dir, checks)
	require.True(t, result.HasIssues())
	assert.Equal(t, "path_mismatch", result.Issues[0].Type)
	assert.Contains(t, result.Issues[0].Message, "traverses outside")
}

func TestValidateHarness_MissingParentDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	checks := []Check{
		{Sprint: 2, Type: CheckFile, Path: "nonexistent/subdir/file.go"},
	}
	result := ValidateHarness(dir, checks)
	require.True(t, result.HasIssues())
	assert.Equal(t, "missing_parent_dir", result.Issues[0].Type)
	assert.Equal(t, 2, result.Issues[0].Sprint)
}

func TestValidateHarness_BareFilename_NoIssue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Bare filenames (no directory component) should not flag missing_parent_dir
	// because the parent is the project root which always exists.
	checks := []Check{
		{Sprint: 1, Type: CheckFile, Path: "go.mod"},
	}
	result := ValidateHarness(dir, checks)
	assert.False(t, result.HasIssues())
}

func TestValidateHarness_EmptyFilePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	checks := []Check{
		{Sprint: 1, Type: CheckFile, Path: ""},
	}
	result := ValidateHarness(dir, checks)
	require.True(t, result.HasIssues())
	assert.Contains(t, result.Issues[0].Message, "empty file target")
}

func TestValidateHarness_EmptyCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	checks := []Check{
		{Sprint: 1, Type: CheckCmd, Command: ""},
	}
	result := ValidateHarness(dir, checks)
	require.True(t, result.HasIssues())
	assert.Contains(t, result.Issues[0].Message, "empty command")
}

func TestValidateHarness_FileContainsCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o755))

	checks := []Check{
		{Sprint: 1, Type: CheckFileContains, Path: "config/app.yaml", Pattern: "port: 8080"},
	}
	result := ValidateHarness(dir, checks)
	assert.False(t, result.HasIssues())
}

func TestValidateHarness_MultipleIssues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	checks := []Check{
		{Sprint: 1, Type: CheckFile, Path: "/absolute/bad"},
		{Sprint: 2, Type: CheckFile, Path: "missing/dir/file.go"},
		{Sprint: 3, Type: CheckCmd, Command: ""},
	}
	result := ValidateHarness(dir, checks)
	assert.Len(t, result.Issues, 3)
}

func TestHarnessCheckResult_Summary(t *testing.T) {
	t.Parallel()

	result := &HarnessCheckResult{}
	assert.Equal(t, "harness self-check passed", result.Summary())

	result.Issues = []HarnessIssue{
		{Sprint: 1, Type: "path_mismatch", Target: "/foo", Message: "absolute path"},
	}
	s := result.Summary()
	assert.Contains(t, s, "sprint 1")
	assert.Contains(t, s, "absolute path")
}

func TestValidateHarness_NilChecks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result := ValidateHarness(dir, nil)
	assert.False(t, result.HasIssues())
}
