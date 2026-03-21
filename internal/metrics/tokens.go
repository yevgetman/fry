package metrics

import (
	"regexp"
	"strconv"
	"strings"
)

// TokenUsage holds token counts for a single engine invocation.
type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
}

// SprintTokens aggregates token usage across all agent calls for one sprint.
type SprintTokens struct {
	SprintNum int
	Usage     TokenUsage
}

// claudeInputRe matches Claude CLI usage lines like:
//   input_tokens: 1234
//   Input tokens: 1234
// It uses a word boundary to avoid matching cache_read_input_tokens or
// cache_creation_input_tokens.
var claudeInputRe = regexp.MustCompile(`(?i)\binput[_\s]tokens?[:\s]+(\d+)`)

// claudeOutputRe matches Claude CLI usage lines like:
//   output_tokens: 567
//   Output tokens: 567
// It uses a word boundary to avoid matching hypothetical cache_*_output_tokens
// fields, parallel to the fix applied to claudeInputRe.
var claudeOutputRe = regexp.MustCompile(`(?i)\boutput[_\s]tokens?[:\s]+(\d+)`)

// ParseClaudeTokens parses token usage from Claude engine output.
// It sums all occurrences of input/output token counts found in the output.
// Returns a zero TokenUsage if no usage lines are found.
func ParseClaudeTokens(output string) TokenUsage {
	var u TokenUsage
	for _, m := range claudeInputRe.FindAllStringSubmatch(output, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			u.Input += n
		}
	}
	for _, m := range claudeOutputRe.FindAllStringSubmatch(output, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			u.Output += n
		}
	}
	u.Total = u.Input + u.Output
	return u
}

// codexInputRe matches OpenAI/Codex usage lines like:
//   prompt_tokens: 1234
//   "prompt_tokens": 1234
// It uses a word boundary to avoid matching hypothetical prefixed fields
// like partial_prompt_tokens.
var codexInputRe = regexp.MustCompile(`(?i)\b"?prompt[_\s]tokens"?[:\s]+(\d+)`)

// codexOutputRe matches OpenAI/Codex usage lines like:
//   completion_tokens: 567
//   "completion_tokens": 567
// It uses a word boundary to avoid matching hypothetical prefixed fields
// like partial_completion_tokens.
var codexOutputRe = regexp.MustCompile(`(?i)\b"?completion[_\s]tokens"?[:\s]+(\d+)`)

// ParseCodexTokens parses token usage from Codex/OpenAI engine output.
// It sums all occurrences of prompt/completion token counts found in the output.
// Returns a zero TokenUsage if no usage lines are found.
func ParseCodexTokens(output string) TokenUsage {
	var u TokenUsage
	for _, m := range codexInputRe.FindAllStringSubmatch(output, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			u.Input += n
		}
	}
	for _, m := range codexOutputRe.FindAllStringSubmatch(output, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			u.Output += n
		}
	}
	u.Total = u.Input + u.Output
	return u
}

// ParseTokens dispatches to the correct parser based on engine name.
func ParseTokens(engineName, output string) TokenUsage {
	switch strings.ToLower(engineName) {
	case "codex":
		return ParseCodexTokens(output)
	default: // Unrecognised engines are treated as Claude-format output.
		return ParseClaudeTokens(output)
	}
}
