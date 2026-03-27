package git

import (
	"context"
	"errors"
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
	require.NoError(t, GitCheckpoint(context.Background(), projectDir, "Epic Name", 2, "Add login page", "complete"))

	cmd := exec.Command("bash", "-c", "git log -1 --pretty=%s")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "Epic Name — Add login page: Sprint 2 complete [automated]", strings.TrimSpace(string(output)))
}

func TestGitCheckpoint_EmptySprintName(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), projectDir))
	require.NoError(t, os.WriteFile(projectDir+"/file.txt", []byte("data\n"), 0o644))
	require.NoError(t, GitCheckpoint(context.Background(), projectDir, "Epic Name", 3, "", "build-audit"))

	cmd := exec.Command("bash", "-c", "git log -1 --pretty=%s")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "Epic Name: Sprint 3 build-audit [automated]", strings.TrimSpace(string(output)))
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
	require.NoError(t, CommitPartialWork(context.Background(), projectDir, "TestEpic", 3, "Fix auth (#42)"))

	cmd := exec.Command("bash", "-c", "git log -1 --pretty=%s")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "TestEpic — Fix auth (#42): Sprint 3 failed-partial [automated]", strings.TrimSpace(string(output)))
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
	nameVal, err := gitConfigValue(context.Background(), projectDir, "user.name")
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(nameVal))

	emailVal, err := gitConfigValue(context.Background(), projectDir, "user.email")
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(emailVal))
}

func TestEnsureLocalIdentity_PreservesExisting(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	cmd := exec.Command("bash", "-c", "git init && git config user.name 'Custom User' && git config user.email 'custom@test.com'")
	cmd.Dir = projectDir
	require.NoError(t, cmd.Run())

	require.NoError(t, ensureLocalIdentity(context.Background(), projectDir))

	name, err := gitConfigValue(context.Background(), projectDir, "user.name")
	require.NoError(t, err)
	assert.Equal(t, "Custom User", strings.TrimSpace(name))

	email, err := gitConfigValue(context.Background(), projectDir, "user.email")
	require.NoError(t, err)
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

	val, err := gitConfigValue(context.Background(), projectDir, "nonexistent.key")
	require.NoError(t, err)
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
	require.NoError(t, GitCheckpoint(context.Background(), projectDir, "Epic's \"Name\" (v2)", 1, "Sprint One", "complete"))

	cmd := exec.Command("bash", "-c", "git log -1 --pretty=%s")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, strings.TrimSpace(string(output)), "Epic's")
}

// mockExecutor is a test double for the Executor interface.
// Set only the function fields you need; unset fields return zero values and nil errors.
type mockExecutor struct {
	IsRepoFn            func(ctx context.Context, dir string) bool
	HasHeadFn           func(ctx context.Context, dir string) bool
	CurrentBranchFn     func(ctx context.Context, dir string) string
	BranchExistsFn      func(ctx context.Context, dir string, name string) bool
	InitFn              func(ctx context.Context, dir string) error
	ConfigGetFn         func(ctx context.Context, dir string, key string) (string, error)
	ConfigSetFn         func(ctx context.Context, dir string, key, value string) error
	AddAllFn            func(ctx context.Context, dir string) error
	AddIntentFn         func(ctx context.Context, dir string, paths []string) error
	CommitAllowEmptyFn  func(ctx context.Context, dir string, message string) error
	ResetFn             func(ctx context.Context, dir string, paths []string) error
	CheckoutFn          func(ctx context.Context, dir string, name string) error
	CreateAndCheckoutFn func(ctx context.Context, dir string, name string) error
	DiffHeadFn          func(ctx context.Context, dir string, excludePathspecs []string) (string, error)
	DiffStatFn          func(ctx context.Context, dir string, excludePathspecs []string) (string, error)
	ListUntrackedFn     func(ctx context.Context, dir string, excludePathspecs []string) ([]string, error)
	StatusPorcelainFn   func(ctx context.Context, dir string) (string, error)
	LogGrepFn           func(ctx context.Context, dir string, grepPattern string, maxCount int, format string) (string, error)
	WorktreeListFn      func(ctx context.Context, dir string) ([]string, error)
	WorktreeAddFn       func(ctx context.Context, dir string, worktreePath, branchName string, createBranch bool) error
	WorktreePruneFn     func(ctx context.Context, dir string) error
}

