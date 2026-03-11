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
	projectDir := validProjectDir(t)

	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.NoError(t, err)
}

func TestPreflightBashExists(t *testing.T) {
	projectDir := validProjectDir(t)

	err := RunPreflight(PreflightConfig{
		ProjectDir: projectDir,
		Engine:     "bash",
	})
	require.NoError(t, err)
}

func TestPreflightMissingTool(t *testing.T) {
	projectDir := validProjectDir(t)

	err := RunPreflight(PreflightConfig{
		ProjectDir:    projectDir,
		Engine:        "bash",
		RequiredTools: []string{"definitely-not-a-real-tool-binary"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required tool")
}

func TestPreflightAgentsFile(t *testing.T) {
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

func validProjectDir(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.PlansDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.PlanFile), []byte("plan\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, config.AgentsFile), []byte("# AGENTS.md — fry-go\n1\n2\n3\n4\n"), 0o644))
	return projectDir
}
