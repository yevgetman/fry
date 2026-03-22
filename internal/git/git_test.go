package git

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitGit(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))

	info, err := os.Stat(projectDir + "/.git")
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestGitCheckpoint(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))
	require.NoError(t, os.WriteFile(projectDir+"/file.txt", []byte("data\n"), 0o644))
	require.NoError(t, GitCheckpoint(context.Background(), projectDir, "Epic Name", 2, "complete"))

	cmd := exec.Command("bash", "-c", "git log -1 --pretty=%s")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "Epic Name: Sprint 2 complete [automated]", strings.TrimSpace(string(output)))
}

func TestInitGitIdempotent(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))
	require.NoError(t, InitGit(context.Background(), projectDir))
}

func TestGitDiffForAudit(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))

	require.NoError(t, os.WriteFile(projectDir+"/existing.txt", []byte("original\n"), 0o644))
	cmd := exec.Command("bash", "-c", "git add -A && git commit -m 'add existing'")
	cmd.Dir = projectDir
	require.NoError(t, cmd.Run())

	require.NoError(t, os.WriteFile(projectDir+"/existing.txt", []byte("modified\n"), 0o644))
	require.NoError(t, os.WriteFile(projectDir+"/newfile.txt", []byte("new content\n"), 0o644))

	diff, err := GitDiffForAudit(context.Background(), projectDir)
	require.NoError(t, err)

	assert.Contains(t, diff, "existing.txt")
	assert.Contains(t, diff, "modified")
	assert.Contains(t, diff, "newfile.txt")
	assert.Contains(t, diff, "new content")

	statusCmd := exec.Command("bash", "-c", "git diff --cached --name-only")
	statusCmd.Dir = projectDir
	out, err := statusCmd.Output()
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(out)), "no files should be staged after GitDiffForAudit")
}

func TestGitDiffForAudit_PreservesExistingStagedChanges(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))

	require.NoError(t, os.WriteFile(projectDir+"/tracked.txt", []byte("original\n"), 0o644))
	cmd := exec.Command("bash", "-c", "git add tracked.txt && git commit -m 'add tracked'")
	cmd.Dir = projectDir
	require.NoError(t, cmd.Run())

	require.NoError(t, os.WriteFile(projectDir+"/tracked.txt", []byte("staged change\n"), 0o644))
	stageCmd := exec.Command("bash", "-c", "git add tracked.txt")
	stageCmd.Dir = projectDir
	require.NoError(t, stageCmd.Run())

	require.NoError(t, os.WriteFile(projectDir+"/newfile.txt", []byte("new content\n"), 0o644))

	diff, err := GitDiffForAudit(context.Background(), projectDir)
	require.NoError(t, err)
	assert.Contains(t, diff, "tracked.txt")
	assert.Contains(t, diff, "newfile.txt")

	statusCmd := exec.Command("bash", "-c", "git diff --cached --name-only")
	statusCmd.Dir = projectDir
	out, err := statusCmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "tracked.txt", strings.TrimSpace(string(out)))
}

func TestGitDiffForAuditExcludesFryDir(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))

	require.NoError(t, os.MkdirAll(projectDir+"/.fry", 0o755))
	require.NoError(t, os.WriteFile(projectDir+"/.fry/sprint-progress.txt", []byte("progress\n"), 0o644))
	require.NoError(t, os.WriteFile(projectDir+"/code.go", []byte("package main\n"), 0o644))

	diff, err := GitDiffForAudit(context.Background(), projectDir)
	require.NoError(t, err)

	assert.Contains(t, diff, "code.go")
	assert.NotContains(t, diff, "sprint-progress.txt")
}

// P1: CommitPartialWork

func TestCommitPartialWork(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))
	require.NoError(t, os.WriteFile(projectDir+"/partial.txt", []byte("wip\n"), 0o644))
	require.NoError(t, CommitPartialWork(context.Background(), projectDir, "TestEpic", 3))

	cmd := exec.Command("bash", "-c", "git log -1 --pretty=%s")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "TestEpic: Sprint 3 failed-partial [automated]", strings.TrimSpace(string(output)))
}