func (m *mockExecutor) IsRepo(ctx context.Context, dir string) bool {
	if m.IsRepoFn != nil {
		return m.IsRepoFn(ctx, dir)
	}
	return false
}
func (m *mockExecutor) HasHead(ctx context.Context, dir string) bool {
	if m.HasHeadFn != nil {
		return m.HasHeadFn(ctx, dir)
	}
	return false
}
func (m *mockExecutor) CurrentBranch(ctx context.Context, dir string) string {
	if m.CurrentBranchFn != nil {
		return m.CurrentBranchFn(ctx, dir)
	}
	return ""
}
func (m *mockExecutor) BranchExists(ctx context.Context, dir string, name string) bool {
	if m.BranchExistsFn != nil {
		return m.BranchExistsFn(ctx, dir, name)
	}
	return false
}
func (m *mockExecutor) Init(ctx context.Context, dir string) error {
	if m.InitFn != nil {
		return m.InitFn(ctx, dir)
	}
	return nil
}
func (m *mockExecutor) ConfigGet(ctx context.Context, dir string, key string) (string, error) {
	if m.ConfigGetFn != nil {
		return m.ConfigGetFn(ctx, dir, key)
	}
	return "", nil
}
func (m *mockExecutor) ConfigSet(ctx context.Context, dir string, key, value string) error {
	if m.ConfigSetFn != nil {
		return m.ConfigSetFn(ctx, dir, key, value)
	}
	return nil
}
func (m *mockExecutor) AddAll(ctx context.Context, dir string) error {
	if m.AddAllFn != nil {
		return m.AddAllFn(ctx, dir)
	}
	return nil
}
func (m *mockExecutor) AddIntent(ctx context.Context, dir string, paths []string) error {
	if m.AddIntentFn != nil {
		return m.AddIntentFn(ctx, dir, paths)
	}
	return nil
}
func (m *mockExecutor) CommitAllowEmpty(ctx context.Context, dir string, message string) error {
	if m.CommitAllowEmptyFn != nil {
		return m.CommitAllowEmptyFn(ctx, dir, message)
	}
	return nil
}
func (m *mockExecutor) Reset(ctx context.Context, dir string, paths []string) error {
	if m.ResetFn != nil {
		return m.ResetFn(ctx, dir, paths)
	}
	return nil
}
func (m *mockExecutor) Checkout(ctx context.Context, dir string, name string) error {
	if m.CheckoutFn != nil {
		return m.CheckoutFn(ctx, dir, name)
	}
	return nil
}
func (m *mockExecutor) CreateAndCheckout(ctx context.Context, dir string, name string) error {
	if m.CreateAndCheckoutFn != nil {
		return m.CreateAndCheckoutFn(ctx, dir, name)
	}
	return nil
}
func (m *mockExecutor) DiffHead(ctx context.Context, dir string, excludePathspecs []string) (string, error) {
	if m.DiffHeadFn != nil {
		return m.DiffHeadFn(ctx, dir, excludePathspecs)
	}
	return "", nil
}
func (m *mockExecutor) DiffStat(ctx context.Context, dir string, excludePathspecs []string) (string, error) {
	if m.DiffStatFn != nil {
		return m.DiffStatFn(ctx, dir, excludePathspecs)
	}
	return "", nil
}
func (m *mockExecutor) ListUntracked(ctx context.Context, dir string, excludePathspecs []string) ([]string, error) {
	if m.ListUntrackedFn != nil {
		return m.ListUntrackedFn(ctx, dir, excludePathspecs)
	}
	return nil, nil
}
func (m *mockExecutor) StatusPorcelain(ctx context.Context, dir string) (string, error) {
	if m.StatusPorcelainFn != nil {
		return m.StatusPorcelainFn(ctx, dir)
	}
	return "", nil
}
func (m *mockExecutor) LogGrep(ctx context.Context, dir string, grepPattern string, maxCount int, format string) (string, error) {
	if m.LogGrepFn != nil {
		return m.LogGrepFn(ctx, dir, grepPattern, maxCount, format)
	}
	return "", nil
}
func (m *mockExecutor) WorktreeList(ctx context.Context, dir string) ([]string, error) {
	if m.WorktreeListFn != nil {
		return m.WorktreeListFn(ctx, dir)
	}
	return nil, nil
}
func (m *mockExecutor) WorktreeAdd(ctx context.Context, dir string, worktreePath, branchName string, createBranch bool) error {
	if m.WorktreeAddFn != nil {
		return m.WorktreeAddFn(ctx, dir, worktreePath, branchName, createBranch)
	}
	return nil
}
func (m *mockExecutor) WorktreePrune(ctx context.Context, dir string) error {
	if m.WorktreePruneFn != nil {
		return m.WorktreePruneFn(ctx, dir)
	}
	return nil
}

