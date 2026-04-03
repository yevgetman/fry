package metrics

import (
	"regexp"
	"strconv"
	"strings"
)

// TokenUsage holds token counts for a single engine invocation.
type TokenUsage struct {
	Input              int `json:"input"`
	Output             int `json:"output"`
	Total              int `json:"total"`
	CacheReadInput     int `json:"cache_read_input,omitempty"`
	CacheCreationInput int `json:"cache_creation_input,omitempty"`
	ModelInput         int `json:"model_input,omitempty"`
	ModelOutput        int `json:"model_output,omitempty"`
	ModelCacheRead     int `json:"model_cache_read_input,omitempty"`
	ModelCacheCreation int `json:"model_cache_creation_input,omitempty"`
}

// SprintTokens aggregates token usage across all agent calls for one sprint.
type SprintTokens struct {
	SprintNum int
	Usage     TokenUsage
}

// claudeInputRe matches Claude CLI usage lines like:
//
//	input_tokens: 1234
//	Input tokens: 1234
//
// It uses a word boundary to avoid matching cache_read_input_tokens or
// cache_creation_input_tokens.
var claudeInputRe = regexp.MustCompile(`(?i)\b"?input[_\s]tokens?"?[:\s]+(\d+)`)

// claudeOutputRe matches Claude CLI usage lines like:
//
//	output_tokens: 567
//	Output tokens: 567
//
// It uses a word boundary to avoid matching hypothetical cache_*_output_tokens
// fields, parallel to the fix applied to claudeInputRe.
var claudeOutputRe = regexp.MustCompile(`(?i)\b"?output[_\s]tokens?"?[:\s]+(\d+)`)
var claudeCacheReadRe = regexp.MustCompile(`(?i)\b"?cache[_\s]read[_\s]input[_\s]tokens?"?[:\s]+(\d+)`)
var claudeCacheCreationRe = regexp.MustCompile(`(?i)\b"?cache[_\s]creation[_\s]input[_\s]tokens?"?[:\s]+(\d+)`)
var modelInputRe = regexp.MustCompile(`(?i)"inputTokens"\s*:\s*(\d+)`)
var modelOutputRe = regexp.MustCompile(`(?i)"outputTokens"\s*:\s*(\d+)`)
var modelCacheReadRe = regexp.MustCompile(`(?i)"cacheReadInputTokens"\s*:\s*(\d+)`)
var modelCacheCreationRe = regexp.MustCompile(`(?i)"cacheCreationInputTokens"\s*:\s*(\d+)`)

// ParseClaudeTokens parses token usage from Claude engine output.
// It sums all occurrences of input/output token counts found in the output.
// Returns a zero TokenUsage if no usage lines are found.
func ParseClaudeTokens(output string) TokenUsage {
	u := TokenUsage{
		Input:              sumTokenMatches(claudeInputRe, output),
		Output:             sumTokenMatches(claudeOutputRe, output),
		CacheReadInput:     sumTokenMatches(claudeCacheReadRe, output),
		CacheCreationInput: sumTokenMatches(claudeCacheCreationRe, output),
		ModelInput:         sumTokenMatches(modelInputRe, output),
		ModelOutput:        sumTokenMatches(modelOutputRe, output),
		ModelCacheRead:     sumTokenMatches(modelCacheReadRe, output),
		ModelCacheCreation: sumTokenMatches(modelCacheCreationRe, output),
	}
	return finalizeTokenUsage(u)
}

// codexInputRe matches OpenAI/Codex usage lines like:
//
//	prompt_tokens: 1234
//	"prompt_tokens": 1234
//
// It uses a word boundary to avoid matching hypothetical prefixed fields
// like partial_prompt_tokens.
var codexInputRe = regexp.MustCompile(`(?i)\b"?prompt[_\s]tokens"?[:\s]+(\d+)`)
var codexInputAltRe = regexp.MustCompile(`(?i)\b"?input[_\s]tokens"?[:\s]+(\d+)`)
var codexCachedInputRe = regexp.MustCompile(`(?i)\b"?cached[_\s]input[_\s]tokens"?[:\s]+(\d+)`)
var codexCachedPromptRe = regexp.MustCompile(`(?i)\b"?cached[_\s]prompt[_\s]tokens"?[:\s]+(\d+)`)

// codexOutputRe matches OpenAI/Codex usage lines like:
//
//	completion_tokens: 567
//	"completion_tokens": 567
//
// It uses a word boundary to avoid matching hypothetical prefixed fields
// like partial_completion_tokens.
var codexOutputRe = regexp.MustCompile(`(?i)\b"?completion[_\s]tokens"?[:\s]+(\d+)`)
var codexOutputAltRe = regexp.MustCompile(`(?i)\b"?output[_\s]tokens"?[:\s]+(\d+)`)

// ParseCodexTokens parses token usage from Codex/OpenAI engine output.
// It sums all occurrences of prompt/completion token counts found in the output.
// Returns a zero TokenUsage if no usage lines are found.
func ParseCodexTokens(output string) TokenUsage {
	var u TokenUsage
	u.Input = sumTokenMatches(codexInputRe, output)
	if u.Input == 0 {
		u.Input = sumTokenMatches(codexInputAltRe, output)
	}
	u.Output = sumTokenMatches(codexOutputRe, output)
	if u.Output == 0 {
		u.Output = sumTokenMatches(codexOutputAltRe, output)
	}
	u.CacheReadInput = sumTokenMatches(codexCachedInputRe, output)
	if u.CacheReadInput == 0 {
		u.CacheReadInput = sumTokenMatches(codexCachedPromptRe, output)
	}
	u.ModelInput = sumTokenMatches(modelInputRe, output)
	u.ModelOutput = sumTokenMatches(modelOutputRe, output)
	u.ModelCacheRead = sumTokenMatches(modelCacheReadRe, output)
	u.ModelCacheCreation = sumTokenMatches(modelCacheCreationRe, output)
	return finalizeTokenUsage(u)
}

// ParseTokens dispatches to the correct parser based on engine name.
func ParseTokens(engineName, output string) TokenUsage {
	switch strings.ToLower(engineName) {
	case "codex":
		return ParseCodexTokens(output)
	case "ollama":
		// Ollama CLI does not report token usage; counts are always zero.
		return TokenUsage{}
	default: // Unrecognised engines are treated as Claude-format output.
		return ParseClaudeTokens(output)
	}
}

func sumTokenMatches(re *regexp.Regexp, output string) int {
	total := 0
	for _, m := range re.FindAllStringSubmatch(output, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			total += n
		}
	}
	return total
}

func finalizeTokenUsage(u TokenUsage) TokenUsage {
	if u.Input == 0 && u.Output == 0 && (u.ModelInput > 0 || u.ModelOutput > 0) {
		u.Input = u.ModelInput
		u.Output = u.ModelOutput
		if u.CacheReadInput == 0 {
			u.CacheReadInput = u.ModelCacheRead
		}
		if u.CacheCreationInput == 0 {
			u.CacheCreationInput = u.ModelCacheCreation
		}
	}
	u.Total = u.Input + u.Output
	return u
}
