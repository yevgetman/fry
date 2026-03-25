package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestParseGitStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    GitStrategy
		wantErr bool
	}{
		{"", StrategyAuto, false},
		{"auto", StrategyAuto, false},
		{"current", StrategyCurrent, false},
		{"branch", StrategyBranch, false},
		{"worktree", StrategyWorktree, false},
		{"invalid", "", true},
		{"BRANCH", "", true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseGitStrategy(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGenerateBranchName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		epicName string
		want     string
	}{
		{"simple", "My Epic", "fry/my-epic"},
		{"special chars", "Epic's \"Name\" (v2)!", "fry/epic-s-name-v2"},
		{"empty", "", "fry/build"},
		{"long name", strings.Repeat("a", 100), "fry/" + strings.Repeat("a", 50)},
		{"numbers", "Sprint 42 Build", "fry/sprint-42-build"},
		{"hyphens collapse", "foo---bar", "fry/foo-bar"},
		{"leading trailing special", "  --hello--  ", "fry/hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GenerateBranchName(tt.epicName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveAutoStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		complexity string
		want       GitStrategy
	}{
		{"SIMPLE", StrategyBranch},
		{"MODERATE", StrategyBranch},
		{"COMPLEX", StrategyWorktree},
		{"complex", StrategyWorktree},
		{"", StrategyBranch},
	}

	for _, tt := range tests {
		t.Run(tt.complexity, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ResolveAutoStrategy(tt.complexity))
		})
	}
}

func TestIsInsideGitRepo(t *testing.T) {
	t.Parallel()

	t.Run("inside repo", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, InitGit(context.Background(), dir))
		assert.True(t, IsInsideGitRepo(context.Background(), dir))
	})

	t.Run("outside repo", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		assert.False(t, IsInsideGitRepo(context.Background(), dir))
	})

	t.Run("subdirectory of repo", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, InitGit(context.Background(), dir))
		sub := filepath.Join(dir, "subdir")
		require.NoError(t, os.MkdirAll(sub, 0o755))
		assert.True(t, IsInsideGitRepo(context.Background(), sub))
	})
}

func TestCurrentBranch(t *testing.T) {
	t.Parallel()

	t.Run("default branch", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, InitGit(context.Background(), dir))
		branch := CurrentBranch(context.Background(), dir)
		assert.NotEmpty(t, branch)
	})

	t.Run("not a repo", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		assert.Empty(t, CurrentBranch(context.Background(), dir))
	})
}

func TestSetupStrategy_Current(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setup, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyCurrent,
	})
	require.NoError(t, err)
	assert.Equal(t, dir, setup.WorkDir)
	assert.Equal(t, dir, setup.OriginalDir)
	assert.Equal(t, StrategyCurrent, setup.Strategy)
	assert.Empty(t, setup.BranchName)
	assert.False(t, setup.IsWorktree)
}

func TestSetupStrategy_Auto_Errors(t *testing.T) {
	t.Parallel()

	_, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: t.TempDir(),
		Strategy:   StrategyAuto,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolved before calling")
}

func TestSetupStrategy_Branch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), dir))

	setup, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyBranch,
		BranchName: "fry/test-branch",
		EpicName:   "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, dir, setup.WorkDir)
	assert.Equal(t, dir, setup.OriginalDir)
	assert.Equal(t, "fry/test-branch", setup.BranchName)
	assert.Equal(t, StrategyBranch, setup.Strategy)
	assert.False(t, setup.IsWorktree)

	// Verify we're on the new branch
	assert.Equal(t, "fry/test-branch", CurrentBranch(context.Background(), dir))
}

func TestSetupStrategy_Branch_AlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), dir))

	// Create branch first
	_, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyBranch,
		BranchName: "fry/existing",
	})
	require.NoError(t, err)

	// Switch back to original branch
	cmd := exec.Command("bash", "-c", "git checkout -")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	// Try to create same branch without ForceReuse
	_, err = SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyBranch,
		BranchName: "fry/existing",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestSetupStrategy_Branch_ForceReuse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), dir))

	// Create branch first
	_, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyBranch,
		BranchName: "fry/reuse-me",
	})
	require.NoError(t, err)

	// Switch back
	cmd := exec.Command("bash", "-c", "git checkout -")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	// ForceReuse should succeed
	setup, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyBranch,
		BranchName: "fry/reuse-me",
		ForceReuse: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "fry/reuse-me", setup.BranchName)
	assert.Equal(t, "fry/reuse-me", CurrentBranch(context.Background(), dir))
}

func TestSetupStrategy_Branch_NoGitRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyBranch,
		BranchName: "fry/test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "existing git repository")
}

