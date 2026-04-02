package textutil

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveArtifactUsesEngineFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "artifact.md")

	// Record before-size (file doesn't exist yet → -1).
	before := FileSize(target)

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
	before := FileSize(target)
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
	before := FileSize(target)

	// Engine doesn't modify the file → fallback writes output.
	output := "new content"
	require.NoError(t, ResolveArtifact(target, before, output))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, output, string(got))
}

func TestResolveArtifactSameSizeRewrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "artifact.md")

	// File exists with 11 bytes.
	require.NoError(t, os.WriteFile(target, []byte("old content"), 0o644))
	before := FileSize(target)

	// Engine rewrites the file with different content but the same byte count.
	require.NoError(t, os.WriteFile(target, []byte("new content"), 0o644))

	// Because sizes match, ResolveArtifact treats the file as unchanged and
	// overwrites with the fallback output — this is the known limitation.
	output := "fallback text"
	require.NoError(t, ResolveArtifact(target, before, output))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, output, string(got), "same-size rewrite triggers fallback (known limitation)")
}

func TestFileSize(t *testing.T) {
	t.Parallel()

	// Non-existent file returns -1.
	assert.Equal(t, int64(-1), FileSize("/no/such/file"))

	// Existing file returns its byte count.
	path := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))
	assert.Equal(t, int64(5), FileSize(path))
}

func TestTruncateUTF8(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{"empty string", "", 10, ""},
		{"shorter than limit", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"longer than limit", "hello world", 5, "hello"},
		{"limit zero", "anything", 0, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := TruncateUTF8(tc.input, tc.limit)
			assert.Equal(t, tc.want, result)
		})
	}

	// Multi-byte: "héllo" is 6 bytes (h=1, é=2, l=1, l=1, o=1).
	// Truncating to 3 bytes must not split the 'é' rune — result is "hé".
	t.Run("multibyte no split rune", func(t *testing.T) {
		t.Parallel()
		result := TruncateUTF8("héllo", 3)
		assert.LessOrEqual(t, len(result), 3)
		assert.True(t, utf8.ValidString(result), "result must be valid UTF-8")
	})
}

func TestExtractJSON(t *testing.T) {
	t.Parallel()

	type result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name    string
		input   string
		want    result
		wantErr bool
	}{
		{
			name:  "raw JSON",
			input: `{"name":"test","value":42}`,
			want:  result{Name: "test", Value: 42},
		},
		{
			name:  "JSON with whitespace",
			input: `  {"name":"test","value":42}  `,
			want:  result{Name: "test", Value: 42},
		},
		{
			name:  "JSON in code fence",
			input: "Here is the result:\n```json\n{\"name\":\"fenced\",\"value\":1}\n```\nDone.",
			want:  result{Name: "fenced", Value: 1},
		},
		{
			name:  "JSON in plain code fence",
			input: "```\n{\"name\":\"plain\",\"value\":2}\n```",
			want:  result{Name: "plain", Value: 2},
		},
		{
			name:  "JSON embedded in prose",
			input: "The analysis shows:\n{\"name\":\"embedded\",\"value\":3}\nEnd of analysis.",
			want:  result{Name: "embedded", Value: 3},
		},
		{
			name:  "last valid JSON object wins",
			input: "first: {\"name\":\"wrong\",\"value\":\"oops\"}\nsecond: {\"name\":\"final\",\"value\":4}",
			want:  result{Name: "final", Value: 4},
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no JSON at all",
			input:   "This is plain text with no structured data.",
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			input:   `{"name": "broken", value: }`,
			wantErr: true,
		},
		{
			name:  "extra fields ignored",
			input: `{"name":"test","value":42,"extra":"ignored"}`,
			want:  result{Name: "test", Value: 42},
		},
		{
			name:  "pretty-printed JSON",
			input: "{\n  \"name\": \"pretty\",\n  \"value\": 99\n}",
			want:  result{Name: "pretty", Value: 99},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got result
			err := ExtractJSON(tt.input, &got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExtractJSONWithDiagnostics(t *testing.T) {
	t.Parallel()

	type result struct {
		Name string `json:"name"`
	}

	var got result
	diag, err := ExtractJSONWithDiagnostics("Reading prompt from stdin...\n```json\n{\"name\":\"fenced\"}\n```", &got)
	require.NoError(t, err)
	assert.Equal(t, result{Name: "fenced"}, got)
	assert.True(t, diag.Repaired)
	assert.Equal(t, "fenced_json", diag.Strategy)
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
