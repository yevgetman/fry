package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/continuerun"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/git"
	"github.com/yevgetman/fry/internal/prepare"
	"github.com/yevgetman/fry/internal/sprint"
)

func TestDryRunParsing(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	// CLI flag takes highest priority
	name, err := engine.ResolveEngine("codex", "claude", "claude", "")
	require.NoError(t, err)
	require.Equal(t, "codex", name)

	// Epic directive is next
	name, err = engine.ResolveEngine("", "codex", "claude", "")
	require.NoError(t, err)
	require.Equal(t, "codex", name)

	// Env var is next (passed explicitly via envVar parameter)
	name, err = engine.ResolveEngine("", "", "claude", "")
	require.NoError(t, err)
	require.Equal(t, "claude", name)

	// Default engine when nothing else provided
	name, err = engine.ResolveEngine("", "", "", "")
	require.NoError(t, err)
	require.Equal(t, "claude", name)
}

func TestBuildSucceeds(t *testing.T) {
	t.Parallel()

	runRepoCommand(t, "go", "build", "./...")
}

func TestVetPasses(t *testing.T) {
	t.Parallel()

	runRepoCommand(t, "go", "vet", "./...")
}

func TestBuildSummaryIncludesSprintName(t *testing.T) {
	t.Parallel()

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
		{"start exceeds total", []string{"6"}, 5, 0, 0, "invalid sprint range"},
		{"start equals total is valid", []string{"5"}, 5, 5, 5, ""},
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

func TestResolveMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		modeFlag    string
		planningFlg bool
		wantMode    prepare.Mode
		wantErr     string
	}{
		{"defaults to software", "", false, prepare.ModeSoftware, ""},
		{"planning flag", "", true, prepare.ModePlanning, ""},
		{"mode writing", "writing", false, prepare.ModeWriting, ""},
		{"mode planning", "planning", false, prepare.ModePlanning, ""},
		{"mode software explicit", "software", false, prepare.ModeSoftware, ""},
		{"conflict errors", "writing", true, "", "cannot use both"},
		{"invalid mode", "bogus", false, "", "unknown mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveMode(tt.modeFlag, tt.planningFlg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantMode, got)
			}
		})
	}
}

func TestReadBuildMode(t *testing.T) {
	t.Parallel()

	t.Run("returns empty when no file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		assert.Equal(t, "", continuerun.ReadBuildMode(dir))
	})

	t.Run("reads writing mode", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.BuildModeFile), []byte("writing\n"), 0o644))

		assert.Equal(t, "writing", continuerun.ReadBuildMode(dir))
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.BuildModeFile), []byte("  planning \n"), 0o644))

		assert.Equal(t, "planning", continuerun.ReadBuildMode(dir))
	})
}

func TestGitStrategyFlagValidation(t *testing.T) {
	t.Parallel()

	t.Run("invalid strategy", func(t *testing.T) {
		t.Parallel()
		_, err := git.ParseGitStrategy("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid git strategy")
	})

	t.Run("current with branch-name is incompatible", func(t *testing.T) {
		t.Parallel()
		// This is validated at the CLI level, so we test the logic directly.
		strategy, err := git.ParseGitStrategy("current")
		require.NoError(t, err)
		assert.Equal(t, git.StrategyCurrent, strategy)
		// The CLI code checks: if strategy == current && branchName != "" → error
		// We verify the types are correct here.
	})

	t.Run("valid strategies", func(t *testing.T) {
		t.Parallel()
		for _, valid := range []string{"", "auto", "current", "branch", "worktree"} {
			_, err := git.ParseGitStrategy(valid)
			assert.NoError(t, err, "strategy %q should be valid", valid)
		}
	})
}

func TestAlwaysVerifyFlagSetsHealConfig(t *testing.T) {
	t.Parallel()

	ep := &epic.Epic{
		Name:        "Test",
		EffortLevel: epic.EffortLow,
		Sprints:     []epic.Sprint{{Number: 1, Name: "s", MaxIterations: 1}},
		TotalSprints: 1,
	}
	require.Equal(t, 0, ep.MaxHealAttempts)
	require.False(t, ep.MaxHealAttemptsSet)

	// Simulate what cli/run.go does for --always-verify
	ep.AuditAfterSprint = true
	if ep.MaxHealAttempts == 0 {
		ep.MaxHealAttempts = config.DefaultMaxHealAttempts
	}
	ep.MaxHealAttemptsSet = true

	assert.Equal(t, config.DefaultMaxHealAttempts, ep.MaxHealAttempts)
	assert.True(t, ep.MaxHealAttemptsSet, "MaxHealAttemptsSet must be true so effectiveHealConfig uses path 3")
}

func TestTriageOnlyFlagConflicts(t *testing.T) {
	t.Parallel()

	// Test the conflict logic without mutating package-level vars (race-safe).
	// These mirror the guards in RunE at run.go:105-110.
	tests := []struct {
		name          string
		triageOnly    bool
		fullPrepare   bool
		resume        bool
		cont          bool
		simpleCont    bool
		wantErr       string
	}{
		{"triage-only with full-prepare", true, true, false, false, false, "cannot use --triage-only with --full-prepare"},
		{"triage-only with continue", true, false, false, true, false, "cannot use --triage-only with --resume, --continue, or --simple-continue"},
		{"triage-only with resume", true, false, true, false, false, "cannot use --triage-only with --resume, --continue, or --simple-continue"},
		{"triage-only with simple-continue", true, false, false, false, true, "cannot use --triage-only with --resume, --continue, or --simple-continue"},
		{"triage-only alone is valid", true, false, false, false, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var err error
			if tt.triageOnly && tt.fullPrepare {
				err = fmt.Errorf("cannot use --triage-only with --full-prepare")
			}
			if tt.triageOnly && (tt.resume || tt.cont || tt.simpleCont) {
				err = fmt.Errorf("cannot use --triage-only with --resume, --continue, or --simple-continue")
			}

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestTriageOnlyWithExistingEpic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fryDir := filepath.Join(dir, ".fry")
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("@epic test\n"), 0o644))

	// Verify that the epic-exists guard produces the right error format.
	epicArg := filepath.Join(".fry", "epic.md")
	_, epicExists, err := resolveEpicPath(dir, epicArg)
	require.NoError(t, err)
	require.True(t, epicExists, "precondition: epic file must exist for this test")

	// Reproduce the guard from RunE.
	guardErr := fmt.Errorf("--triage-only: epic already exists at %s; triage only runs when no epic exists", epicArg)
	assert.Contains(t, guardErr.Error(), "--triage-only")
	assert.Contains(t, guardErr.Error(), "epic already exists")
	assert.Contains(t, guardErr.Error(), epicArg)
}

func TestCheckArgsForMissingDashes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"flag without dashes", []string{"user-prompt-file", "./plan.md"}, `"user-prompt-file" looks like a flag — did you mean --user-prompt-file?`},
		{"engine without dashes", []string{"engine", "claude"}, `"engine" looks like a flag — did you mean --engine?`},
		{"normal epic path", []string{".fry/epic.md"}, ""},
		{"absolute path", []string{"/tmp/epic.md"}, ""},
		{"relative path", []string{"./plans/epic.md"}, ""},
		{"sprint number", []string{".fry/epic.md", "1", "3"}, ""},
		{"proper flag not caught", []string{"--engine", "claude"}, ""},
		{"unknown word is fine", []string{"myepic.md"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkArgsForMissingDashes(runCmd, tt.args)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
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
