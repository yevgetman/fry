package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
)

// --- stub engine ---

type stubEngine struct {
	name    string
	outputs []string
	prompts []string
	// sideEffect is called during Run to simulate agent behavior (e.g. writing files)
	sideEffect func(projectDir string, callIndex int)
	callIndex  int
}

func (s *stubEngine) Run(_ context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	s.prompts = append(s.prompts, prompt)
	var output string
	if len(s.outputs) > 0 {
		output = s.outputs[0]
		s.outputs = s.outputs[1:]
	}
	if s.sideEffect != nil {
		s.sideEffect(opts.WorkDir, s.callIndex)
	}
	s.callIndex++
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(output))
	}
	return output, 0, nil
}

func (s *stubEngine) Name() string {
	return s.name
}

// --- helpers ---

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func makeOpts(t *testing.T, eng engine.Engine) AuditOpts {
	t.Helper()
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, config.SprintProgressFile), "Built the feature.\n")
	return AuditOpts{
		ProjectDir: projectDir,
		Sprint: &epic.Sprint{
			Number:        1,
			Name:          "Setup",
			MaxIterations: 2,
			Promise:       "DONE",
			Prompt:        "Build the setup sprint.",
		},
		Epic: &epic.Epic{
			TotalSprints:       3,
			AuditAfterSprint:   true,
			MaxAuditIterations: 3,
		},
		Engine:  eng,
		GitDiff: "+new line\n-old line\n",
		Verbose: false,
	}
}

// --- tests ---

func TestAuditPromptContainsDiff(t *testing.T) {
	opts := makeOpts(t, &stubEngine{name: "codex"})
	prompt := buildAuditPrompt(opts)
	assert.Contains(t, prompt, "+new line")
	assert.Contains(t, prompt, "-old line")
}

func TestAuditPromptWritingMode(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	opts.Mode = "writing"
	prompt := buildAuditPrompt(opts)
	assert.Contains(t, prompt, "content auditor")
	assert.Contains(t, prompt, "Coherence")
	assert.Contains(t, prompt, "Tone & Voice")
	assert.Contains(t, prompt, "Depth")
	assert.NotContains(t, prompt, "code auditor")
	assert.NotContains(t, prompt, "Security")
}

func TestAuditFixPromptWritingMode(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	opts.Mode = "writing"
	prompt := buildAuditFixPrompt(opts, "## Findings\n- weak transition\n")
	assert.Contains(t, prompt, "content audit found issues")
	assert.Contains(t, prompt, "minimal editorial changes")
}

func TestAuditPromptCondensesExecutive(t *testing.T) {
	opts := makeOpts(t, &stubEngine{name: "codex"})
	// Write a long executive file
	long := make([]byte, 3000)
	for i := range long {
		long[i] = 'x'
	}
	writeFile(t, filepath.Join(opts.ProjectDir, config.ExecutiveFile), string(long))

	prompt := buildAuditPrompt(opts)
	assert.Contains(t, prompt, "...(truncated)")
	// Should contain at most 2000 chars of executive + truncation notice
	assert.Contains(t, prompt, "## Project Context")
}

func TestAuditPromptTruncatesSprintProgress(t *testing.T) {
	opts := makeOpts(t, &stubEngine{name: "codex"})
	// Write a large sprint progress file (>50KB)
	large := make([]byte, 60000)
	for i := range large {
		large[i] = 'y'
	}
	writeFile(t, filepath.Join(opts.ProjectDir, config.SprintProgressFile), string(large))

	prompt := buildAuditPrompt(opts)
	assert.Contains(t, prompt, "...(sprint progress truncated at 50KB)")
	assert.Contains(t, prompt, "## What Was Done")
}

func TestAuditFixPromptReferencesFindings(t *testing.T) {
	opts := makeOpts(t, &stubEngine{name: "codex"})
	findings := "## Findings\n- **Severity:** CRITICAL\n- Missing null check\n"
	prompt := buildAuditFixPrompt(opts, findings)
	assert.Contains(t, prompt, "CRITICAL")
	assert.Contains(t, prompt, "Missing null check")
	assert.Contains(t, prompt, config.SprintProgressFile)
	assert.Contains(t, prompt, config.PlanFile)
}

