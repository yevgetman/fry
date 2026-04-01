package engine

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RateLimitResult holds details about a detected rate limit.
type RateLimitResult struct {
	Detected   bool
	RetryAfter time.Duration // parsed from output; zero if not found
}

// Rate-limit detection patterns, compiled once at package load.
var (
	rateLimitRe    = regexp.MustCompile(`(?i)rate[_\s-]?limit`)
	http429Re      = regexp.MustCompile(`\b429\b`)
	overloadedRe   = regexp.MustCompile(`(?i)overloaded`)
	tooManyReqRe   = regexp.MustCompile(`(?i)too many requests`)
	throttledRe    = regexp.MustCompile(`(?i)throttl(?:ed|ing)`)
	requestLimitRe = regexp.MustCompile(`(?i)(?:request|usage|burst)[_\s-]?limit`)
	resourceExRe   = regexp.MustCompile(`(?i)resource[_\s-]?exhausted`)
	retryAfterRe   = regexp.MustCompile(`(?i)(?:retry[_\-\s]?after|try again in)[:\s]+(\d+)\s*(ms|msec|millisecond|milliseconds|s|sec|secs|second|seconds|m|min|mins|minute|minutes|h|hr|hrs|hour|hours)?`)
)

// DetectRateLimit inspects engine output and error for rate-limit indicators.
// For the "ollama" engine it always returns not-detected (local models do not
// rate-limit via HTTP 429).
func DetectRateLimit(engineName, output string, err error) RateLimitResult {
	if engineName == "ollama" {
		return RateLimitResult{}
	}

	combined := output
	if err != nil {
		combined += " " + err.Error()
	}

	detected := rateLimitRe.MatchString(combined) ||
		http429Re.MatchString(combined) ||
		overloadedRe.MatchString(combined) ||
		tooManyReqRe.MatchString(combined) ||
		throttledRe.MatchString(combined) ||
		requestLimitRe.MatchString(combined) ||
		resourceExRe.MatchString(combined)

	if !detected {
		return RateLimitResult{}
	}

	var retryAfter time.Duration
	if m := retryAfterRe.FindStringSubmatch(combined); len(m) > 1 {
		if secs, parseErr := strconv.Atoi(m[1]); parseErr == nil {
			retryAfter = time.Duration(secs) * retryAfterUnit(m[2])
		}
	}

	return RateLimitResult{Detected: true, RetryAfter: retryAfter}
}

func retryAfterUnit(unit string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "ms", "msec", "millisecond", "milliseconds":
		return time.Millisecond
	case "m", "min", "mins", "minute", "minutes":
		return time.Minute
	case "h", "hr", "hrs", "hour", "hours":
		return time.Hour
	case "", "s", "sec", "secs", "second", "seconds":
		return time.Second
	default:
		return time.Second
	}
}
