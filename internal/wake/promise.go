package wake

import (
	"bytes"
	"encoding/json"
	"strings"
)

const PromiseToken = "===WAKE_DONE==="

// ExtractPromise searches output for the promise token.
// If output looks like JSON (from --output-format json), it searches the "result" field.
// Returns (found, resultText).
func ExtractPromise(output []byte) (bool, string) {
	text := tryExtractResultField(output)
	if text == "" {
		text = string(output)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == PromiseToken {
			return true, text
		}
	}
	return false, text
}

// ExtractStatusTransition looks for FRY_STATUS_TRANSITION=<status> in output.
// Returns ("", false) if not found.
func ExtractStatusTransition(output []byte) (string, bool) {
	text := tryExtractResultField(output)
	if text == "" {
		text = string(output)
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FRY_STATUS_TRANSITION=") {
			val := strings.TrimPrefix(line, "FRY_STATUS_TRANSITION=")
			val = strings.TrimSpace(val)
			if val != "" {
				return val, true
			}
		}
	}
	return "", false
}

// ParseCostUSD extracts total_cost_usd from JSON output, or returns 0.
func ParseCostUSD(output []byte) float64 {
	var obj struct {
		TotalCostUSD float64 `json:"total_cost_usd"`
	}
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		_ = json.Unmarshal(trimmed, &obj)
	}
	return obj.TotalCostUSD
}

// tryExtractResultField parses JSON output and returns the "result" field text.
func tryExtractResultField(output []byte) string {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return ""
	}
	var obj struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return ""
	}
	return obj.Result
}
