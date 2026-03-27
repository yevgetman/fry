package engine

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDetectRateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		engine     string
		output     string
		err        error
		wantDetect bool
		wantRA     time.Duration
	}{
		{
			name:       "claude rate_limit in output",
			engine:     "claude",
			output:     "Error: rate_limit exceeded",
			wantDetect: true,
		},
		{
			name:       "claude rate limit with space",
			engine:     "claude",
			output:     "Error: rate limit exceeded",
			wantDetect: true,
		},
		{
			name:       "codex HTTP 429",
			engine:     "codex",
			output:     "HTTP 429 Too Many Requests",
			wantDetect: true,
		},
		{
			name:       "overloaded_error",
			engine:     "claude",
			output:     "overloaded_error: server is busy",
			wantDetect: true,
		},
		{
			name:       "too many requests",
			engine:     "codex",
			output:     "Error: too many requests",
			wantDetect: true,
		},
		{
			name:       "retry-after header parsed",
			engine:     "claude",
			output:     "rate_limit retry-after: 30",
			wantDetect: true,
			wantRA:     30 * time.Second,
		},
		{
			name:       "retry_after underscore parsed",
			engine:     "claude",
			output:     "rate limit hit, retry_after: 15",
			wantDetect: true,
			wantRA:     15 * time.Second,
		},
		{
			name:       "no match on normal output",
			engine:     "claude",
			output:     "Successfully processed 500 tokens",
			wantDetect: false,
		},
		{
			name:       "429 in larger number is not matched",
			engine:     "claude",
			output:     "processed 1429 tokens",
			wantDetect: false,
		},
		{
			name:       "ollama always skipped",
			engine:     "ollama",
			output:     "rate_limit 429 overloaded",
			wantDetect: false,
		},
		{
			name:       "rate limit in error string only",
			engine:     "claude",
			output:     "",
			err:        fmt.Errorf("rate limit exceeded"),
			wantDetect: true,
		},
		{
			name:       "overloaded in error string",
			engine:     "codex",
			output:     "",
			err:        fmt.Errorf("server overloaded"),
			wantDetect: true,
		},
		{
			name:       "non-rate-limit error",
			engine:     "claude",
			output:     "",
			err:        fmt.Errorf("connection refused"),
			wantDetect: false,
		},
		{
			name:       "empty output and nil error",
			engine:     "claude",
			output:     "",
			err:        nil,
			wantDetect: false,
		},
		{
			name:       "429 at word boundary start",
			engine:     "claude",
			output:     "429 rate limit",
			wantDetect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := DetectRateLimit(tt.engine, tt.output, tt.err)
			assert.Equal(t, tt.wantDetect, result.Detected, "Detected mismatch")
			assert.Equal(t, tt.wantRA, result.RetryAfter, "RetryAfter mismatch")
		})
	}
}
