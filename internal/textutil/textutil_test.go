package textutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileModTime(t *testing.T) {
	t.Parallel()

	// Non-existent file returns zero time.
	assert.True(t, FileModTime("/no/such/file").IsZero())

	// Existing file returns a non-zero time.
	path := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, os.WriteFile(path, []byte("hi"), 0o644))
	assert.False(t, FileModTime(path).IsZero())
}

func TestResolveArtifactUsesEngineFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "artifact.md")

	// Record before-mod (file doesn't exist yet → zero).
	before := FileModTime(target)

	// Simulate engine writing the file with clean content.
	cleanContent := "1. Rule one\n2. Rule two\n"
	require.NoError(t, os.WriteFile(target, []byte(cleanContent), 0o644))

	// Engine output contains session noise that should NOT be used.
	noisyOutput := "OpenAI Codex v1.0\n--------\nuser\nprompt...\n" + cleanContent + "\ndiff --git ...\n"

	require.NoError(t, ResolveArtifact(target, before, noisyOutput))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, cleanContent, string(got), "should preserve the engine-written file, not the noisy output")
}

func TestResolveArtifactFallsBackToOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "artifact.md")

	// File doesn't exist and engine doesn't write it → fallback to output.
	before := FileModTime(target)
	output := "1. Rule one\n2. Rule two\n"

	require.NoError(t, ResolveArtifact(target, before, output))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, output, string(got))
}

func TestResolveArtifactFallsBackWhenFileUnchanged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "artifact.md")

	// File exists before engine run.
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))
	before := FileModTime(target)

	// Wait to ensure any new write would have a later mod time.
	time.Sleep(10 * time.Millisecond)

	// Engine doesn't modify the file → fallback writes output.
	output := "new content"
	require.NoError(t, ResolveArtifact(target, before, output))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, output, string(got))
}

func TestStripMarkdownFences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"basic fences", "```\ncontent\n```", "content"},
		{"language tag", "```markdown\ncontent\n```", "content"},
		{"no fences", "no fences here", "no fences here"},
		{"empty input", "", ""},
		{"go language tag", "```go\nfunc main() {}\n```", "func main() {}"},
		{"surrounding whitespace", "  ```\ncontent\n```  ", "content"},
		{"no closing fence", "```\ncontent\nmore", "content\nmore"},
		{"multi-line content", "```\nline1\nline2\nline3\n```", "line1\nline2\nline3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, StripMarkdownFences(tt.input))
		})
	}
}
