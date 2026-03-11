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

func truncateLines(s string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return "  "
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
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
