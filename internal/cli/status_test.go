package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/config"
)

func newTestCmd(t *testing.T, projectDir string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String("project-dir", projectDir, "")
	cmd.SetContext(context.Background())
	return cmd
}

func TestStatusCommandNoBuild(t *testing.T) {
	t.Parallel()

	// .fry/ exists but no epic — this is a scaffolded project with no build.
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry"), 0o755))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No active build")
}

func TestStatusCommandWithBuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fryDir := filepath.Join(dir, ".fry")
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("@epic Test Epic\n"), 0o644))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Test Epic")
}

func TestStatusCommandNoBuild_WithArchives(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create an archived build
	archiveDir := filepath.Join(dir, ".fry-archive", ".fry--build--20260327-120000")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "epic.md"),
		[]byte("@epic Archived Epic\n@sprint Setup\n@sprint Build\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "epic-progress.txt"),
		[]byte("## Sprint 1: Setup \u2014 PASS\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "build-mode.txt"),
		[]byte("software"), 0o644))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No active build found")
	assert.Contains(t, output, "Archived Builds (1)")
	assert.Contains(t, output, "Archived Epic")
	assert.Contains(t, output, "1/2 sprints passed")
}

func TestStatusCommandNoBuild_WithWorktrees(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a worktree with a build
	wtFry := filepath.Join(dir, ".fry-worktrees", "my-epic", ".fry")
	require.NoError(t, os.MkdirAll(wtFry, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry, "epic.md"),
		[]byte("@epic Worktree Epic\n@sprint S1\n@sprint S2\n@sprint S3\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry, "build-mode.txt"),
		[]byte("software"), 0o644))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No active build found")
	assert.Contains(t, output, "Worktree Builds (1)")
	assert.Contains(t, output, "Worktree Epic")
	assert.Contains(t, output, ".fry-worktrees/my-epic/")
}

func TestStatusCommandNoBuild_Empty(t *testing.T) {
	t.Parallel()

	// .fry/ exists but no epic and no archives — scaffolded, never built.
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry"), 0o755))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No active build found")
	assert.Contains(t, output, "Run 'fry run' to start a build.")
	assert.NotContains(t, output, "Archived")
	assert.NotContains(t, output, "Worktree")
}

func TestStatusCommandNoFryProject(t *testing.T) {
	t.Parallel()

	// Completely empty directory — no .fry/, no archives, no worktrees.
	dir := t.TempDir()

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No fry project found")
	assert.Contains(t, output, "Run 'fry init' to get started.")
}

func TestStatusCommandConsciousnessLocal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry", "consciousness", "distilled"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry", "consciousness", "upload-queue"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.ConsciousnessSessionFile), []byte(`{
	  "session_id": "session-1",
	  "status": "running",
	  "current_sprint": 2,
	  "total_sprints": 4,
	  "checkpoints_persisted": 3,
	  "parse_failures": 1,
	  "repair_successes": 2,
	  "distillations_succeeded": 2,
	  "distillations_failed": 1,
	  "upload_attempts": 5,
	  "upload_successes": 4,
	  "session_resumed_count": 1
	}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry", "consciousness", "distilled", "0001.json"), []byte(`{"session_id":"session-1","sequence":1,"summary":"checkpoint"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry", "consciousness", "upload-queue", "pending.json"), []byte(`{"id":"pending","event_type":"checkpoint_summary"}`), 0o644))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.Flags().Bool("consciousness", true, "")
	require.NoError(t, fakeCmd.Flags().Set("consciousness", "true"))
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Consciousness Session")
	assert.Contains(t, output, "Checkpoints persisted: 3")
	assert.Contains(t, output, "Parse failures:        1")
	assert.Contains(t, output, "1 pending")
}

func newTestCmdWithRunsFlags(t *testing.T, projectDir string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String("project-dir", projectDir, "")
	cmd.Flags().Bool("runs", false, "")
	cmd.Flags().String("run", "", "")
	cmd.Flags().Bool("json", false, "")
	cmd.Flags().Bool("consciousness", false, "")
	cmd.Flags().Bool("consciousness-remote", false, "")
	cmd.SetContext(context.Background())
	return cmd
}

func writeRunSnapshot(t *testing.T, dir, runID string, status *agent.BuildStatus) {
	t.Helper()
	runDir := filepath.Join(dir, config.RunsDir, runID)
	require.NoError(t, os.MkdirAll(runDir, 0o755))
	data, err := json.MarshalIndent(status, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "build-status.json"), data, 0o644))
}

