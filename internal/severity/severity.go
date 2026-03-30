// Package severity provides shared severity level utilities used by multiple
// packages that parse audit and dependency scan output.
package severity

// Rank maps a severity level string to an integer for comparison.
// Higher values indicate greater severity.
// Known levels: CRITICAL=4, HIGH=3, MODERATE=2, LOW=1. Unknown input returns 0.
func Rank(sev string) int {
	switch sev {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MODERATE":
		return 2
	case "LOW":
		return 1
	default:
		return 0
	}
}