// #52: ConfigGet error propagation tests

func TestEnsureLocalIdentityWith_ConfigGetError(t *testing.T) {
	t.Parallel()

	configErr := errors.New("git config failed")
	ex := &mockExecutor{
		ConfigGetFn: func(_ context.Context, _ string, _ string) (string, error) {
			return "", configErr
		},
	}
	err := ensureLocalIdentityWith(context.Background(), t.TempDir(), ex)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get git user.name")
	assert.ErrorIs(t, err, configErr)
}

func TestEnsureLocalIdentityWith_ConfigGetEmailError(t *testing.T) {
	t.Parallel()

	callCount := 0
	configErr := errors.New("git config email failed")
	ex := &mockExecutor{
		ConfigGetFn: func(_ context.Context, _ string, key string) (string, error) {
			callCount++
			if callCount == 1 {
				return "existing-name", nil
			}
			return "", configErr
		},
	}
	err := ensureLocalIdentityWith(context.Background(), t.TempDir(), ex)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get git user.email")
	assert.ErrorIs(t, err, configErr)
}

func TestMockConfigGet_KeyNotFoundMapping(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		ConfigGetFn: func(_ context.Context, _ string, _ string) (string, error) {
			return "", nil // simulates exit code 1 mapped to ("", nil)
		},
	}
	val, err := ex.ConfigGet(context.Background(), t.TempDir(), "nonexistent.key")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

// #31: DiffStatForNoopDetectionWith tests

func TestDiffStatForNoopDetectionWith_Success(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		DiffStatFn: func(_ context.Context, _ string, _ []string) (string, error) {
			return "file.go | 3 +++", nil
		},
	}
	result := DiffStatForNoopDetectionWith(context.Background(), t.TempDir(), ex)
	assert.Equal(t, "file.go | 3 +++", result)
}

func TestDiffStatForNoopDetectionWith_DiffStatError(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		DiffStatFn: func(_ context.Context, _ string, _ []string) (string, error) {
			return "", errors.New("git error")
		},
	}
	result := DiffStatForNoopDetectionWith(context.Background(), t.TempDir(), ex)
	assert.True(t, strings.HasPrefix(result, "__git_error_"), "expected __git_error_ prefix, got %q", result)
}

func TestDiffStatForNoopDetectionWith_EmptyDiff(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		DiffStatFn: func(_ context.Context, _ string, _ []string) (string, error) {
			return "", nil
		},
	}
	result := DiffStatForNoopDetectionWith(context.Background(), t.TempDir(), ex)
	assert.Equal(t, "", result)
}

// #31: CollectStateWith tests

func TestCollectStateWith_AllSucceed(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		StatusPorcelainFn: func(_ context.Context, _ string) (string, error) {
			return "", nil
		},
		CurrentBranchFn: func(_ context.Context, _ string) string {
			return "main"
		},
		LogGrepFn: func(_ context.Context, _ string, _ string, _ int, _ string) (string, error) {
			return "Epic: Sprint 3 complete [automated]", nil
		},
	}
	clean, branch, lastCommit := CollectStateWith(context.Background(), t.TempDir(), ex)
	assert.True(t, clean)
	assert.Equal(t, "main", branch)
	assert.Contains(t, lastCommit, "Sprint 3")
}

func TestCollectStateWith_StatusPorcelainFails(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		StatusPorcelainFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("porcelain error")
		},
	}
	clean, _, _ := CollectStateWith(context.Background(), t.TempDir(), ex)
	assert.True(t, clean)
}

func TestCollectStateWith_LogGrepFails(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		StatusPorcelainFn: func(_ context.Context, _ string) (string, error) {
			return "", nil
		},
		CurrentBranchFn: func(_ context.Context, _ string) string {
			return "main"
		},
		LogGrepFn: func(_ context.Context, _ string, _ string, _ int, _ string) (string, error) {
			return "", errors.New("log error")
		},
	}
	_, _, lastCommit := CollectStateWith(context.Background(), t.TempDir(), ex)
	assert.Equal(t, "", lastCommit)
}
