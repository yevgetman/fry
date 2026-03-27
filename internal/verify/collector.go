package verify

import (
	"fmt"
	"strings"
)

func formatCheckLine(result CheckResult, prefix string, includeDetail bool) string {
	switch result.Check.Type {
	case CheckFile:
		return fmt.Sprintf("- %s: File missing or empty: %s", prefix, result.Check.Path)
	case CheckFileContains:
		return fmt.Sprintf("- %s: File '%s' does not contain pattern: %s", prefix, result.Check.Path, result.Check.Pattern)
	case CheckCmd:
		return fmt.Sprintf("- %s: Command failed: %s", prefix, result.Check.Command)
	case CheckCmdOutput:
		if includeDetail {
			return fmt.Sprintf("- %s: Command output mismatch: %s\n  Expected pattern: %s", prefix, result.Check.Command, result.Check.Pattern)
		}
		return fmt.Sprintf("- %s: Command output mismatch: %s (expected pattern: %s)", prefix, result.Check.Command, result.Check.Pattern)
	case CheckTest:
		if includeDetail {
			return fmt.Sprintf("- %s: Test command failed: %s (pass=%d fail=%d skip=%d framework=%s)",
				prefix, result.Check.Command, result.TestPassCount, result.TestFailCount, result.TestSkipCount, result.TestFramework)
		}
		return fmt.Sprintf("- %s: Test command failed: %s (pass=%d fail=%d)",
			prefix, result.Check.Command, result.TestPassCount, result.TestFailCount)
	}
	return ""
}

func CollectFailures(results []CheckResult, passCount, totalCount int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Sanity checks: %d/%d passed.\n\nFailed checks:", passCount, totalCount))

	for _, result := range results {
		if result.Passed {
			continue
		}

		b.WriteString("\n")
		b.WriteString(formatCheckLine(result, "FAILED", true))
		switch result.Check.Type {
		case CheckCmd:
			b.WriteString(fmt.Sprintf("\n  Output (truncated):\n%s", indentLines(truncateLines(result.Output, 20))))
		case CheckCmdOutput:
			b.WriteString(fmt.Sprintf("\n  Actual output (truncated):\n%s", indentLines(truncateLines(result.Output, 10))))
		case CheckTest:
			b.WriteString(fmt.Sprintf("\n  Output (truncated):\n%s", indentLines(truncateLines(result.Output, 20))))
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
		b.WriteString(formatCheckLine(result, "DEFERRED", false))
		b.WriteByte('\n')
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
