package verify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/textutil"
)

func TestParseVerification(t *testing.T) {
	t.Parallel()

	path := writeVerificationFile(t, `
# comment
@sprint 3
@check_file go.mod
@check_file_contains README.md "hello world"
@check_cmd go test ./...
@check_cmd_output printf 'ready\n' | "^ready$"
`)

	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 4)

	assert.Equal(t, Check{Sprint: 3, Type: CheckFile, Path: "go.mod"}, checks[0])
	assert.Equal(t, Check{Sprint: 3, Type: CheckFileContains, Path: "README.md", Pattern: "hello world"}, checks[1])
	assert.Equal(t, Check{Sprint: 3, Type: CheckCmd, Command: "go test ./..."}, checks[2])
	assert.Equal(t, Check{Sprint: 3, Type: CheckCmdOutput, Command: "printf 'ready\\n'", Pattern: "^ready$"}, checks[3])
}

func TestParseVerificationSkipsComments(t *testing.T) {
	t.Parallel()

	path := writeVerificationFile(t, `

# one
@sprint 1   
  	
# two
@check_file internal/verify/parser.go   
`)

	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, 1, checks[0].Sprint)
	assert.Equal(t, "internal/verify/parser.go", checks[0].Path)
}

func TestParseVerificationMultipleSprints(t *testing.T) {
	t.Parallel()

	path := writeVerificationFile(t, `
@sprint 1
@check_file one.txt
@sprint 2
@check_cmd true
@sprint 1
@check_cmd_output printf 'x\n' | "x"
`)

	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 3)
	assert.Equal(t, 1, checks[0].Sprint)
	assert.Equal(t, 2, checks[1].Sprint)
	assert.Equal(t, 1, checks[2].Sprint)
}

func TestUnquotePattern(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", unquotePattern("\"hello\""))
	assert.Equal(t, "plain", unquotePattern("plain"))
	assert.Equal(t, `say "hi"`, unquotePattern(`say \"hi\"`))
}

func TestUnquotePatternEscapes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", unquotePattern(`"hello"`))
	assert.Equal(t, `\test`, unquotePattern(`\\test`))
	assert.Equal(t, `say "hi"`, unquotePattern(`say \"hi\"`))
}

func TestRunChecksFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "present.txt"), []byte("ok"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "empty.txt"), nil, 0o644))

	checks := []Check{
		{Sprint: 1, Type: CheckFile, Path: "present.txt"},
		{Sprint: 1, Type: CheckFile, Path: "missing.txt"},
		{Sprint: 1, Type: CheckFile, Path: "empty.txt"},
	}

	results, passCount, totalCount := RunChecks(context.Background(), checks, 1, projectDir)
	require.Len(t, results, 3)
	assert.Equal(t, 1, passCount)
	assert.Equal(t, 3, totalCount)
	assert.True(t, results[0].Passed)
	assert.False(t, results[1].Passed)
	assert.False(t, results[2].Passed)
}

func TestRunChecksCmd(t *testing.T) {
	t.Parallel()

	checks := []Check{
		{Sprint: 1, Type: CheckCmd, Command: "true"},
		{Sprint: 1, Type: CheckCmd, Command: "false"},
	}

	results, passCount, totalCount := RunChecks(context.Background(), checks, 1, t.TempDir())
	require.Len(t, results, 2)
	assert.Equal(t, 1, passCount)
	assert.Equal(t, 2, totalCount)
	assert.True(t, results[0].Passed)
	assert.False(t, results[1].Passed)
}

func TestRunChecksNoShortCircuit(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	marker := filepath.Join(projectDir, "marker.txt")

	checks := []Check{
		{Sprint: 1, Type: CheckCmd, Command: "false"},
		{Sprint: 1, Type: CheckCmd, Command: "printf ok > marker.txt"},
		{Sprint: 1, Type: CheckFile, Path: "marker.txt"},
	}

	results, passCount, totalCount := RunChecks(context.Background(), checks, 1, projectDir)
	require.Len(t, results, 3)
	assert.Equal(t, 2, passCount)
	assert.Equal(t, 3, totalCount)
	assert.False(t, results[0].Passed)
	assert.True(t, results[1].Passed)
	assert.True(t, results[2].Passed)
	_, err := os.Stat(marker)
	assert.NoError(t, err)
}

