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

// stubEngine is a test double that records the invocation prompt and writes
// a canned response to the expected output file.
type stubEngine struct {
	lastPrompt string
	response   string
	writeFile  bool // if true, writes response to config.CodebaseFile
	workDir    string
}

func (s *stubEngine) Run(_ context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	s.lastPrompt = prompt
	s.workDir = opts.WorkDir
	if s.writeFile {
		outPath := filepath.Join(opts.WorkDir, config.CodebaseFile)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return "", 1, err
		}
		if err := os.WriteFile(outPath, []byte(s.response), 0o644); err != nil {
			return "", 1, err
		}
	}
	return s.response, 0, nil
}

func (s *stubEngine) Name() string { return "stub" }

func TestRunSemanticScan_WritesCodebaseMd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create .fry dir.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))

	snap := &StructuralSnapshot{
		RootDir:    dir,
		IsExisting: true,
		Languages: []Language{
			{Name: "Go", Confidence: "high", Marker: "go.mod"},
		},
		Frameworks:  []string{"Cobra"},
		EntryPoints: []string{"cmd/app/main.go"},
		FileTree: []FileEntry{
			{RelPath: "go.mod", Extension: ".mod", Size: 100},
			{RelPath: "cmd/app/main.go", Extension: ".go", Size: 200},
		},
		FileStats: FileStats{
			TotalFiles: 2,
			TotalDirs:  2,
			TotalSize:  300,
			CountByExt: map[string]int{".go": 1, ".mod": 1},
		},
	}

	eng := &stubEngine{
		response:  "# Codebase: Test\n\n## Summary\nA test project.\n",
		writeFile: true,
	}

	err := RunSemanticScan(context.Background(), SemanticScanOpts{
		ProjectDir: dir,
		Snapshot:   snap,
		Engine:     eng,
		Model:      "sonnet",
	})
	require.NoError(t, err)

	// Verify codebase.md was created.
	codebasePath := filepath.Join(dir, config.CodebaseFile)
	data, err := os.ReadFile(codebasePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Codebase: Test")

	// Verify prompt file was cleaned up.
	_, err = os.Stat(filepath.Join(dir, scanPromptFile))
	assert.True(t, os.IsNotExist(err), "scan prompt should be cleaned up after success")
}

func TestRunSemanticScan_ErrorWhenNoEngine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := RunSemanticScan(context.Background(), SemanticScanOpts{
		ProjectDir: dir,
		Snapshot:   &StructuralSnapshot{},
		Engine:     nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

func TestRunSemanticScan_ErrorWhenNoSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := RunSemanticScan(context.Background(), SemanticScanOpts{
		ProjectDir: dir,
		Snapshot:   nil,
		Engine:     &stubEngine{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot is required")
}

func TestRunSemanticScan_ErrorWhenEngineDoesntWriteFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))

	eng := &stubEngine{
		response:  "some output",
		writeFile: false, // Engine doesn't write the file.
	}

	err := RunSemanticScan(context.Background(), SemanticScanOpts{
		ProjectDir: dir,
		Snapshot:   &StructuralSnapshot{RootDir: dir},
		Engine:     eng,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "engine did not write")
}

func TestAssembleSemanticPrompt_IncludesStructuralData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	snap := &StructuralSnapshot{
		RootDir:    dir,
		IsExisting: true,
		Languages: []Language{
			{Name: "Go", Confidence: "high", Marker: "go.mod"},
			{Name: "Python", Confidence: "medium", Marker: ".py (10 files)"},
		},
		Frameworks:  []string{"Cobra", "Flask"},
		EntryPoints: []string{"cmd/main.go", "app.py"},
		Dependencies: []Dependency{
			{Name: "github.com/spf13/cobra", Version: "v1.10.2", Source: "go.mod"},
			{Name: "flask", Version: "==2.0.1", Source: "requirements.txt"},
		},
		TestDirs:  []string{"internal/foo", "tests"},
		DocFiles:  []string{"README.md", "docs/"},
		FileTree:  []FileEntry{{RelPath: "cmd/main.go", Extension: ".go", Size: 100}},
		FileStats: FileStats{
			TotalFiles:     50,
			TotalDirs:      10,
			TotalSize:      100_000,
			CountByExt:     map[string]int{".go": 30, ".py": 10, ".md": 5},
			TopDirectories: []DirStat{{RelPath: "internal", FileCount: 20}},
		},
		GitHistory: &GitHistory{
			TotalCommits: 42,
			RecentLog:    []string{"abc123 Fix bug", "def456 Add feature"},
			HotFiles:     []HotFile{{RelPath: "main.go", ChangeCount: 15}},
			TopAuthors:   []string{"10\tAlice"},
		},
		ReadmeContent: "# Test Project\nA great project.\n",
	}

	prompt := assembleSemanticPrompt(snap, dir)

	// Verify key sections are present.
	assert.Contains(t, prompt, "# Codebase Analysis Task")
	assert.Contains(t, prompt, "## Output Format")
	assert.Contains(t, prompt, "## Input Data")

	// Structural data.
	assert.Contains(t, prompt, "Total files: 50")
	assert.Contains(t, prompt, "Go (confidence: high")
	assert.Contains(t, prompt, "Python (confidence: medium")
	assert.Contains(t, prompt, "Cobra")
	assert.Contains(t, prompt, "Flask")
	assert.Contains(t, prompt, "cmd/main.go")
	assert.Contains(t, prompt, "github.com/spf13/cobra")
	assert.Contains(t, prompt, "flask")
	assert.Contains(t, prompt, "internal/foo")
	assert.Contains(t, prompt, "docs/")

	// Git history.
	assert.Contains(t, prompt, "Total commits: 42")
	assert.Contains(t, prompt, "abc123 Fix bug")
	assert.Contains(t, prompt, "main.go (15 changes)")
	assert.Contains(t, prompt, "Alice")

	// README.
	assert.Contains(t, prompt, "# Test Project")
}

func TestAssembleSemanticPrompt_EmptySnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	snap := &StructuralSnapshot{
		RootDir: dir,
		FileStats: FileStats{
			CountByExt: make(map[string]int),
		},
	}

	prompt := assembleSemanticPrompt(snap, dir)

	// Should still have the task and format sections.
	assert.Contains(t, prompt, "# Codebase Analysis Task")
	assert.Contains(t, prompt, "## Output Format")
	assert.Contains(t, prompt, "Total files: 0")
}

