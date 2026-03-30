package scan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
)

// memoryStubEngine writes memories to stdout.
type memoryStubEngine struct {
	output string
}

func (s *memoryStubEngine) Run(_ context.Context, _ string, _ engine.RunOpts) (string, int, error) {
	return s.output, 0, nil
}
func (s *memoryStubEngine) Name() string { return "stub" }

func TestParseMemoryOutput_MultipleMemories(t *testing.T) {
	t.Parallel()

	output := `MEMORY: <confidence: high>
Tests require DATABASE_URL to be set. The test suite loads .env.test automatically.
---
MEMORY: <confidence: medium>
The auth middleware must be registered before route handlers in server.go.
---
MEMORY: <confidence: low>
There's a TODO comment in utils.go about refactoring the date parser.
---`

	memories := parseMemoryOutput(output, "build-123", 3)

	require.Equal(t, 3, len(memories))

	assert.Equal(t, "high", memories[0].Confidence)
	assert.Contains(t, memories[0].Body, "DATABASE_URL")

	assert.Equal(t, "medium", memories[1].Confidence)
	assert.Contains(t, memories[1].Body, "auth middleware")

	assert.Equal(t, "low", memories[2].Confidence)
	assert.Contains(t, memories[2].Body, "TODO comment")
}

func TestParseMemoryOutput_NoMemories(t *testing.T) {
	t.Parallel()

	assert.Nil(t, parseMemoryOutput("NO_MEMORIES", "build-1", 1))
	assert.Nil(t, parseMemoryOutput("", "build-1", 1))
}

func TestParseMemoryOutput_ConfidenceVariants(t *testing.T) {
	t.Parallel()

	// Test confidence without angle brackets.
	output := `MEMORY: high
A learning.
---`
	memories := parseMemoryOutput(output, "build-1", 1)
	require.Equal(t, 1, len(memories))
	assert.Equal(t, "high", memories[0].Confidence)
}

func TestWriteAndParseMemoryFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	m := Memory{
		Filename:   "001-2026-03-29-test.md",
		Confidence: "high",
		Source:     "build-2026-03-29-143857",
		Sprint:     3,
		Date:       "2026-03-29",
		Reinforced: 2,
		Body:       "Tests require DATABASE_URL env var to be set.",
	}

	path := filepath.Join(dir, m.Filename)
	require.NoError(t, writeMemoryFile(path, m))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "confidence: high")
	assert.Contains(t, content, "reinforced: 2")
	assert.Contains(t, content, "DATABASE_URL")

	// Parse it back.
	parsed := parseMemoryFile(m.Filename, content)
	assert.Equal(t, "high", parsed.Confidence)
	assert.Equal(t, "build-2026-03-29-143857", parsed.Source)
	assert.Equal(t, 3, parsed.Sprint)
	assert.Equal(t, "2026-03-29", parsed.Date)
	assert.Equal(t, 2, parsed.Reinforced)
	assert.Equal(t, "Tests require DATABASE_URL env var to be set.", parsed.Body)
}

func TestLoadMemories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write two memory files.
	m1 := Memory{Filename: "001-test.md", Confidence: "high", Body: "First learning.", Date: "2026-03-29"}
	m2 := Memory{Filename: "002-test.md", Confidence: "medium", Body: "Second learning.", Date: "2026-03-29"}
	require.NoError(t, writeMemoryFile(filepath.Join(dir, m1.Filename), m1))
	require.NoError(t, writeMemoryFile(filepath.Join(dir, m2.Filename), m2))

	// Also write a non-md file that should be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644))

	memories, err := LoadMemories(dir)
	require.NoError(t, err)
	assert.Equal(t, 2, len(memories))
}

func TestLoadMemories_NonExistentDir(t *testing.T) {
	t.Parallel()

	memories, err := LoadMemories("/nonexistent/path")
	assert.NoError(t, err)
	assert.Nil(t, memories)
}

func TestLoadMemoriesForPrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	memoriesDir := filepath.Join(dir, config.CodebaseMemoriesDir)
	require.NoError(t, os.MkdirAll(memoriesDir, 0o755))

	m1 := Memory{Filename: "001-test.md", Confidence: "high", Body: "Important fact.", Date: "2026-03-29"}
	m2 := Memory{Filename: "002-test.md", Confidence: "low", Body: "Minor detail.", Date: "2026-03-29"}
	require.NoError(t, writeMemoryFile(filepath.Join(memoriesDir, m1.Filename), m1))
	require.NoError(t, writeMemoryFile(filepath.Join(memoriesDir, m2.Filename), m2))

	result := LoadMemoriesForPrompt(dir)

	assert.Contains(t, result, "Important fact")
	assert.Contains(t, result, "Minor detail")
	// High confidence should appear first.
	highIdx := strings.Index(result, "Important fact")
	lowIdx := strings.Index(result, "Minor detail")
	assert.True(t, highIdx < lowIdx, "high-confidence memory should appear before low-confidence")
}

