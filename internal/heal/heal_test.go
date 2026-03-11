package heal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/verify"
)

func TestHealPromptStructure(t *testing.T) {
	// Use a temp dir with no executive.md → reference should be absent
	projectDir := t.TempDir()
	opts := HealOpts{
		ProjectDir: projectDir,
		Sprint: &epic.Sprint{
			Number: 3,
			Name:   "Setup Auth",
		},
	}

	report := "Verification: 2/5 checks passed.\n\nFailed checks:\n- FAILED: File missing or empty: src/auth.ts"
	expected := "# HEAL MODE — Sprint 3: Setup Auth\n\n" +
		"## What happened\n" +
		"The sprint finished its work but FAILED independent verification checks.\n" +
		"Your job is to fix ONLY the issues described below. Do not start the sprint over.\n" +
		"Do not refactor or reorganize. Make the minimum changes needed to pass the checks.\n\n" +
		"## Failed verification checks\n\n" +
		report + "\n\n" +
		"## Instructions\n" +
		"1. Read .fry/sprint-progress.txt for context on what was built this sprint\n" +
		"2. Read .fry/epic-progress.txt for context on what was built in prior sprints\n" +
		"3. Read the failed checks above carefully\n" +
		"4. Fix each failure — create missing files, fix build errors, correct config\n" +
		"5. After fixing, do a final sanity check (e.g., run the build command if applicable)\n" +
		"6. Append a brief note to .fry/sprint-progress.txt about what you fixed in this heal pass\n\n" +
		"## Context files\n" +
		"- Read .fry/sprint-progress.txt for current sprint iteration history\n" +
		"- Read .fry/epic-progress.txt for prior sprint summaries\n" +
		"- Read plans/plan.md for the overall project plan\n" +
		"\n" +
		"Do NOT output any promise tokens. Just fix the issues.\n"

	assert.Equal(t, expected, buildHealPrompt(opts, report))
}

func TestHealPromptWithExecutiveAndUserDirective(t *testing.T) {
	projectDir := t.TempDir()
	// Create executive.md so the conditional reference appears
	writeFile(t, filepath.Join(projectDir, config.ExecutiveFile), "Executive context\n")

	opts := HealOpts{
		ProjectDir: projectDir,
		UserPrompt: "Focus on auth module",
		Sprint: &epic.Sprint{
			Number: 2,
			Name:   "Auth Layer",
		},
	}

	report := "Verification: 1/3 checks passed."
	result := buildHealPrompt(opts, report)

	assert.Contains(t, result, "- Read plans/executive.md for executive context\n")
	assert.Contains(t, result, "- User directive: Focus on auth module\n")
	// User directive and executive should be in context files section, not separate headers
	assert.NotContains(t, result, "## User Directive")
}

func TestHealLoopMaxAttempts(t *testing.T) {
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, config.SprintProgressFile), "")
	sprintLog := filepath.Join(projectDir, config.BuildLogsDir, "sprint1.log")
	require.NoError(t, os.MkdirAll(filepath.Dir(sprintLog), 0o755))

	mockEngine := &stubEngine{name: "codex"}
	healed, err := RunHealLoop(context.Background(), HealOpts{
		ProjectDir: projectDir,
		Sprint: &epic.Sprint{
			Number: 1,
			Name:   "Heal",
		},
		Epic: &epic.Epic{
			TotalSprints:    2,
			MaxHealAttempts: 2,
		},
		Engine:        mockEngine,
		SprintLogFile: sprintLog,
		Checks: []verify.Check{
			{Sprint: 1, Type: verify.CheckFile, Path: "missing.txt"},
		},
	})
	require.NoError(t, err)
	assert.False(t, healed)
	assert.Len(t, mockEngine.prompts, 2)
	for _, prompt := range mockEngine.prompts {
		assert.Equal(t, config.HealInvocationPrompt, prompt)
	}
}

func TestHealPerSprintOverride(t *testing.T) {
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, config.SprintProgressFile), "")
	sprintLog := filepath.Join(projectDir, config.BuildLogsDir, "sprint1.log")
	require.NoError(t, os.MkdirAll(filepath.Dir(sprintLog), 0o755))

	override := 1
	mockEngine := &stubEngine{name: "claude"}
	healed, err := RunHealLoop(context.Background(), HealOpts{
		ProjectDir: projectDir,
		Sprint: &epic.Sprint{
			Number:          1,
			Name:            "Heal",
			MaxHealAttempts: &override,
		},
		Epic: &epic.Epic{
			TotalSprints:    2,
			MaxHealAttempts: 3,
		},
		Engine:        mockEngine,
		SprintLogFile: sprintLog,
		Checks: []verify.Check{
			{Sprint: 1, Type: verify.CheckFile, Path: "missing.txt"},
		},
	})
	require.NoError(t, err)
	assert.False(t, healed)
	assert.Len(t, mockEngine.prompts, 1)
}

type stubEngine struct {
	name    string
	prompts []string
}

func (s *stubEngine) Run(_ context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	s.prompts = append(s.prompts, prompt)
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte("heal output\n"))
	}
	return "heal output\n", 0, nil
}

func (s *stubEngine) Name() string {
	return s.name
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