func TestSelectKeyFiles_PrioritizesEntryPoints(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create files.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "app", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.22\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "types.go"), []byte("package main\ntype Foo struct{}\n"), 0o644))

	snap := &StructuralSnapshot{
		RootDir:     dir,
		EntryPoints: []string{"cmd/app/main.go"},
		FileTree: []FileEntry{
			{RelPath: "cmd/app/main.go", Extension: ".go", Size: 30},
			{RelPath: "go.mod", Extension: ".mod", Size: 20},
			{RelPath: "types.go", Extension: ".go", Size: 30},
		},
	}

	files := selectKeyFiles(snap, dir)

	require.NotEmpty(t, files)
	// Entry point should be first (priority 1).
	assert.Equal(t, "cmd/app/main.go", files[0].relPath)
	assert.NotEmpty(t, files[0].content)
}

func TestSelectKeyFiles_RespectsMaxBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a bunch of large files.
	var tree []FileEntry
	for i := 0; i < 30; i++ {
		name := "types_" + strings.Repeat("x", 5) + string(rune('a'+i%26)) + ".go"
		content := strings.Repeat("x", maxSingleFileBytes-100)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
		tree = append(tree, FileEntry{RelPath: name, Extension: ".go", Size: int64(len(content))})
	}

	snap := &StructuralSnapshot{
		RootDir:  dir,
		FileTree: tree,
	}

	files := selectKeyFiles(snap, dir)

	// Should be limited by maxKeyFileBytes total.
	totalBytes := 0
	for _, f := range files {
		totalBytes += len(f.content)
	}
	assert.LessOrEqual(t, totalBytes, maxKeyFileBytes+1000, "total bytes should respect budget")
	assert.LessOrEqual(t, len(files), maxKeyFiles, "file count should respect limit")
}

func TestSelectKeyFiles_DeduplicatesFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// A file that matches both entry point and config criteria.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))

	snap := &StructuralSnapshot{
		RootDir:     dir,
		EntryPoints: []string{"main.go"},
		FileTree: []FileEntry{
			{RelPath: "main.go", Extension: ".go", Size: 13},
		},
	}

	files := selectKeyFiles(snap, dir)

	// main.go should appear exactly once.
	count := 0
	for _, f := range files {
		if f.relPath == "main.go" {
			count++
		}
	}
	assert.Equal(t, 1, count, "main.go should not be duplicated")
}
