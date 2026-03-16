package verify

import (
	"fmt"
	"strings"
)

func CollectFailures(results []CheckResult, passCount, totalCount int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Verification: %d/%d checks passed.\n\nFailed checks:", passCount, totalCount))

	for _, result := range results {
		if result.Passed {
			continue
		}

		switch result.Check.Type {
		case CheckFile:
			b.WriteString(fmt.Sprintf("\n- FAILED: File missing or empty: %s", result.Check.Path))
		case CheckFileContains:
			b.WriteString(fmt.Sprintf("\n- FAILED: File '%s' does not contain pattern: %s", result.Check.Path, result.Check.Pattern))
		case CheckCmd:
			b.WriteString(fmt.Sprintf("\n- FAILED: Command failed: %s\n  Output (truncated):\n%s", result.Check.Command, indentLines(truncateLines(result.Output, 20))))
		case CheckCmdOutput:
			b.WriteString(fmt.Sprintf("\n- FAILED: Command output mismatch: %s\n  Expected pattern: %s\n  Actual output (truncated):\n%s", result.Check.Command, result.Check.Pattern, indentLines(truncateLines(result.Output, 10))))
		}
	}

	return b.String()
}

func EvaluateThreshold(results []CheckResult, passCount, totalCount, maxFailPercent int) VerificationOutcome {
	outcome := VerificationOutcome{
		Results:    results,
		PassCount:  passCount,
		TotalCount: totalCount,
	}

	if totalCount == 0 {
		outcome.WithinThreshold = true
		return outcome
	}

	failCount := totalCount - passCount
	outcome.FailPercent = float64(failCount) / float64(totalCount) * 100

	// Use integer arithmetic to avoid floating point precision issues.
	// e.g., 1/5 = 20.000...004 in float64, which would exceed integer 20.
	outcome.WithinThreshold = failCount*100 <= maxFailPercent*totalCount

	if outcome.WithinThreshold && failCount > 0 {
		for _, r := range results {
			if !r.Passed {
				outcome.DeferredFailures = append(outcome.DeferredFailures, r)
			}
		}
	}

	return outcome
}

func CollectDeferredSummary(deferred []CheckResult) string {
	var b strings.Builder
	for _, result := range deferred {
		switch result.Check.Type {
		case CheckFile:
			b.WriteString(fmt.Sprintf("- DEFERRED: File missing or empty: %s\n", result.Check.Path))
		case CheckFileContains:
			b.WriteString(fmt.Sprintf("- DEFERRED: File '%s' does not contain pattern: %s\n", result.Check.Path, result.Check.Pattern))
		case CheckCmd:
			b.WriteString(fmt.Sprintf("- DEFERRED: Command failed: %s\n", result.Check.Command))
		case CheckCmdOutput:
			b.WriteString(fmt.Sprintf("- DEFERRED: Command output mismatch: %s (expected pattern: %s)\n", result.Check.Command, result.Check.Pattern))
		}
	}
	return b.String()
}

func truncateLines(s string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return "  "
	}
	if len(lines) > maxLines {
		omitted := len(lines) - maxLines
		lines = append(lines[:maxLines], fmt.Sprintf("... (%d more lines)", omitted))
	}
	return strings.Join(lines, "\n")
}

func indentLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
