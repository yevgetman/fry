package sprint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
)

func TestPromptAssembly(t *testing.T) {
	projectDir := t.TempDir()
	executive := "Executive context line.\n"

	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:       projectDir,
		ExecutiveContent: executive,
		UserPrompt:       "User directive.",
		SprintPrompt:     "Sprint instructions.",
		Promise:          "TOKEN",
	})
	require.NoError(t, err)

	// Layer 1: Executive context with orientation disclaimer
	assert.Contains(t, prompt, "# ===== PROJECT CONTEXT =====\n")
	assert.Contains(t, prompt, "# NOT derive implementation decisions from this section.\n")
	assert.Contains(t, prompt, "Executive context line.\n")

	// Layer 1.5: User directive with priority explanation
	assert.Contains(t, prompt, "# ===== USER DIRECTIVE =====\n")
	assert.Contains(t, prompt, "# Treat this as a priority directive that applies to all sprints.\n")
	assert.Contains(t, prompt, "User directive.\n")

	// Layer 2: Strategic plan with "true north" guidance
	assert.Contains(t, prompt, "# ===== STRATEGIC PLAN =====\n")
	assert.Contains(t, prompt, "\"true north\"")
	assert.Contains(t, prompt, "# Do NOT implement work from other phases")

	// Layer 3: Sprint instructions
	assert.Contains(t, prompt, "# ===== SPRINT INSTRUCTIONS =====\n\nSprint instructions.\n")

	// Layer 4: Iteration memory with detailed read/append instructions
	assert.Contains(t, prompt, "# ===== ITERATION MEMORY =====\n")
	assert.Contains(t, prompt, "# Two progress files track build history:\n")
	assert.Contains(t, prompt, "#    BEFORE you begin work, READ this file")
	assert.Contains(t, prompt, "#    AFTER you finish, APPEND a brief entry")
	assert.Contains(t, prompt, "#      ## Iteration N")
	assert.Contains(t, prompt, "#    Do NOT write to this file — it is managed by the build system.\n")

	// Layer 5: Completion signal (conditional on Promise)
	assert.Contains(t, prompt, "# ===== COMPLETION SIGNAL =====\n")
	assert.Contains(t, prompt, "# output exactly this line:\n# ===PROMISE: TOKEN===\n")
	assert.Contains(t, prompt, "# If tasks remain incomplete")

	written, err := os.ReadFile(filepath.Join(projectDir, config.PromptFile))
	require.NoError(t, err)
	assert.Equal(t, prompt, string(written))
}

func TestPromptAssemblyPartialLayers(t *testing.T) {
	projectDir := t.TempDir()

	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   projectDir,
		SprintPrompt: "Only sprint instructions.",
		Promise:      "TOKEN",
	})
	require.NoError(t, err)

	assert.NotContains(t, prompt, "# ===== PROJECT CONTEXT =====")
	assert.NotContains(t, prompt, "# ===== USER DIRECTIVE =====")
	assert.Contains(t, prompt, "# ===== STRATEGIC PLAN =====")
	assert.Contains(t, prompt, "# ===== SPRINT INSTRUCTIONS =====")
	assert.Contains(t, prompt, "# ===== COMPLETION SIGNAL =====")
}

func TestPromptAssemblyNoPromise(t *testing.T) {
	projectDir := t.TempDir()

	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:   projectDir,
		SprintPrompt: "Do the work.",
		Promise:      "",
	})
	require.NoError(t, err)

	assert.NotContains(t, prompt, "# ===== COMPLETION SIGNAL =====")
	assert.NotContains(t, prompt, "===PROMISE:")
}

func TestPromptAssemblyExactHeaders(t *testing.T) {
	projectDir := t.TempDir()
	prompt, err := AssemblePrompt(PromptOpts{
		ProjectDir:       projectDir,
		ExecutiveContent: "Executive\n",
		UserPrompt:       "Directive",
		SprintPrompt:     "Instructions",
		Promise:          "TOKEN",
	})
	require.NoError(t, err)

	headers := []string{
		"# ===== PROJECT CONTEXT =====",
		"# ===== USER DIRECTIVE =====",
		"# ===== STRATEGIC PLAN =====",
		"# ===== SPRINT INSTRUCTIONS =====",
		"# ===== ITERATION MEMORY =====",
		"# ===== COMPLETION SIGNAL =====",
	}
	for _, header := range headers {
		assert.Contains(t, prompt, header)
	}

	// Without promise, COMPLETION SIGNAL should be absent
	promptNoPromise, err := AssemblePrompt(PromptOpts{
		ProjectDir:       projectDir,
		ExecutiveContent: "Executive\n",
		UserPrompt:       "Directive",
		SprintPrompt:     "Instructions",
		Promise:          "",
	})
	require.NoError(t, err)
	assert.NotContains(t, promptNoPromise, "# ===== COMPLETION SIGNAL =====")
}