// P1: ensureLocalIdentity

func TestEnsureLocalIdentity(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	initCmd := exec.Command("bash", "-c", "git init")
	initCmd.Dir = projectDir
	require.NoError(t, initCmd.Run())

	// ensureLocalIdentity should not error regardless of whether
	// global git config provides user.name/user.email or not.
	require.NoError(t, ensureLocalIdentity(context.Background(), projectDir))

	// After calling ensureLocalIdentity, at least one of local or global
	// should provide user.name — verify git agrees.
	nameVal := gitConfigValue(context.Background(), projectDir, "user.name")
	assert.NotEmpty(t, strings.TrimSpace(nameVal))

	emailVal := gitConfigValue(context.Background(), projectDir, "user.email")
	assert.NotEmpty(t, strings.TrimSpace(emailVal))
}

func TestEnsureLocalIdentity_PreservesExisting(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	cmd := exec.Command("bash", "-c", "git init && git config user.name 'Custom User' && git config user.email 'custom@test.com'")
	cmd.Dir = projectDir
	require.NoError(t, cmd.Run())

	require.NoError(t, ensureLocalIdentity(context.Background(), projectDir))

	name := gitConfigValue(context.Background(), projectDir, "user.name")
	assert.Equal(t, "Custom User", strings.TrimSpace(name))

	email := gitConfigValue(context.Background(), projectDir, "user.email")
	assert.Equal(t, "custom@test.com", strings.TrimSpace(email))
}

// P1: ensureGitignoreEntries

func TestEnsureGitignoreEntries(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, ensureGitignoreEntries(projectDir, []string{".fry/", ".env"}))

	data, err := os.ReadFile(projectDir + "/.gitignore")
	require.NoError(t, err)
	assert.Contains(t, string(data), ".fry/")
	assert.Contains(t, string(data), ".env")
}

func TestEnsureGitignoreEntries_Idempotent(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(projectDir+"/.gitignore", []byte(".fry/\n"), 0o644))

	require.NoError(t, ensureGitignoreEntries(projectDir, []string{".fry/", ".env"}))

	data, err := os.ReadFile(projectDir + "/.gitignore")
	require.NoError(t, err)
	content := string(data)
	assert.Equal(t, 1, strings.Count(content, ".fry/"))
	assert.Contains(t, content, ".env")
}

// P1: GitDiffForAudit with no changes

func TestGitDiffForAudit_NoChanges(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))

	diff, err := GitDiffForAudit(context.Background(), projectDir)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(diff))
}

// P1: gitConfigValue for missing key

func TestGitConfigValue_MissingKey(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	cmd := exec.Command("bash", "-c", "git init")
	cmd.Dir = projectDir
	require.NoError(t, cmd.Run())

	val := gitConfigValue(context.Background(), projectDir, "nonexistent.key")
	assert.Empty(t, strings.TrimSpace(val))
}

// P1: hasHead

func TestHasHead_EmptyRepo(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	cmd := exec.Command("bash", "-c", "git init")
	cmd.Dir = projectDir
	require.NoError(t, cmd.Run())

	assert.False(t, hasHead(context.Background(), projectDir))
}

func TestHasHead_AfterCommit(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))

	assert.True(t, hasHead(context.Background(), projectDir))
}

// P1: GitCheckpoint with special characters in epic name

func TestGitCheckpoint_SpecialChars(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))
	require.NoError(t, os.WriteFile(projectDir+"/file.txt", []byte("data\n"), 0o644))
	require.NoError(t, GitCheckpoint(context.Background(), projectDir, "Epic's \"Name\" (v2)", 1, "complete"))

	cmd := exec.Command("bash", "-c", "git log -1 --pretty=%s")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, strings.TrimSpace(string(output)), "Epic's")
}