func TestStatusRuns_ListsRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeRunSnapshot(t, dir, "run-20260402-100000", &agent.BuildStatus{
		Version: 1,
		Run:     &agent.RunMeta{RunID: "run-20260402-100000", RunType: agent.RunTypeFresh},
		Build:   agent.BuildInfo{Epic: "Test", Status: "completed"},
		Sprints: []agent.SprintStatus{{Number: 1, Name: "S1", Status: "PASS"}},
	})
	writeRunSnapshot(t, dir, "run-20260403-060000", &agent.BuildStatus{
		Version: 1,
		Run:     &agent.RunMeta{RunID: "run-20260403-060000", RunType: agent.RunTypeContinue, ParentRunID: "run-20260402-100000"},
		Build:   agent.BuildInfo{Epic: "Test", Status: "failed"},
		Sprints: []agent.SprintStatus{{Number: 1, Name: "S1", Status: "running"}},
	})

	var buf bytes.Buffer
	cmd := newTestCmdWithRunsFlags(t, dir)
	require.NoError(t, cmd.Flags().Set("runs", "true"))
	cmd.SetOut(&buf)

	err := statusCmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "run-20260402-100000")
	assert.Contains(t, output, "run-20260403-060000")
	assert.Contains(t, output, "fresh")
	assert.Contains(t, output, "continue")
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, "failed")
}

func TestStatusRuns_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	var buf bytes.Buffer
	cmd := newTestCmdWithRunsFlags(t, dir)
	require.NoError(t, cmd.Flags().Set("runs", "true"))
	cmd.SetOut(&buf)

	err := statusCmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No run snapshots found")
}

func TestStatusRun_ByID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeRunSnapshot(t, dir, "run-20260402-100000", &agent.BuildStatus{
		Version: 1,
		Run:     &agent.RunMeta{RunID: "run-20260402-100000", RunType: agent.RunTypeFresh},
		Build: agent.BuildInfo{
			Epic:   "Specific Run",
			Status: "completed",
			Engine: "claude",
			Effort: "max",
		},
		Sprints: []agent.SprintStatus{
			{Number: 1, Name: "Scaffolding", Status: "PASS"},
			{Number: 2, Name: "Core", Status: "PASS"},
		},
	})

	var buf bytes.Buffer
	cmd := newTestCmdWithRunsFlags(t, dir)
	require.NoError(t, cmd.Flags().Set("run", "run-20260402-100000"))
	cmd.SetOut(&buf)

	err := statusCmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "run-20260402-100000")
	assert.Contains(t, output, "Specific Run")
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, "Scaffolding")
	assert.Contains(t, output, "Core")
}

func TestStatusRun_ByShortID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeRunSnapshot(t, dir, "run-20260402-100000", &agent.BuildStatus{
		Version: 1,
		Run:     &agent.RunMeta{RunID: "run-20260402-100000", RunType: agent.RunTypeFresh},
		Build:   agent.BuildInfo{Epic: "Short ID Test", Status: "completed"},
		Sprints: []agent.SprintStatus{},
	})

	var buf bytes.Buffer
	cmd := newTestCmdWithRunsFlags(t, dir)
	// Pass without the "run-" prefix — should auto-prepend
	require.NoError(t, cmd.Flags().Set("run", "20260402-100000"))
	cmd.SetOut(&buf)

	err := statusCmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Short ID Test")
}

func TestStatusRun_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	var buf bytes.Buffer
	cmd := newTestCmdWithRunsFlags(t, dir)
	require.NoError(t, cmd.Flags().Set("run", "run-nonexistent"))
	cmd.SetOut(&buf)

	err := statusCmd.RunE(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStatusRun_RejectsPathTraversal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	tests := []string{
		"run-foo/../../..",
		"../../../etc",
		"run-x\\..\\..\\etc",
	}
	for _, input := range tests {
		var buf bytes.Buffer
		cmd := newTestCmdWithRunsFlags(t, dir)
		require.NoError(t, cmd.Flags().Set("run", input))
		cmd.SetOut(&buf)

		err := statusCmd.RunE(cmd, []string{})
		assert.Error(t, err, "input %q should be rejected", input)
		assert.Contains(t, err.Error(), "invalid run ID", "input %q", input)
	}
}