func TestCollectFailures(t *testing.T) {
	t.Parallel()

	cmdOutput := strings.Join([]string{
		"line1", "line2", "line3", "line4", "line5",
		"line6", "line7", "line8", "line9", "line10", "line11",
	}, "\n")

	report := CollectFailures([]CheckResult{
		{Check: Check{Type: CheckFile, Path: "missing.txt"}},
		{Check: Check{Type: CheckFileContains, Path: "go.mod", Pattern: "module x"}},
		{Check: Check{Type: CheckCmd, Command: "go test ./..."}, Output: strings.Repeat("a\n", 25)},
		{Check: Check{Type: CheckCmdOutput, Command: "printf nope", Pattern: "^ok$"}, Output: cmdOutput},
	}, 0, 4)

	assert.Contains(t, report, "Verification: 0/4 checks passed.\n\nFailed checks:")
	assert.Contains(t, report, "- FAILED: File missing or empty: missing.txt")
	assert.Contains(t, report, "- FAILED: File 'go.mod' does not contain pattern: module x")
	assert.Contains(t, report, "- FAILED: Command failed: go test ./...")
	assert.Contains(t, report, "  Output (truncated):")
	assert.Contains(t, report, "- FAILED: Command output mismatch: printf nope")
	assert.Contains(t, report, "  Expected pattern: ^ok$")
	assert.NotContains(t, report, "line11")
}

func TestShellQuote(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "''", textutil.ShellQuote(""))
	assert.Equal(t, "'simple'", textutil.ShellQuote("simple"))
	assert.Equal(t, `'`+`it'\''s`+`'`, textutil.ShellQuote("it's"))
	assert.Equal(t, "'$HOME *.go'", textutil.ShellQuote("$HOME *.go"))
}

func TestRunChecksFileContains(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0o644))

	checks := []Check{
		{Sprint: 1, Type: CheckFileContains, Path: "go.mod", Pattern: "module example"},
		{Sprint: 1, Type: CheckFileContains, Path: "go.mod", Pattern: "^nonexistent$"},
		{Sprint: 1, Type: CheckFileContains, Path: "missing.txt", Pattern: "anything"},
	}

	results, passCount, totalCount := RunChecks(context.Background(), checks, 1, projectDir)
	require.Len(t, results, 3)
	assert.Equal(t, 1, passCount)
	assert.Equal(t, 3, totalCount)
	assert.True(t, results[0].Passed)
	assert.False(t, results[1].Passed)
	assert.False(t, results[2].Passed)
}

func TestRunChecksCmdOutput(t *testing.T) {
	t.Parallel()

	checks := []Check{
		{Sprint: 1, Type: CheckCmdOutput, Command: "echo hello", Pattern: "^hello$"},
		{Sprint: 1, Type: CheckCmdOutput, Command: "echo world", Pattern: "^nope$"},
		{Sprint: 1, Type: CheckCmdOutput, Command: "false", Pattern: "anything"},
	}

	results, passCount, totalCount := RunChecks(context.Background(), checks, 1, t.TempDir())
	require.Len(t, results, 3)
	assert.Equal(t, 1, passCount)
	assert.Equal(t, 3, totalCount)
	assert.True(t, results[0].Passed)
	assert.False(t, results[1].Passed)
	assert.False(t, results[2].Passed)
}

func TestRunChecksCmdOutputTrimsWhitespace(t *testing.T) {
	t.Parallel()

	// Simulate macOS wc -w which outputs leading whitespace (e.g., "     42")
	checks := []Check{
		{Sprint: 1, Type: CheckCmdOutput, Command: "printf '   42\\n'", Pattern: "^42$"},
		{Sprint: 1, Type: CheckCmdOutput, Command: "printf '  hello  \\n  world  \\n'", Pattern: "^world$"},
	}

	results, passCount, totalCount := RunChecks(context.Background(), checks, 1, t.TempDir())
	require.Len(t, results, 2)
	assert.Equal(t, 2, passCount)
	assert.Equal(t, 2, totalCount)
	assert.True(t, results[0].Passed, "leading whitespace should be trimmed before matching")
	assert.True(t, results[1].Passed, "per-line trimming should work across multiple lines")
}

func TestTrimOutputLines(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "42", trimOutputLines("   42"))
	assert.Equal(t, "42\n", trimOutputLines("   42\n"))
	assert.Equal(t, "hello\nworld", trimOutputLines("  hello  \n  world  "))
	assert.Equal(t, "", trimOutputLines(""))
	assert.Equal(t, "ok", trimOutputLines("ok"))
}

