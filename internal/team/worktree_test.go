package team

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWorkerFinalizesWorktreeIntegration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "user.email", "test@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note.txt"), []byte("base\n"), 0o644))
	runGit(t, dir, "add", "note.txt")
	runGit(t, dir, "commit", "-m", "initial")

	require.NoError(t, ensureTeamLayout(dir, "git-team"))
	cfg := &Config{
		TeamID:           "git-team",
		ProjectDir:       dir,
		BuildDir:         dir,
		Status:           StatusRunning,
		TMuxSession:      "git-team-session",
		GitIsolationMode: IsolationPerWorkerWorktree,
	}
	require.NoError(t, SaveConfig(dir, cfg))
	require.NoError(t, setActiveTeam(dir, "git-team"))

	identity, err := createWorkerIdentity(context.Background(), cfg, "worker-1", "executor")
	require.NoError(t, err)
	require.NoError(t, SaveWorkerRecord(dir, "git-team", &WorkerRecord{
		WorkerID:      "worker-1",
		Status:        WorkerIdle,
		DesiredStatus: WorkerRunning,
	}))
	task := &Task{
		ID:        "001",
		Title:     "modify note",
		Command:   "printf 'worker-change\\n' >> note.txt",
		Status:    TaskPending,
		Priority:  1,
		CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, SaveTask(dir, "git-team", task))

	err = RunWorker(context.Background(), WorkerRunOptions{
		ProjectDir: dir,
		TeamID:     "git-team",
		WorkerID:   identity.WorkerID,
		Once:       true,
	})
	require.NoError(t, err)

	cfg, err = LoadConfig(dir, "git-team")
	require.NoError(t, err)
	assert.Equal(t, StatusComplete, cfg.Status)
	assert.DirExists(t, IntegratedOutputDir(dir, "git-team"))

	data, err := os.ReadFile(filepath.Join(IntegratedOutputDir(dir, "git-team"), "note.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "base")
	assert.Contains(t, string(data), "worker-change")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, strings.TrimSpace(string(output)))
}
