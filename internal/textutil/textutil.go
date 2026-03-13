package textutil

import (
	"os"
	"strings"
	"time"
)

// StripMarkdownFences removes leading/trailing markdown code fence markers
// (```) from LLM output that may wrap its response in a code block.
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

// FileModTime returns the modification time of a file, or the zero time if
// the file does not exist or cannot be stat'd.
func FileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// ShellQuote returns a single-quoted shell string, escaping any embedded
// single quotes. Suitable for embedding user strings in bash -c commands.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ResolveArtifact checks whether the engine wrote the target file during its
// run (by comparing modification times). If the file was written by the
// engine, its on-disk content is authoritative and we leave it in place.
// Otherwise we fall back to writing the captured engine output (stripped of
// markdown fences) ourselves.
func ResolveArtifact(targetPath string, beforeMod time.Time, engineOutput string) error {
	if FileModTime(targetPath).After(beforeMod) {
		return nil
	}
	return os.WriteFile(targetPath, []byte(StripMarkdownFences(engineOutput)), 0o644)
}
