package continuerun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/git"
)

func TestResolveContinueTarget_NoPersistedStrategy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target, err := ResolveContinueTarget(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, dir, target.ProjectDir)
	assert.Nil(t, target.Strategy)
}

func TestResolveContinueTarget_PrefersOriginalWhenMoreComplete(t *testing.T) {
	t.Parallel()

	root := initResolverRepo(t)
	worktree := initResolverRepo(t)

	writeResolverEpicProgress(t, root, "## Sprint 1: Setup — PASS\n\n## Sprint 2: Auth — PASS\n")
	writeResolverEpicProgress(t, worktree, "")
	require.NoError(t, git.PersistStrategy(root, &git.StrategySetup{
		Strategy:    git.StrategyWorktree,
		WorkDir:     worktree,
		OriginalDir: root,
		IsWorktree:  true,
	}))

	target, err := ResolveContinueTarget(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, root, target.ProjectDir)
	assert.Nil(t, target.Strategy)
	assert.Contains(t, target.Reason, "original project state is newer or more complete")
}

func TestResolveContinueTarget_PrefersWorktreeWhenMoreComplete(t *testing.T) {
	t.Parallel()

	root := initResolverRepo(t)
	worktree := initResolverRepo(t)

	writeResolverEpicProgress(t, root, "## Sprint 1: Setup — PASS\n")
	writeResolverEpicProgress(t, worktree, "## Sprint 1: Setup — PASS\n\n## Sprint 2: Auth — PASS\n")
	setup := &git.StrategySetup{
		Strategy:    git.StrategyWorktree,
		WorkDir:     worktree,
		OriginalDir: root,
		IsWorktree:  true,
	}
	require.NoError(t, git.PersistStrategy(root, setup))

	target, err := ResolveContinueTarget(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, worktree, target.ProjectDir)
	require.NotNil(t, target.Strategy)
	assert.Equal(t, worktree, target.Strategy.WorkDir)
}

func TestResolveContinueTarget_BreaksTieByNewerStatus(t *testing.T) {
	t.Parallel()

	root := initResolverRepo(t)
	worktree := initResolverRepo(t)

	writeResolverEpicProgress(t, root, "## Sprint 1: Setup — PASS\n")
	writeResolverEpicProgress(t, worktree, "## Sprint 1: Setup — PASS\n")
	writeResolverBuildStatus(t, root, time.Now().Add(-2*time.Hour))
	writeResolverBuildStatus(t, worktree, time.Now())
	setup := &git.StrategySetup{
		Strategy:    git.StrategyWorktree,
		WorkDir:     worktree,
		OriginalDir: root,
		IsWorktree:  true,
	}
	require.NoError(t, git.PersistStrategy(root, setup))

	target, err := ResolveContinueTarget(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, worktree, target.ProjectDir)
}

func initResolverRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	cmd := exec.Command("git", "init", "-q", dir)
	require.NoError(t, cmd.Run())
	return dir
}

func writeResolverEpicProgress(t *testing.T, dir, body string) {
	t.Helper()
	content := "# Epic Progress — Test\n\n" + body
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.EpicProgressFile), []byte(content), 0o644))
}

func writeResolverBuildStatus(t *testing.T, dir string, updated time.Time) {
	t.Helper()
	data := fmt.Sprintf("{\n  \"version\": 1,\n  \"updated_at\": %q,\n  \"build\": {\n    \"epic\": \"Test\",\n    \"status\": \"running\",\n    \"phase\": \"sprint\",\n    \"started_at\": %q\n  },\n  \"sprints\": []\n}\n", updated.Format(time.RFC3339Nano), updated.Format(time.RFC3339Nano))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.BuildStatusFile), []byte(data), 0o644))
}
