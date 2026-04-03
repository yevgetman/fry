package metrics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, 0, u.CacheReadInput)
	assert.Equal(t, 0, u.CacheCreationInput)
}

func TestParseClaudeTokensCacheFieldsNotCounted(t *testing.T) {
	t.Parallel()

	// Non-zero cache fields must not inflate Input count.
	output := `Usage:
  input_tokens: 500
  output_tokens: 200
  cache_read_input_tokens: 300
  cache_creation_input_tokens: 150`

	u := ParseClaudeTokens(output)
	assert.Equal(t, 500, u.Input, "Input should only count input_tokens, not cache fields")
	assert.Equal(t, 200, u.Output)
	assert.Equal(t, 700, u.Total)
	assert.Equal(t, 300, u.CacheReadInput)
	assert.Equal(t, 150, u.CacheCreationInput)
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

func TestParseCodexTokensJSONLUsageReturnsCorrectCounts(t *testing.T) {
	t.Parallel()

	output := `{"type":"thread.started","thread_id":"019d5066-f512-7bc1-aba8-e45cf2fb9a84"}
{"type":"turn.completed","usage":{"input_tokens":16250,"cached_input_tokens":14080,"output_tokens":32}}`

	u := ParseCodexTokens(output)
	assert.Equal(t, 16250, u.Input)
	assert.Equal(t, 32, u.Output)
	assert.Equal(t, 16282, u.Total)
	assert.Equal(t, 14080, u.CacheReadInput)
}

func TestParseClaudeTokensCacheOutputFieldsNotCounted(t *testing.T) {
	t.Parallel()

	// Hypothetical future cache_write_output_tokens field must not inflate Output count.
	output := `Usage:
  input_tokens: 500
  output_tokens: 200
  cache_write_output_tokens: 100`

	u := ParseClaudeTokens(output)
	assert.Equal(t, 500, u.Input, "Input should only count input_tokens")
	assert.Equal(t, 200, u.Output, "Output should only count output_tokens, not cache fields")
	assert.Equal(t, 700, u.Total)
}

func TestParseCodexTokensPrefixedFieldsNotCounted(t *testing.T) {
	t.Parallel()

	// Hypothetical prefixed fields must not inflate counts.
	output := `{
  "usage": {
    "prompt_tokens": 800,
    "completion_tokens": 320,
    "cached_prompt_tokens": 400,
    "partial_completion_tokens": 50
  }
}`

	u := ParseCodexTokens(output)
	assert.Equal(t, 800, u.Input, "Input should only count prompt_tokens, not cached_prompt_tokens")
	assert.Equal(t, 320, u.Output, "Output should only count completion_tokens, not partial_completion_tokens")
	assert.Equal(t, 1120, u.Total)
	assert.Equal(t, 400, u.CacheReadInput)
}

func TestParseTokensOllamaReturnsZero(t *testing.T) {
	t.Parallel()
	for _, output := range []string{"", "some random output", "input_tokens: 100"} {
		got := ParseTokens("ollama", output)
		assert.Equal(t, TokenUsage{}, got, "ollama output: %q", output)
	}
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

func TestParseClaudeTokensMeetinglyFixtureOne(t *testing.T) {
	t.Parallel()

	output := readTokenFixture(t, "claude_meetingly_audit1.json")

	u := ParseClaudeTokens(output)
	assert.Equal(t, 2507, u.Input)
	assert.Equal(t, 11196, u.Output)
	assert.Equal(t, 13703, u.Total)
	assert.Equal(t, 1003915, u.CacheReadInput)
	assert.Equal(t, 84559, u.CacheCreationInput)
	assert.Equal(t, 5326, u.ModelInput)
	assert.Equal(t, 59919, u.ModelOutput)
	assert.Equal(t, 4099722, u.ModelCacheRead)
	assert.Equal(t, 493678, u.ModelCacheCreation)
}

func TestParseClaudeTokensMeetinglyFixtureTwo(t *testing.T) {
	t.Parallel()

	output := readTokenFixture(t, "claude_meetingly_audit2.json")

	u := ParseClaudeTokens(output)
	assert.Equal(t, 8553, u.Input)
	assert.Equal(t, 14854, u.Output)
	assert.Equal(t, 23407, u.Total)
	assert.Equal(t, 2087279, u.CacheReadInput)
	assert.Equal(t, 130994, u.CacheCreationInput)
	assert.Equal(t, 8673, u.ModelInput)
	assert.Equal(t, 54082, u.ModelOutput)
	assert.Equal(t, 7478093, u.ModelCacheRead)
	assert.Equal(t, 406728, u.ModelCacheCreation)
}

func TestParseClaudeTokensFallsBackToModelUsageTotals(t *testing.T) {
	t.Parallel()

	output := `{"type":"result","modelUsage":{"claude-opus-4-6[1m]":{"inputTokens":4100,"outputTokens":900,"cacheReadInputTokens":22000,"cacheCreationInputTokens":700}}}`

	u := ParseClaudeTokens(output)
	assert.Equal(t, 4100, u.Input)
	assert.Equal(t, 900, u.Output)
	assert.Equal(t, 5000, u.Total)
	assert.Equal(t, 22000, u.CacheReadInput)
	assert.Equal(t, 700, u.CacheCreationInput)
	assert.Equal(t, 4100, u.ModelInput)
	assert.Equal(t, 900, u.ModelOutput)
}

func TestParseCodexTokensFixturePreservesCacheRead(t *testing.T) {
	t.Parallel()

	output := readTokenFixture(t, "codex_turn_completed_usage.jsonl")

	u := ParseCodexTokens(output)
	assert.Equal(t, 16250, u.Input)
	assert.Equal(t, 32, u.Output)
	assert.Equal(t, 16282, u.Total)
	assert.Equal(t, 14080, u.CacheReadInput)
}

func readTokenFixture(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