func TestInitSprintProgress(t *testing.T) {
	projectDir := t.TempDir()
	err := InitSprintProgress(projectDir, 4, "Sprint Execution")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(projectDir, config.SprintProgressFile))
	require.NoError(t, err)
	assert.Equal(t, "# Sprint 4: Sprint Execution — Progress\n\n", string(content))
}

func TestEpicProgressReset(t *testing.T) {
	assert.True(t, ShouldResetEpicProgress(1, 1, 6, 6))
	assert.False(t, ShouldResetEpicProgress(1, 1, 5, 6))
	assert.False(t, ShouldResetEpicProgress(2, 2, 6, 6))
}

func TestMechanicalCompaction(t *testing.T) {
	progress := strings.Join([]string{
		"# Sprint 4: Sprint Execution — Progress",
		"",
		"## Iteration 1 — Tue Mar 10 22:00 CDT",
		"First pass",
		"",
		"--- Heal attempt 1 failed ---",
		"Heal details",
		"",
		"## Iteration 2 — Tue Mar 10 22:30 CDT",
		"Second pass",
		"",
	}, "\n")

	assert.Equal(t, "## Iteration 2 — Tue Mar 10 22:30 CDT\nSecond pass", mechanicalCompaction(progress))
}

func TestSprintResultStatusStrings(t *testing.T) {
	assert.Equal(t, "PASS", StatusPass)
	assert.Equal(t, "PASS (healed)", StatusPassHealed)
	assert.Equal(t, "PASS (verification passed, no promise)", StatusPassVerificationPassedNoPromise)
	assert.Equal(t, "PASS (healed, no promise)", StatusPassHealedNoPromise)
	assert.Equal(t, "FAIL (verification failed, heal exhausted)", StatusFailVerificationFailedHealExhausted)
	assert.Equal(t, "FAIL (no promise, verification failed, heal exhausted)", StatusFailNoPromiseVerificationHealExhaust)
	assert.Equal(t, "FAIL (no prompt)", StatusFailNoPrompt)
	assert.Equal(t, "SKIPPED", StatusSkipped)
}

func TestPromiseDetection(t *testing.T) {
	output := "agent output\n===PROMISE: TOKEN===\nmore output"
	assert.True(t, strings.Contains(output, "===PROMISE: TOKEN==="))
}

func TestRunSprintPassesWithPromiseAndChecks(t *testing.T) {
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, config.ExecutiveFile), "Executive\n")
	writeFile(t, filepath.Join(projectDir, config.DefaultVerificationFile), "@sprint 1\n@check_file result.txt\n")
	writeFile(t, filepath.Join(projectDir, "result.txt"), "ok\n")

	mockEngine := &stubEngine{
		name:    "codex",
		outputs: []string{"work complete\n===PROMISE: DONE===\n"},
	}

	result, err := RunSprint(context.Background(), RunConfig{
		ProjectDir: projectDir,
		Epic: &epic.Epic{
			TotalSprints:     3,
			VerificationFile: config.DefaultVerificationFile,
			MaxHealAttempts:  1,
			AgentModel:       "",
			AgentFlags:       "",
			PreIterationCmd:  "",
			PreSprintCmd:     "",
		},
		Sprint: &epic.Sprint{
			Number:        1,
			Name:          "Runner",
			MaxIterations: 2,
			Promise:       "DONE",
			Prompt:        "Build it.",
		},
		Engine: mockEngine,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusPass, result.Status)
	assert.Len(t, mockEngine.prompts, 1)
	assert.Equal(t, config.AgentInvocationPrompt, mockEngine.prompts[0])
}

func TestRunSprintFailsWithoutPrompt(t *testing.T) {
	projectDir := t.TempDir()
	mockEngine := &stubEngine{name: "codex"}

	result, err := RunSprint(context.Background(), RunConfig{
		ProjectDir: projectDir,
		Epic: &epic.Epic{
			TotalSprints: 1,
		},
		Sprint: &epic.Sprint{
			Number:        1,
			Name:          "Empty",
			MaxIterations: 1,
			Promise:       "DONE",
			Prompt:        "",
		},
		Engine: mockEngine,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusFailNoPrompt, result.Status)
}

type stubEngine struct {
	name    string
	outputs []string
	prompts []string
}

func (s *stubEngine) Run(_ context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	s.prompts = append(s.prompts, prompt)
	var output string
	if len(s.outputs) > 0 {
		output = s.outputs[0]
		s.outputs = s.outputs[1:]
	}
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(output))
	}
	if opts.Stderr != nil {
		_, _ = opts.Stderr.Write(nil)
	}
	return output, 0, nil
}

func (s *stubEngine) Name() string {
	return s.name
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
