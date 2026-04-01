package engine

import (
	"context"
	"errors"
	"regexp"
)

// FailoverResult describes whether an engine failure is worth failing over.
type FailoverResult struct {
	Detected bool
	Reason   string
}

var (
	http5xxRe            = regexp.MustCompile(`\b(500|502|503|504)\b`)
	serviceUnavailableRe = regexp.MustCompile(`(?i)service unavailable|temporarily unavailable|unavailable`)
	connectionResetRe    = regexp.MustCompile(`(?i)connection reset|econnreset|broken pipe`)
	connectionRefusedRe  = regexp.MustCompile(`(?i)connection refused|econnrefused`)
	timeoutRe            = regexp.MustCompile(`(?i)i/o timeout|timed out|timeout|deadline exceeded`)
	networkErrorRe       = regexp.MustCompile(`(?i)network error|transport error|upstream connect error|no healthy upstream`)
	quotaExceededRe      = regexp.MustCompile(`(?i)quota exceeded|exceeded your current quota|credit balance too low|insufficient credits|billing hard limit|usage limit reached`)
)

// DetectFailoverCondition inspects an engine failure and determines whether
// retrying on another engine is reasonable. It deliberately excludes caller
// cancellation and deterministic configuration errors.
func DetectFailoverCondition(engineName, output string, err error) FailoverResult {
	if err == nil {
		return FailoverResult{}
	}
	if errors.Is(err, context.Canceled) {
		return FailoverResult{}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return FailoverResult{Detected: true, Reason: "timeout"}
	}
	if rl := DetectRateLimit(engineName, output, err); rl.Detected {
		return FailoverResult{Detected: true, Reason: "rate limit"}
	}

	combined := output + " " + err.Error()
	switch {
	case quotaExceededRe.MatchString(combined):
		return FailoverResult{Detected: true, Reason: "provider quota"}
	case http5xxRe.MatchString(combined):
		return FailoverResult{Detected: true, Reason: "server error"}
	case serviceUnavailableRe.MatchString(combined):
		return FailoverResult{Detected: true, Reason: "service unavailable"}
	case connectionResetRe.MatchString(combined):
		return FailoverResult{Detected: true, Reason: "connection reset"}
	case connectionRefusedRe.MatchString(combined):
		return FailoverResult{Detected: true, Reason: "connection refused"}
	case timeoutRe.MatchString(combined):
		return FailoverResult{Detected: true, Reason: "timeout"}
	case networkErrorRe.MatchString(combined):
		return FailoverResult{Detected: true, Reason: "network error"}
	default:
		return FailoverResult{}
	}
}
