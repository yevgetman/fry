package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/sprint"
)

func TestDryRunParsing(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".fry"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "plans"), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".fry", "AGENTS.md"), []byte("# AGENTS.md\n1. Keep changes minimal.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "plans", "plan.md"), []byte("# Plan\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".fry", "epic.md"), []byte(`
@epic CLI Integration
@engine codex
@verification .fry/verification.md

@sprint 1
@name Wire CLI
@max_iterations 1
@promise cli-wired
@prompt
Implement the CLI.
@end
`), 0o644))

	epicPath, exists, err := resolveEpicPath(projectDir, filepath.Join(".fry", "epic.md"))
	require.NoError(t, err)
	require.True(t, exists)

	parsed, err := epic.ParseEpic(epicPath)
	require.NoError(t, err)
	require.NoError(t, epic.ValidateEpic(parsed))

	engineName, err := engine.ResolveEngine("", parsed.Engine, "", "")
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, printDryRunReport(&out, projectDir, epicPath, parsed, engineName, 1, parsed.TotalSprints))
	require.Contains(t, out.String(), "Epic: CLI Integration")
	require.Contains(t, out.String(), "Engine: codex")
}

func TestEpicPathResolution(t *testing.T) {
	projectDir := t.TempDir()

	existingPath := filepath.Join(projectDir, "custom-epic.md")
	require.NoError(t, os.WriteFile(existingPath, []byte("@epic x\n"), 0o644))

	resolved, exists, err := resolveEpicPath(projectDir, existingPath)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, existingPath, resolved)

	resolved, exists, err = resolveEpicPath(projectDir, "missing-epic.md")
	require.NoError(t, err)
	require.False(t, exists)
	require.Equal(t, filepath.Join(projectDir, ".fry", "missing-epic.md"), resolved)
}

func TestEngineResolution(t *testing.T) {
	t.Setenv("FRY_ENGINE", "claude")

	name, err := engine.ResolveEngine("codex", "claude", "", "")
	require.NoError(t, err)
	require.Equal(t, "codex", name)

	name, err = engine.ResolveEngine("", "codex", "", "")
	require.NoError(t, err)
	require.Equal(t, "codex", name)

	name, err = engine.ResolveEngine("", "", "", "")
	require.NoError(t, err)
	require.Equal(t, "claude", name)

	t.Setenv("FRY_ENGINE", "")
	name, err = engine.ResolveEngine("", "", "", "")
	require.NoError(t, err)
	require.Equal(t, "codex", name)
}

func TestBuildSucceeds(t *testing.T) {
	runRepoCommand(t, "go", "build", "./...")
}

func TestVetPasses(t *testing.T) {
	runRepoCommand(t, "go", "vet", "./...")
}

func TestBuildSummaryIncludesSprintName(t *testing.T) {
	var out bytes.Buffer
	printBuildSummary(&out, []sprint.SprintResult{
		{
			Number:   1,
			Name:     "Wire CLI",
			Status:   sprint.StatusPass,
			Duration: 2 * time.Second,
		},
	})

	require.Contains(t, out.String(), "SPRINT")
	require.Contains(t, out.String(), "NAME")
	require.Contains(t, out.String(), "STATUS")
	require.Contains(t, out.String(), "DURATION")
	require.Contains(t, out.String(), "Wire CLI")
	require.Contains(t, out.String(), "PASS")
	require.Contains(t, out.String(), "2s")
}

func TestResolveSprintRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		totalSprints int
		wantStart    int
		wantEnd      int
		wantErr      string
	}{
		{"no args defaults to full range", []string{}, 5, 1, 5, ""},
		{"start only", []string{"3"}, 5, 3, 5, ""},
		{"start and end", []string{"2", "4"}, 5, 2, 4, ""},
		{"single sprint", []string{"1", "1"}, 5, 1, 1, ""},
		{"last sprint only", []string{"5", "5"}, 5, 5, 5, ""},
		{"invalid start", []string{"abc"}, 5, 0, 0, "invalid start sprint"},
		{"invalid end", []string{"1", "xyz"}, 5, 0, 0, "invalid end sprint"},
		{"start too low", []string{"0"}, 5, 0, 0, "invalid sprint range"},
		{"end exceeds total", []string{"1", "6"}, 5, 0, 0, "invalid sprint range"},
		{"start after end", []string{"3", "2"}, 5, 0, 0, "invalid sprint range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			start, end, err := resolveSprintRange(tt.args, tt.totalSprints)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantStart, start)
				assert.Equal(t, tt.wantEnd, end)
			}
		})
	}
}

func runRepoCommand(t *testing.T, name string, args ...string) {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	cmd := exec.Command(name, args...)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}