func TestParseAuditSeverity(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"## Findings\n- **Severity:** CRITICAL\n", "CRITICAL"},
		{"Severity: HIGH\nSeverity: MODERATE\n", "HIGH"},
		{"- **Severity:** MODERATE\nedge case\n", "MODERATE"},
		{"- **Severity:** LOW\nstyle issue\n", "LOW"},
		{"## Verdict\nPASS\n", ""},
		{"No issues found.", ""},
		// Words outside severity-labeled lines should NOT match
		{"CRITICAL bug found here", ""},
		{"This is HIGH priority work", ""},
		// Multiple severity lines: highest wins
		{"- **Severity:** LOW\n- **Severity:** HIGH\n- **Severity:** MODERATE\n", "HIGH"},
		{"Severity: CRITICAL\nSeverity: LOW\n", "CRITICAL"},
		// Substrings of severity keywords should NOT match (word-boundary check)
		{"**Severity:** LOW — HIGHLY unusual but cosmetic\n", "LOW"},
		{"**Severity:** LOW — HIGHLIGHTED concern\n", "LOW"},
		{"**Severity:** LOW — CRITICALLY important style\n", "LOW"},
		{"**Severity:** MODERATE — ALLOW this pattern\n", "MODERATE"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, parseAuditSeverity(tt.content), "content: %q", tt.content)
	}
}

func TestIsAuditPass(t *testing.T) {
	assert.True(t, isAuditPass(""))
	assert.True(t, isAuditPass("LOW"))
	assert.False(t, isAuditPass("MODERATE"))
	assert.False(t, isAuditPass("HIGH"))
	assert.False(t, isAuditPass("CRITICAL"))
}

func TestIsBlockingSeverity(t *testing.T) {
	assert.True(t, isBlockingSeverity("CRITICAL"))
	assert.True(t, isBlockingSeverity("HIGH"))
	assert.False(t, isBlockingSeverity("MODERATE"))
	assert.False(t, isBlockingSeverity("LOW"))
	assert.False(t, isBlockingSeverity(""))
}

func TestCleanup(t *testing.T) {
	projectDir := t.TempDir()

	// Create files to clean up
	writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), "findings\n")
	writeFile(t, filepath.Join(projectDir, config.AuditPromptFile), "prompt\n")

	require.NoError(t, Cleanup(projectDir))

	_, err := os.Stat(filepath.Join(projectDir, config.SprintAuditFile))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(projectDir, config.AuditPromptFile))
	assert.True(t, os.IsNotExist(err))
}

func TestCleanupMissingFiles(t *testing.T) {
	projectDir := t.TempDir()
	// Should not error when files don't exist
	require.NoError(t, Cleanup(projectDir))
}

func TestRunAuditLoopPassesImmediately(t *testing.T) {
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			// Audit agent writes a clean audit file
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				"## Summary\nAll good.\n\n## Findings\nNone.\n\n## Verdict\nPASS\n")
		},
	}
	opts := makeOpts(t, eng)

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 1, result.Iterations)
	// Only audit agent should have been called, not fix agent
	assert.Len(t, eng.prompts, 1)
	assert.Equal(t, config.AuditInvocationPrompt, eng.prompts[0])
}

func TestRunAuditLoopExhaustsCritical(t *testing.T) {
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			// Always write CRITICAL findings
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				"## Summary\nBad stuff.\n\n## Findings\n- **Severity:** CRITICAL\n\n## Verdict\nFAIL\n")
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 2

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, 2, result.Iterations)
	assert.Equal(t, "CRITICAL", result.MaxSeverity)
	// 2 audit + 2 fix + 1 final audit = 5 agent calls
	assert.Len(t, eng.prompts, 5)
}

func TestRunAuditLoopExhaustsModerateAdvisory(t *testing.T) {
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				"## Summary\nMinor issues.\n\n## Findings\n- **Severity:** MODERATE\n\n## Verdict\nFAIL\n")
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 2

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.False(t, result.Blocking)
	assert.Equal(t, 2, result.Iterations)
	assert.Equal(t, "MODERATE", result.MaxSeverity)
}

func TestRunAuditLoopExhaustsHighBlocking(t *testing.T) {
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				"## Summary\nBugs found.\n\n## Findings\n- **Severity:** HIGH\n\n## Verdict\nFAIL\n")
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.MaxAuditIterations = 2

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, 2, result.Iterations)
	assert.Equal(t, "HIGH", result.MaxSeverity)
}

func TestRunAuditLoopNoFindingsFile(t *testing.T) {
	// Agent doesn't write any file — treat as pass
	eng := &stubEngine{name: "codex"}
	opts := makeOpts(t, eng)

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 1, result.Iterations)
}

func TestRunAuditLoopContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	eng := &stubEngine{name: "codex"}
	opts := makeOpts(t, eng)

	_, err := RunAuditLoop(ctx, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunAuditLoopNilEpic(t *testing.T) {
	_, err := RunAuditLoop(context.Background(), AuditOpts{
		Engine: &stubEngine{name: "codex"},
		Sprint: &epic.Sprint{Number: 1, Name: "One"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "epic and sprint are required")
}

func TestRunAuditLoopNilEngine(t *testing.T) {
	_, err := RunAuditLoop(context.Background(), AuditOpts{
		Epic:   &epic.Epic{},
		Sprint: &epic.Sprint{Number: 1, Name: "One"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

// --- extractFindings tests ---

func TestExtractFindings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected map[string]struct{}
	}{
		{
			name: "single finding",
			content: "## Findings\n- **Description:** Missing null check\n- **Severity:** HIGH\n",
			expected: map[string]struct{}{"missing null check": {}},
		},
		{
			name: "multiple findings",
			content: "- **Description:** SQL injection risk\n- **Description:** Missing auth check\n",
			expected: map[string]struct{}{
				"sql injection risk":  {},
				"missing auth check":  {},
			},
		},
		{
			name:     "no findings",
			content:  "## Summary\nAll good.\n## Verdict\nPASS\n",
			expected: map[string]struct{}{},
		},
		{
			name: "case insensitive label",
			content: "- **description:** Unused variable\n",
			expected: map[string]struct{}{"unused variable": {}},
		},
		{
			name: "plain format without bold",
			content: "- Description: Buffer overflow\n",
			expected: map[string]struct{}{"buffer overflow": {}},
		},
		{
			name: "duplicate descriptions deduplicated",
			content: "- **Description:** Same issue\n- **Description:** Same issue\n",
			expected: map[string]struct{}{"same issue": {}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractFindings(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- hasProgress tests ---

func TestHasProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		previous map[string]struct{}
		current  map[string]struct{}
		expected bool
	}{
		{
			name:     "current empty — all resolved",
			previous: map[string]struct{}{"a": {}},
			current:  map[string]struct{}{},
			expected: true,
		},
		{
			name:     "previous empty — first iteration",
			previous: map[string]struct{}{},
			current:  map[string]struct{}{"a": {}},
			expected: true,
		},
		{
			name:     "both empty",
			previous: map[string]struct{}{},
			current:  map[string]struct{}{},
			expected: true,
		},
		{
			name:     "identical findings — no progress",
			previous: map[string]struct{}{"a": {}, "b": {}},
			current:  map[string]struct{}{"a": {}, "b": {}},
			expected: false,
		},
		{
			name:     "fewer findings — progress",
			previous: map[string]struct{}{"a": {}, "b": {}, "c": {}},
			current:  map[string]struct{}{"a": {}, "b": {}},
			expected: true,
		},
		{
			name:     "different findings — progress",
			previous: map[string]struct{}{"a": {}, "b": {}},
			current:  map[string]struct{}{"c": {}, "d": {}},
			expected: true,
		},
		{
			name:     "superset of previous — no progress",
			previous: map[string]struct{}{"a": {}, "b": {}},
			current:  map[string]struct{}{"a": {}, "b": {}, "c": {}},
			expected: false,
		},
		{
			name:     "partial overlap with new — progress",
			previous: map[string]struct{}{"a": {}, "b": {}},
			current:  map[string]struct{}{"a": {}, "c": {}},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, hasProgress(tt.previous, tt.current))
		})
	}
}

// --- effectiveMaxIter tests ---

func TestEffectiveMaxIter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		epic          *epic.Epic
		wantMax       int
		wantProgress  bool
	}{
		{
			name:         "medium effort default",
			epic:         &epic.Epic{EffortLevel: epic.EffortMedium, MaxAuditIterations: 3},
			wantMax:      3,
			wantProgress: false,
		},
		{
			name:         "high effort not explicitly set",
			epic:         &epic.Epic{EffortLevel: epic.EffortHigh, MaxAuditIterations: 3},
			wantMax:      config.MaxAuditIterationsSafetyCap,
			wantProgress: true,
		},
		{
			name:         "max effort not explicitly set",
			epic:         &epic.Epic{EffortLevel: epic.EffortMax, MaxAuditIterations: 3},
			wantMax:      config.MaxAuditIterationsSafetyCap,
			wantProgress: true,
		},
		{
			name:         "high effort explicitly set",
			epic:         &epic.Epic{EffortLevel: epic.EffortHigh, MaxAuditIterations: 5, MaxAuditIterationsSet: true},
			wantMax:      5,
			wantProgress: false,
		},
		{
			name:         "low effort",
			epic:         &epic.Epic{EffortLevel: epic.EffortLow, MaxAuditIterations: 3},
			wantMax:      3,
			wantProgress: false,
		},
		{
			name:         "unset effort",
			epic:         &epic.Epic{MaxAuditIterations: 3},
			wantMax:      3,
			wantProgress: false,
		},
		{
			name:         "unset effort zero iterations",
			epic:         &epic.Epic{},
			wantMax:      config.DefaultMaxAuditIterations,
			wantProgress: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			maxIter, progressBased := effectiveMaxIter(tt.epic)
			assert.Equal(t, tt.wantMax, maxIter)
			assert.Equal(t, tt.wantProgress, progressBased)
		})
	}
}

// --- progress-based loop tests ---

func TestRunAuditLoopProgressStopsOnStale(t *testing.T) {
	t.Parallel()

	// Stub engine always returns the same CRITICAL findings with same description.
	// At high effort with progress-based mode, should stop after:
	// pass 1: baseline (runs fix), pass 2: stale #1 (runs fix), pass 3: stale #2 (stops)
	// Then final audit pass.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				"## Findings\n- **Description:** Null pointer dereference\n- **Severity:** CRITICAL\n\n## Verdict\nFAIL\n")
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortHigh
	opts.Epic.MaxAuditIterationsSet = false

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, "CRITICAL", result.MaxSeverity)
	// pass 1: audit+fix, pass 2: audit+fix, pass 3: audit (stale#2, break), final audit
	// = 3 audits + 2 fixes + 1 final = 6 agent calls
	assert.Len(t, eng.prompts, 6)
}

func TestRunAuditLoopProgressContinues(t *testing.T) {
	t.Parallel()

	// Stub engine returns different findings each time — progress is always made.
	// Should continue beyond the default 3 iterations.
	callCount := 0
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			callCount++
			desc := fmt.Sprintf("Issue number %d", callIndex)
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				fmt.Sprintf("## Findings\n- **Description:** %s\n- **Severity:** HIGH\n\n## Verdict\nFAIL\n", desc))
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortMax
	opts.Epic.MaxAuditIterationsSet = false

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	// Should have run all 10 iterations (safety cap) + final audit
	// = 10 audits + 10 fixes + 1 final = 21 agent calls
	assert.Len(t, eng.prompts, 21)
}

func TestRunAuditLoopExplicitCapAtHighEffort(t *testing.T) {
	t.Parallel()

	// User explicitly set @max_audit_iterations 2 at high effort.
	// Should respect the cap without progress detection.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				"## Findings\n- **Description:** Always the same\n- **Severity:** HIGH\n\n## Verdict\nFAIL\n")
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortHigh
	opts.Epic.MaxAuditIterations = 2
	opts.Epic.MaxAuditIterationsSet = true

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.True(t, result.Blocking)
	assert.Equal(t, 2, result.Iterations)
	// 2 audit + 2 fix + 1 final = 5 (same as bounded behavior)
	assert.Len(t, eng.prompts, 5)
}

func TestRunAuditLoopMediumEffortBounded(t *testing.T) {
	t.Parallel()

	// Medium effort: should use bounded behavior (3 iterations), no progress detection.
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				"## Findings\n- **Description:** Same issue\n- **Severity:** MODERATE\n\n## Verdict\nFAIL\n")
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortMedium
	opts.Epic.MaxAuditIterations = 3

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.False(t, result.Blocking)
	assert.Equal(t, 3, result.Iterations)
	// 3 audit + 3 fix + 1 final = 7 (bounded, no early stop from progress detection)
	assert.Len(t, eng.prompts, 7)
}

func TestRunAuditLoopProgressResetsStaleCount(t *testing.T) {
	t.Parallel()

	// Alternating: stale, progress, stale, progress — should not trigger stale stop.
	iteration := 0
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			iteration++
			var desc string
			if iteration%2 == 0 {
				desc = "recurring issue"
			} else {
				desc = fmt.Sprintf("new issue %d", iteration)
			}
			writeFile(t, filepath.Join(projectDir, config.SprintAuditFile),
				fmt.Sprintf("## Findings\n- **Description:** %s\n- **Severity:** HIGH\n\n## Verdict\nFAIL\n", desc))
		},
	}
	opts := makeOpts(t, eng)
	opts.Epic.EffortLevel = epic.EffortHigh
	opts.Epic.MaxAuditIterationsSet = false

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	// Should run all 10 iterations (progress resets prevent early stop)
	assert.Len(t, eng.prompts, 21) // 10 audit + 10 fix + 1 final
}