func TestLoadMemoriesForPrompt_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result := LoadMemoriesForPrompt(dir)
	assert.Equal(t, "", result)
}

func TestFindDuplicate_ExactMatch(t *testing.T) {
	t.Parallel()

	existing := []Memory{
		{Body: "Tests require DATABASE_URL to be set."},
		{Body: "Auth middleware must be registered first."},
	}

	// Exact match (modulo whitespace/case).
	idx := findDuplicate(existing, "tests require database_url to be set.")
	assert.Equal(t, 0, idx)
}

func TestFindDuplicate_HighOverlap(t *testing.T) {
	t.Parallel()

	existing := []Memory{
		{Body: "The auth middleware requires JWT_SECRET to be set for tests to pass."},
	}

	// Very similar phrasing — only one word different.
	idx := findDuplicate(existing, "The auth middleware requires JWT_SECRET to be set for tests to work.")
	assert.Equal(t, 0, idx)
}

func TestFindDuplicate_NoMatch(t *testing.T) {
	t.Parallel()

	existing := []Memory{
		{Body: "Tests require DATABASE_URL."},
	}

	idx := findDuplicate(existing, "The deployment uses Docker Compose.")
	assert.Equal(t, -1, idx)
}

func TestWordOverlap(t *testing.T) {
	t.Parallel()

	// Identical strings.
	assert.InDelta(t, 1.0, wordOverlap("hello world", "hello world"), 0.01)

	// No overlap.
	assert.InDelta(t, 0.0, wordOverlap("hello world", "foo bar"), 0.01)

	// Partial overlap.
	overlap := wordOverlap("the cat sat on the mat", "the dog sat on the rug")
	assert.True(t, overlap > 0.3 && overlap < 0.8, "partial overlap should be moderate: %f", overlap)

	// Empty strings.
	assert.InDelta(t, 0.0, wordOverlap("", "hello"), 0.01)
	assert.InDelta(t, 0.0, wordOverlap("hello", ""), 0.01)
}

func TestExtractCodebaseMemories_Integration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))

	eng := &memoryStubEngine{
		output: `MEMORY: <confidence: high>
Tests require the DATABASE_URL environment variable to be set.
---
MEMORY: <confidence: medium>
The API routes follow a handler-service-repository pattern.
---`,
	}

	err := ExtractCodebaseMemories(context.Background(), MemoryExtractionOpts{
		ProjectDir:  dir,
		Engine:      eng,
		Model:       "haiku",
		BuildID:     "build-test",
		SprintCount: 2,
	})
	require.NoError(t, err)

	// Verify memory files were created.
	memoriesDir := filepath.Join(dir, config.CodebaseMemoriesDir)
	memories, loadErr := LoadMemories(memoriesDir)
	require.NoError(t, loadErr)
	assert.Equal(t, 2, len(memories))
}

func TestExtractCodebaseMemories_Deduplication(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	memoriesDir := filepath.Join(dir, config.CodebaseMemoriesDir)
	require.NoError(t, os.MkdirAll(memoriesDir, 0o755))

	// Pre-existing memory.
	existing := Memory{
		Filename:   "001-2026-03-29-tests-require-db.md",
		Confidence: "high",
		Body:       "Tests require DATABASE_URL to be set.",
		Date:       "2026-03-29",
		Reinforced: 0,
	}
	require.NoError(t, writeMemoryFile(filepath.Join(memoriesDir, existing.Filename), existing))

	eng := &memoryStubEngine{
		output: `MEMORY: <confidence: high>
Tests require DATABASE_URL to be set.
---
MEMORY: <confidence: medium>
A brand new learning.
---`,
	}

	err := ExtractCodebaseMemories(context.Background(), MemoryExtractionOpts{
		ProjectDir:  dir,
		Engine:      eng,
		Model:       "haiku",
		BuildID:     "build-test-2",
		SprintCount: 1,
	})
	require.NoError(t, err)

	memories, _ := LoadMemories(memoriesDir)

	// Should have 2 memories: existing (reinforced) + new one.
	assert.Equal(t, 2, len(memories))

	// Find the reinforced one.
	for _, m := range memories {
		if strings.Contains(m.Body, "DATABASE_URL") {
			assert.Equal(t, 1, m.Reinforced, "existing memory should be reinforced")
		}
	}
}

func TestExtractCodebaseMemories_NoEngine(t *testing.T) {
	t.Parallel()

	err := ExtractCodebaseMemories(context.Background(), MemoryExtractionOpts{
		ProjectDir: t.TempDir(),
		Engine:     nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "tests-require", slugify("Tests require DATABASE_URL to be set.", 15))
	assert.Equal(t, "memory", slugify("!!!", 20))
	assert.Equal(t, "a", slugify("a", 20))
}

func TestNextMemoryNumber(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, nextMemoryNumber(nil))
	assert.Equal(t, 4, nextMemoryNumber([]Memory{
		{Filename: "001-test.md"},
		{Filename: "003-test.md"},
	}))
}
