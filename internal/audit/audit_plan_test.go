package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	frygit "github.com/yevgetman/fry/internal/git"
	"github.com/yevgetman/fry/internal/review"
)

func TestBuildVerifyPromptRequestsNotes(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	prompt := buildVerifyPrompt(opts, []Finding{{Description: "Issue A", Severity: "HIGH"}})

	assert.Contains(t, prompt, "**Notes:**")
}

func TestAuditPromptIncludesReconciliationDirective(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	opts.Mode = "writing"
	opts.Complexity = ComplexityHigh

	prompt := buildAuditPrompt(opts, nil, nil)
	assert.Contains(t, prompt, "Priority Check: Figure Reconciliation")
	assert.Contains(t, prompt, "Trace each claim to its source calculation")
}

func TestAuditPromptIncludesRelevantDeviations(t *testing.T) {
	t.Parallel()

	opts := makeOpts(t, &stubEngine{name: "codex"})
	require.NoError(t, review.AppendDeviationLog(opts.ProjectDir, review.DeviationLogEntry{
		SprintNum:       1,
		SprintName:      "Setup",
		Verdict:         review.VerdictDeviate,
		Trigger:         "The pricing appendix remains authoritative over the summary.",
		Impact:          "- Preserve the appendix numbers.\n",
		RiskAssessment:  "Low risk with a reconciliation note.",
		AffectedSprints: []int{1},
	}))

	prompt := buildAuditPrompt(opts, nil, nil)
	assert.Contains(t, prompt, "Known Intentional Divergences")
	assert.Contains(t, prompt, "pricing appendix remains authoritative")
}

func TestRunAuditLoopSkipsVerifyOnNoOp(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			switch callIndex {
			case 0, 3:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), highFindings)
			}
		},
	}
	opts := makeOpts(t, eng)
	initAuditGitRepo(t, opts.ProjectDir)
	opts.Epic.MaxAuditIterations = 1

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, result.Passed)
	for _, prompt := range eng.prompts {
		assert.NotEqual(t, config.AuditVerifyInvocationPrompt, prompt)
	}
}

func TestRunAuditLoopVerifiesAlreadyFixedClaimOnNoOp(t *testing.T) {
	t.Parallel()

	eng := &stubEngine{
		name: "codex",
		outputs: []string{
			"",
			"No changes needed. This issue is already fixed.",
			"",
			"",
		},
		sideEffect: func(projectDir string, callIndex int) {
			switch callIndex {
			case 0:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), highFindings)
			case 2:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), "- **Issue:** 1\n- **Status:** RESOLVED\n- **Notes:** guard already present in the current code path\n")
			case 3:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), cleanAudit)
			}
		},
	}
	opts := makeOpts(t, eng)
	initAuditGitRepo(t, opts.ProjectDir)
	opts.Epic.MaxAuditIterations = 1

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	require.True(t, result.Passed)
	require.NotNil(t, result.Metrics)
	require.Len(t, result.Metrics.Calls, 4)
	assert.Equal(t, fixValidationVerifyOnly, result.Metrics.Calls[1].ValidationResult)
	assert.True(t, result.Metrics.Calls[1].AlreadyFixedClaim)
	assert.Contains(t, eng.prompts, config.AuditVerifyInvocationPrompt)
}

func TestRunAuditLoopRejectsCommentOnlyFixBeforeVerify(t *testing.T) {
	t.Parallel()

	findings := "## Findings\n- **Location:** tracked.go:1\n- **Description:** Missing error handling\n- **Severity:** HIGH\n- **Recommended Fix:** Add a nil guard\n\n## Verdict\nFAIL\n"
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			switch callIndex {
			case 0, 3:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), findings)
			case 1, 2:
				require.NoError(t, os.WriteFile(filepath.Join(projectDir, "tracked.go"), []byte("package main\n\n// comment only\n"), 0o644))
			}
		},
	}
	opts := makeOpts(t, eng)
	writeFile(t, filepath.Join(opts.ProjectDir, "tracked.go"), "package main\n")
	require.NoError(t, frygit.InitGit(context.Background(), opts.ProjectDir))
	opts.Epic.MaxAuditIterations = 1

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	require.False(t, result.Passed)
	require.NotNil(t, result.Metrics)
	assert.Equal(t, 2, result.Metrics.Snapshot().RejectedFixCalls)
	for _, prompt := range eng.prompts {
		assert.NotEqual(t, config.AuditVerifyInvocationPrompt, prompt)
	}
}

func TestRunAuditLoopFixHistoryIntegration(t *testing.T) {
	t.Parallel()

	var secondFixPrompt string
	findings := "## Findings\n- **Location:** tracked.txt:1\n- **Description:** Missing error handling\n- **Severity:** HIGH\n- **Recommended Fix:** Add a nil guard\n\n## Verdict\nFAIL\n"
	eng := &stubEngine{
		name: "codex",
		sideEffect: func(projectDir string, callIndex int) {
			switch callIndex {
			case 0:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), findings)
			case 1:
				writeFile(t, filepath.Join(projectDir, "tracked.txt"), "first fix\n")
			case 2:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), "- **Issue:** 1\n- **Status:** STILL PRESENT\n- **Notes:** nil check added but the conditional is inverted\n")
			case 3:
				data, err := os.ReadFile(filepath.Join(projectDir, config.AuditPromptFile))
				require.NoError(t, err)
				secondFixPrompt = string(data)
				writeFile(t, filepath.Join(projectDir, "tracked.txt"), "second fix\n")
			case 4:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), "- **Issue:** 1\n- **Status:** RESOLVED\n- **Notes:** nil check now guards the panic path\n")
			case 5:
				writeFile(t, filepath.Join(projectDir, config.SprintAuditFile), cleanAudit)
			}
		},
	}
	opts := makeOpts(t, eng)
	initAuditGitRepo(t, opts.ProjectDir)
	opts.Epic.MaxAuditIterations = 1
	opts.Complexity = ComplexityModerate

	result, err := RunAuditLoop(context.Background(), opts)
	require.NoError(t, err)
	require.True(t, result.Passed)
	assert.Contains(t, secondFixPrompt, "Previous Fix Attempts")
	assert.Contains(t, secondFixPrompt, "conditional is inverted")
	assert.Equal(t, ComplexityModerate, result.Complexity)
	require.NotNil(t, result.Metrics)
	assert.GreaterOrEqual(t, result.Metrics.TotalCalls(), 4)
}

func initAuditGitRepo(t *testing.T, projectDir string) {
	t.Helper()
	writeFile(t, filepath.Join(projectDir, "tracked.txt"), "base\n")
	require.NoError(t, frygit.InitGit(context.Background(), projectDir))
}
