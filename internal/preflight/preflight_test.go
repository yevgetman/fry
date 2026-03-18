package preflight

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
)

func TestPreflightGitExists(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)
	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.NoError(t, err)
}

func TestPreflightBashExists(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)
	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.NoError(t, err)
}

func TestPreflightMissingTool(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)
	err := RunPreflight(PreflightConfig{
		ProjectDir:    projectDir,
		Engine:        "bash",
		RequiredTools: []string{"definitely-not-a-real-tool-binary"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required tool")
	assert.Contains(t, err.Error(), "definitely-not-a-real-tool-binary")
}

func TestPreflightAgentsFile(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)

	agentsPath := filepath.Join(projectDir, config.AgentsFile)
	require.NoError(t, os.WriteFile(agentsPath, []byte("# AGENTS.md — PLACEHOLDER\none\ntwo\nthree\nfour\n"), 0o644))
	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "placeholder")

	require.NoError(t, os.WriteFile(agentsPath, []byte("# AGENTS.md — fry-go\none\ntwo\nthree\n"), 0o644))
	err = RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 5 lines")
}

// P2: Missing engine CLI

func TestPreflightMissingEngine(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)
	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "definitely-not-a-real-engine",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine CLI")
	assert.Contains(t, err.Error(), "not found on PATH")
}

// P2: Default engine used when empty

func TestPreflightDefaultEngine(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)
	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "", // Should default to config.DefaultEngine
	})
	// Will likely fail since default engine (codex) isn't on PATH in test env,
	// but the error should reference the default engine name
	if err != nil {
		assert.Contains(t, err.Error(), config.DefaultEngine)
	}
}

// P2: Missing plans directory

func TestPreflightMissingPlansDir(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	// Create only agents file, no plans dir
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.AgentsFile), []byte("# AGENTS.md — fry-go\n1\n2\n3\n4\n"), 0o644))

	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plans directory missing")
}

// P2: Missing both plan.md and executive.md

func TestPreflightMissingBothPlanAndExecutive(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.PlansDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.AgentsFile), []byte("# AGENTS.md — fry-go\n1\n2\n3\n4\n"), 0o644))
	// No plan.md, no executive.md

	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing both")
}

// P2: Executive.md works instead of plan.md

func TestPreflightExecutiveInsteadOfPlan(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.PlansDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.ExecutiveFile), []byte("executive\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.AgentsFile), []byte("# AGENTS.md — fry-go\n1\n2\n3\n4\n"), 0o644))

	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.NoError(t, err)
}

// P2: Missing agents file

func TestPreflightMissingAgentsFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.PlansDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.PlanFile), []byte("plan\n"), 0o644))
	// No agents file

	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
	assert.Contains(t, err.Error(), config.AgentsFile)
}

// P2: Preflight command failure

func TestPreflightCommandFailure(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)
	err := RunPreflight(PreflightConfig{
		ProjectDir:    projectDir,
		Engine:        "bash",
		PreflightCmds: []string{"exit 1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command failed")
}

// P2: Preflight command success

func TestPreflightCommandSuccess(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)
	err := RunPreflight(PreflightConfig{
		ProjectDir:    projectDir,
		Engine:        "bash",
		PreflightCmds: []string{"echo ok"},
	})
	require.NoError(t, err)
}

// P2: Docker not needed when sprint is before docker_from_sprint

func TestPreflightDockerNotNeeded(t *testing.T) {
	t.Parallel()

	projectDir := validProjectDir(t)
	// Docker from sprint 3, current is sprint 1 — no docker check
	err := RunPreflight(PreflightConfig{
		ProjectDir:       projectDir,
		Engine:           "bash",
		DockerFromSprint: 3,
		CurrentSprint:    1,
	})
	require.NoError(t, err)
}

func validProjectDir(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.PlansDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.PlanFile), []byte("plan\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.AgentsFile), []byte("# AGENTS.md — fry-go\n1\n2\n3\n4\n"), 0o644))
	return projectDir
}