func TestSetupStrategy_Worktree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), dir))

	// Create .fry/ with an artifact to verify copying
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("# Epic\n"), 0o644))

	setup, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyWorktree,
		BranchName: "fry/wt-test",
		EpicName:   "Test",
	})
	require.NoError(t, err)

	assert.NotEqual(t, dir, setup.WorkDir)
	assert.Equal(t, dir, setup.OriginalDir)
	assert.Equal(t, "fry/wt-test", setup.BranchName)
	assert.Equal(t, StrategyWorktree, setup.Strategy)
	assert.True(t, setup.IsWorktree)

	// Verify worktree directory exists and is a git repo
	assert.True(t, IsInsideGitRepo(context.Background(), setup.WorkDir))

	// Verify .fry/ was copied
	data, err := os.ReadFile(filepath.Join(setup.WorkDir, config.FryDir, "epic.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Epic\n", string(data))

	// Cleanup
	require.NoError(t, setup.Cleanup())
}

func TestSetupStrategy_Worktree_NoGitRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyWorktree,
		BranchName: "fry/test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "existing git repository")
}

func TestSetupStrategy_BranchNameAutoGenerated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), dir))

	setup, err := SetupStrategy(context.Background(), StrategyOpts{
		ProjectDir: dir,
		Strategy:   StrategyBranch,
		EpicName:   "My Cool Epic",
	})
	require.NoError(t, err)
	assert.Equal(t, "fry/my-cool-epic", setup.BranchName)
}

func TestDetectExistingSetup_Branch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), dir))

	// Create a branch
	cmd := exec.Command("bash", "-c", "git checkout -b fry/detect-me && git checkout -")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	setup, err := DetectExistingSetup(context.Background(), dir, "fry/detect-me")
	require.NoError(t, err)
	require.NotNil(t, setup)
	assert.Equal(t, StrategyBranch, setup.Strategy)
	assert.Equal(t, "fry/detect-me", setup.BranchName)
}

func TestDetectExistingSetup_Nothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitGit(context.Background(), dir))

	setup, err := DetectExistingSetup(context.Background(), dir, "fry/nonexistent")
	require.NoError(t, err)
	assert.Nil(t, setup)
}

func TestPersistAndReadStrategy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))

	original := &StrategySetup{
		WorkDir:     "/tmp/worktree",
		OriginalDir: dir,
		BranchName:  "fry/my-branch",
		Strategy:    StrategyWorktree,
		IsWorktree:  true,
	}

	require.NoError(t, PersistStrategy(dir, original))

	loaded, err := ReadPersistedStrategy(dir)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, original.Strategy, loaded.Strategy)
	assert.Equal(t, original.BranchName, loaded.BranchName)
	assert.Equal(t, original.WorkDir, loaded.WorkDir)
	assert.Equal(t, original.OriginalDir, loaded.OriginalDir)
	assert.True(t, loaded.IsWorktree)
}

func TestReadPersistedStrategy_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setup, err := ReadPersistedStrategy(dir)
	require.NoError(t, err)
	assert.Nil(t, setup)
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello world", "hello-world"},
		{"special chars", "Hello's World! (v2)", "hello-s-world-v2"},
		{"empty", "", ""},
		{"hyphens", "foo---bar", "foo-bar"},
		{"long", strings.Repeat("x", 100), strings.Repeat("x", 50)},
		{"trailing hyphens on truncation", strings.Repeat("ab-", 30), strings.TrimRight(strings.Repeat("ab-", 30)[:50], "-")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, slugify(tt.input))
		})
	}
}

func TestCopyDirIfExists(t *testing.T) {
	t.Parallel()

	t.Run("copies files recursively", func(t *testing.T) {
		t.Parallel()
		src := t.TempDir()
		dst := filepath.Join(t.TempDir(), "dest")

		require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("aaa"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("bbb"), 0o644))

		require.NoError(t, copyDirIfExists(src, dst))

		data, err := os.ReadFile(filepath.Join(dst, "a.txt"))
		require.NoError(t, err)
		assert.Equal(t, "aaa", string(data))

		data, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
		require.NoError(t, err)
		assert.Equal(t, "bbb", string(data))
	})

	t.Run("src does not exist", func(t *testing.T) {
		t.Parallel()
		dst := filepath.Join(t.TempDir(), "dest")
		require.NoError(t, copyDirIfExists("/nonexistent/path", dst))
		_, err := os.Stat(dst)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestCleanup_Idempotent(t *testing.T) {
	t.Parallel()

	setup := &StrategySetup{
		WorkDir:    "/tmp/test",
		IsWorktree: true,
	}
	require.NoError(t, setup.Cleanup())
	require.NoError(t, setup.Cleanup()) // second call is no-op
}

func TestCleanup_Nil(t *testing.T) {
	t.Parallel()

	var setup *StrategySetup
	require.NoError(t, setup.Cleanup())
}

// #32: CheckoutBranchWith tests

func TestCheckoutBranchWith_Success(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		CheckoutFn: func(_ context.Context, _ string, _ string) error {
			return nil
		},
	}
	err := CheckoutBranchWith(context.Background(), t.TempDir(), "fry/my-branch", ex)
	require.NoError(t, err)
}

func TestCheckoutBranchWith_Error(t *testing.T) {
	t.Parallel()

	ex := &mockExecutor{
		CheckoutFn: func(_ context.Context, _ string, _ string) error {
			return errors.New("branch not found")
		},
	}
	err := CheckoutBranchWith(context.Background(), t.TempDir(), "fry/nonexistent", ex)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branch not found")
}
