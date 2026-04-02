package textutil

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// JSONExtractDiagnostics describes how JSON extraction succeeded or failed.
type JSONExtractDiagnostics struct {
	Strategy         string
	FenceCandidates  int
	ObjectCandidates int
	Repaired         bool
}

// StripMarkdownFences removes leading/trailing markdown code fence markers.
func StripMarkdownFences(output string) string {
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "```") {
		return output
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return ""
	}
	lines = lines[1:]
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// FileSize returns the size in bytes of a file, or -1 if the file does not exist.
func FileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return info.Size()
}

// ShellQuote returns a single-quoted shell string, escaping embedded single quotes.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// TruncateUTF8 truncates s to at most maxBytes bytes without splitting a rune.
func TruncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes]
}

// ExtractJSON extracts and unmarshals a JSON object from LLM output into dest.
func ExtractJSON(output string, dest interface{}) error {
	_, err := ExtractJSONWithDiagnostics(output, dest)
	return err
}

// ExtractJSONWithDiagnostics extracts a JSON object and reports how it was found.
func ExtractJSONWithDiagnostics(output string, dest interface{}) (JSONExtractDiagnostics, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return JSONExtractDiagnostics{}, fmt.Errorf("empty output")
	}

	if err := json.Unmarshal([]byte(trimmed), dest); err == nil {
		return JSONExtractDiagnostics{Strategy: "raw"}, nil
	}

	fences := extractFenceBlocks(trimmed)
	diag := JSONExtractDiagnostics{FenceCandidates: len(fences)}
	for _, fence := range fences {
		if err := json.Unmarshal([]byte(fence.content), dest); err == nil {
			diag.Strategy = fence.strategy
			diag.Repaired = true
			return diag, nil
		}
	}

	candidates := extractJSONObjectCandidates(trimmed)
	diag.ObjectCandidates = len(candidates)
	for i := len(candidates) - 1; i >= 0; i-- {
		if err := json.Unmarshal([]byte(candidates[i]), dest); err == nil {
			diag.Strategy = "last_valid_object"
			diag.Repaired = true
			return diag, nil
		}
	}

	return diag, fmt.Errorf(
		"no valid JSON object found (raw invalid, fenced=%d, object_candidates=%d)",
		diag.FenceCandidates,
		diag.ObjectCandidates,
	)
}

type fenceBlock struct {
	content  string
	strategy string
}

func extractFenceBlocks(s string) []fenceBlock {
	var jsonBlocks []fenceBlock
	var genericBlocks []fenceBlock

	for idx := 0; idx < len(s); {
		start := strings.Index(s[idx:], "```")
		if start < 0 {
			break
		}
		start += idx
		afterFence := s[start+3:]
		newline := strings.Index(afterFence, "\n")
		if newline < 0 {
			break
		}
		lang := strings.TrimSpace(afterFence[:newline])
		bodyStart := start + 3 + newline + 1
		endRel := strings.Index(s[bodyStart:], "```")
		if endRel < 0 {
			break
		}
		bodyEnd := bodyStart + endRel
		content := strings.TrimSpace(s[bodyStart:bodyEnd])
		if content != "" {
			if strings.EqualFold(lang, "json") {
				jsonBlocks = append(jsonBlocks, fenceBlock{content: content, strategy: "fenced_json"})
			} else {
				genericBlocks = append(genericBlocks, fenceBlock{content: content, strategy: "fenced"})
			}
		}
		idx = bodyEnd + 3
	}

	return append(jsonBlocks, genericBlocks...)
}

func extractJSONObjectCandidates(s string) []string {
	var candidates []string
	depth := 0
	start := -1
	inString := false
	escape := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			switch ch {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				candidates = append(candidates, s[start:i+1])
				start = -1
			}
		}
	}

	return candidates
}

// ResolveArtifact checks whether the engine wrote the target file during its run.
func ResolveArtifact(targetPath string, beforeSize int64, engineOutput string) error {
	if FileSize(targetPath) != beforeSize {
		return nil
	}
	return os.WriteFile(targetPath, []byte(StripMarkdownFences(engineOutput)), 0o644)
}
