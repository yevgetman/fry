package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseClaudeTokensReturnsCorrectCounts(t *testing.T) {
	t.Parallel()

	// Realistic Claude CLI output with usage block
	output := `Running task...
Done with analysis.

Usage:
  input_tokens: 1250
  output_tokens: 487
  cache_read_input_tokens: 0
  cache_creation_input_tokens: 0`

	u := ParseClaudeTokens(output)
	assert.Equal(t, 1250, u.Input)
	assert.Equal(t, 487, u.Output)
	assert.Equal(t, 1737, u.Total)
}

func TestParseClaudeTokensNoUsageReturnsZero(t *testing.T) {
	t.Parallel()

	output := "Agent completed successfully.\nSome output here.\nDone."

	u := ParseClaudeTokens(output)
	assert.Equal(t, 0, u.Input)
	assert.Equal(t, 0, u.Output)
	assert.Equal(t, 0, u.Total)
}

func TestParseCodexTokensReturnsCorrectCounts(t *testing.T) {
	t.Parallel()

	// Realistic Codex/OpenAI usage output
	output := `{
  "usage": {
    "prompt_tokens": 800,
    "completion_tokens": 320,
    "total_tokens": 1120
  }
}`

	u := ParseCodexTokens(output)
	assert.Equal(t, 800, u.Input)
	assert.Equal(t, 320, u.Output)
	assert.Equal(t, 1120, u.Total)
}

func TestParseCodexTokensNoUsageReturnsZero(t *testing.T) {
	t.Parallel()

	output := "Processing complete. No usage information available."

	u := ParseCodexTokens(output)
	assert.Equal(t, 0, u.Input)
	assert.Equal(t, 0, u.Output)
	assert.Equal(t, 0, u.Total)
}

func TestParseTokensDispatchesByEngine(t *testing.T) {
	t.Parallel()

	claudeOutput := "input_tokens: 100\noutput_tokens: 50"
	codexOutput := "prompt_tokens: 200\ncompletion_tokens: 80"

	uClaude := ParseTokens("claude", claudeOutput)
	assert.Equal(t, 100, uClaude.Input)
	assert.Equal(t, 50, uClaude.Output)

	uCodex := ParseTokens("codex", codexOutput)
	assert.Equal(t, 200, uCodex.Input)
	assert.Equal(t, 80, uCodex.Output)

	// Unknown engine falls through to Claude parser.
	uUnknown := ParseTokens("gemini", claudeOutput)
	assert.Equal(t, 100, uUnknown.Input)
	assert.Equal(t, 50, uUnknown.Output)
}
