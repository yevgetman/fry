package color

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestColorState groups tests that mutate the package-level enabled flag.
// Subtests run sequentially to avoid logical interference.
func TestColorState(t *testing.T) {
	t.Parallel()

	t.Run("SetEnabled", func(t *testing.T) {
		SetEnabled(true)
		assert.True(t, Enabled())

		SetEnabled(false)
		assert.False(t, Enabled())
	})

	t.Run("Colorize_disabled", func(t *testing.T) {
		SetEnabled(false)
		got := Colorize("hello", Red)
		assert.Equal(t, "hello", got)
	})

	t.Run("Colorize_enabled", func(t *testing.T) {
		SetEnabled(true)
		got := Colorize("hello", Red)
		assert.Equal(t, "\033[31mhello\033[0m", got)
	})

	t.Run("ConvenienceFunctions", func(t *testing.T) {
		SetEnabled(true)
		assert.Equal(t, "\033[31mtext\033[0m", RedText("text"))
		assert.Equal(t, "\033[32mtext\033[0m", GreenText("text"))
		assert.Equal(t, "\033[33mtext\033[0m", YellowText("text"))
		assert.Equal(t, "\033[36mtext\033[0m", CyanText("text"))
	})

	t.Run("ColorizeLogLine_patterns", func(t *testing.T) {
		SetEnabled(true)

		tests := []struct {
			name    string
			input   string
			contain string
		}{
			{
				name:    "agent banner",
				input:   `[2026-03-10 12:00:00] ▶ AGENT  Sprint 1/3 "Setup"  iter 1/10  engine=claude`,
				contain: "\033[36m▶ \033[0m",
			},
			{
				name:    "pass status",
				input:   "[2026-03-10 12:00:00] SPRINT 1 PASS (2m35s)",
				contain: "\033[32mPASS\033[0m",
			},
			{
				name:    "fail status",
				input:   "[2026-03-10 12:00:00] SPRINT 2 FAIL (alignment exhausted)",
				contain: "\033[31mFAIL\033[0m",
			},
			{
				name:    "warning",
				input:   "[2026-03-10 12:00:00] WARNING: git diff failed",
				contain: "\033[33mWARNING\033[0m",
			},
			{
				name:    "git checkpoint",
				input:   "[2026-03-10 12:00:00]   GIT: checkpoint — sprint 1 complete",
				contain: "\033[2m",
			},
			{
				name:    "separator",
				input:   "[2026-03-10 12:00:00] =========================================",
				contain: "\033[1m",
			},
		}

		for _, tt := range tests {
			result := ColorizeLogLine(tt.input)
			assert.Contains(t, result, tt.contain, "pattern %s", tt.name)
			assert.NotEqual(t, tt.input, result, "pattern %s should be colorized", tt.name)
		}
	})

	t.Run("ColorizeLogLine_disabled", func(t *testing.T) {
		SetEnabled(false)
		input := "[2026-03-10 12:00:00] ▶ AGENT  Sprint 1/3 iter 1/10"
		assert.Equal(t, input, ColorizeLogLine(input))
	})

	t.Run("ColorizeLogLine_plain_passthrough", func(t *testing.T) {
		SetEnabled(true)
		input := "[2026-03-10 12:00:00] Generated plans/plan.md."
		assert.Equal(t, input, ColorizeLogLine(input), "unrecognized lines should pass through unchanged")
	})

	// Restore disabled state for any subsequent test in this process.
	SetEnabled(false)
}

// TestContainsWord is pure logic with no global state dependency.
func TestContainsWord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		word string
		want bool
	}{
		{"SPRINT 1 PASS (2m35s)", "PASS", true},
		{"SPRINT 1 FAIL (alignment exhausted)", "FAIL", true},
		{"PASSWORD reset required", "PASS", false},
		{"PASSING all tests", "PASS", false},
		{"FAIL at start", "FAIL", true},
		{"end FAIL", "FAIL", true},
		{"no match here", "FAIL", false},
	}

	for _, tt := range tests {
		got := containsWord(tt.line, tt.word)
		assert.Equal(t, tt.want, got, "containsWord(%q, %q)", tt.line, tt.word)
	}
}