func TestEvaluateThresholdNoChecks(t *testing.T) {
	t.Parallel()

	outcome := EvaluateThreshold(nil, 0, 0, 20)
	assert.True(t, outcome.WithinThreshold)
	assert.Equal(t, float64(0), outcome.FailPercent)
	assert.Empty(t, outcome.DeferredFailures)
}

func TestEvaluateThresholdAllPass(t *testing.T) {
	t.Parallel()

	results := []CheckResult{
		{Check: Check{Type: CheckFile, Path: "a.txt"}, Passed: true},
		{Check: Check{Type: CheckFile, Path: "b.txt"}, Passed: true},
	}
	outcome := EvaluateThreshold(results, 2, 2, 20)
	assert.True(t, outcome.WithinThreshold)
	assert.Equal(t, float64(0), outcome.FailPercent)
	assert.Empty(t, outcome.DeferredFailures)
}

func TestEvaluateThresholdWithinThreshold(t *testing.T) {
	t.Parallel()

	results := []CheckResult{
		{Check: Check{Type: CheckFile, Path: "a.txt"}, Passed: true},
		{Check: Check{Type: CheckFile, Path: "b.txt"}, Passed: true},
		{Check: Check{Type: CheckFile, Path: "c.txt"}, Passed: true},
		{Check: Check{Type: CheckFile, Path: "d.txt"}, Passed: true},
		{Check: Check{Type: CheckFile, Path: "e.txt"}, Passed: false},
	}
	// 1 of 5 = 20%, threshold is 20% → within
	outcome := EvaluateThreshold(results, 4, 5, 20)
	assert.True(t, outcome.WithinThreshold)
	assert.InDelta(t, 20.0, outcome.FailPercent, 0.01)
	require.Len(t, outcome.DeferredFailures, 1)
	assert.Equal(t, "e.txt", outcome.DeferredFailures[0].Check.Path)
}

func TestEvaluateThresholdExceedsThreshold(t *testing.T) {
	t.Parallel()

	results := []CheckResult{
		{Check: Check{Type: CheckFile, Path: "a.txt"}, Passed: true},
		{Check: Check{Type: CheckFile, Path: "b.txt"}, Passed: false},
		{Check: Check{Type: CheckFile, Path: "c.txt"}, Passed: false},
	}
	// 2 of 3 = 66.7%, threshold 20% → exceeds
	outcome := EvaluateThreshold(results, 1, 3, 20)
	assert.False(t, outcome.WithinThreshold)
	assert.InDelta(t, 66.67, outcome.FailPercent, 0.01)
	assert.Empty(t, outcome.DeferredFailures)
}

func TestEvaluateThresholdZeroPercent(t *testing.T) {
	t.Parallel()

	results := []CheckResult{
		{Check: Check{Type: CheckFile, Path: "a.txt"}, Passed: true},
		{Check: Check{Type: CheckFile, Path: "b.txt"}, Passed: false},
	}
	// strict mode: 0% threshold, any failure exceeds
	outcome := EvaluateThreshold(results, 1, 2, 0)
	assert.False(t, outcome.WithinThreshold)
}

func TestEvaluateThresholdHundredPercent(t *testing.T) {
	t.Parallel()

	results := []CheckResult{
		{Check: Check{Type: CheckFile, Path: "a.txt"}, Passed: false},
		{Check: Check{Type: CheckFile, Path: "b.txt"}, Passed: false},
	}
	// 100% threshold → always within
	outcome := EvaluateThreshold(results, 0, 2, 100)
	assert.True(t, outcome.WithinThreshold)
	require.Len(t, outcome.DeferredFailures, 2)
}

func TestCollectDeferredSummary(t *testing.T) {
	t.Parallel()

	deferred := []CheckResult{
		{Check: Check{Type: CheckFile, Path: "missing.txt"}},
		{Check: Check{Type: CheckFileContains, Path: "go.mod", Pattern: "module x"}},
		{Check: Check{Type: CheckCmd, Command: "go test ./..."}},
		{Check: Check{Type: CheckCmdOutput, Command: "echo nope", Pattern: "^ok$"}},
	}
	summary := CollectDeferredSummary(deferred)
	assert.Contains(t, summary, "DEFERRED: File missing or empty: missing.txt")
	assert.Contains(t, summary, "DEFERRED: File 'go.mod' does not contain pattern: module x")
	assert.Contains(t, summary, "DEFERRED: Command failed: go test ./...")
	assert.Contains(t, summary, "DEFERRED: Command output mismatch: echo nope (expected pattern: ^ok$)")
}

func writeVerificationFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "verification.md")
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimLeft(contents, "\n")), 0o644))
	return path
}
