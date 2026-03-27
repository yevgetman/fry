package engine

import (
	"regexp"
	"strconv"
	"time"
)

// RateLimitResult holds details about a detected rate limit.
type RateLimitResult struct {
	Detected   bool
	RetryAfter time.Duration // parsed from output; zero if not found
}

// Rate-limit detection patterns, compiled once at package load.
var (
	rateLimitRe  = regexp.MustCompile(`(?i)rate[_\s]?limit`)
	http429Re    = regexp.MustCompile(`\b429\b`)
	overloadedRe = regexp.MustCompile(`(?i)overloaded`)
	tooManyReqRe = regexp.MustCompile(`(?i)too many requests`)
	retryAfterRe = regexp.MustCompile(`(?i)retry[_\-\s]?after[:\s]+(\d+)`)
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
		tooManyReqRe.MatchString(combined)

	if !detected {
		return RateLimitResult{}
	}

	var retryAfter time.Duration
	if m := retryAfterRe.FindStringSubmatch(combined); len(m) > 1 {
		if secs, parseErr := strconv.Atoi(m[1]); parseErr == nil {
			retryAfter = time.Duration(secs) * time.Second
		}
	}

	return RateLimitResult{Detected: true, RetryAfter: retryAfter}
}
