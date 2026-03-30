package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo creates a git repo with an initial commit in dir.
func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	ctx := context.Background()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		require.NoError(t, cmd.Run())
	}
	run("init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(""), 0o644))
	run("add", ".")
	run("commit", "-m", "initial")
}

func TestRunStructuralScan_GoProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Set up a minimal Go project structure.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n\nrequire (\n\tgithub.com/spf13/cobra v1.10.2\n)\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "app", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "foo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal", "foo", "foo.go"), []byte("package foo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal", "foo", "foo_test.go"), []byte("package foo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Project\nA test.\n"), 0o644))

	initTestRepo(t, dir)

	snap, err := RunStructuralScan(context.Background(), dir)
	require.NoError(t, err)

	assert.True(t, snap.IsExisting)
	assert.NotEmpty(t, snap.FileTree)
	assert.True(t, snap.FileStats.TotalFiles >= 5, "expected at least 5 files, got %d", snap.FileStats.TotalFiles)

	// Language detection.
	foundGo := false
	for _, lang := range snap.Languages {
		if lang.Name == "Go" {
			foundGo = true
			assert.Equal(t, "high", lang.Confidence)
			assert.Equal(t, "go.mod", lang.Marker)
		}
	}
	assert.True(t, foundGo, "expected Go language to be detected")

	// Framework detection.
	assert.Contains(t, snap.Frameworks, "Cobra")

	// Entry points.
	foundMain := false
	for _, ep := range snap.EntryPoints {
		if strings.HasSuffix(ep, "main.go") {
			foundMain = true
		}
	}
	assert.True(t, foundMain, "expected main.go entry point")

	// Test dirs.
	assert.NotEmpty(t, snap.TestDirs, "expected test directories to be detected")

	// Doc files.
	foundReadme := false
	for _, doc := range snap.DocFiles {
		if strings.Contains(doc, "README.md") {
			foundReadme = true
		}
	}
	assert.True(t, foundReadme, "expected README.md in doc files")

	// README content.
	assert.Contains(t, snap.ReadmeContent, "# Test Project")

	// Dependencies.
	foundCobra := false
	for _, dep := range snap.Dependencies {
		if dep.Name == "github.com/spf13/cobra" {
			foundCobra = true
			assert.Equal(t, "v1.10.2", dep.Version)
			assert.Equal(t, "go.mod", dep.Source)
		}
	}
	assert.True(t, foundCobra, "expected cobra dependency")

	// Git history.
	assert.NotNil(t, snap.GitHistory)
	assert.True(t, snap.GitHistory.TotalCommits >= 1)
}

func TestRunStructuralScan_NodeProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "name": "test-app",
  "dependencies": {
    "react": "^18.0.0",
    "express": "^4.18.2"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("console.log('hello');\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "app.js"), []byte("module.exports = {};\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "__tests__"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "__tests__", "app.test.js"), []byte("test('it', () => {});\n"), 0o644))

	initTestRepo(t, dir)

	snap, err := RunStructuralScan(context.Background(), dir)
	require.NoError(t, err)

	// Language detection.
	foundJS := false
	for _, lang := range snap.Languages {
		if lang.Name == "JavaScript/TypeScript" && lang.Confidence == "high" {
			foundJS = true
		}
	}
	assert.True(t, foundJS, "expected JavaScript/TypeScript language detected from package.json")

	// Framework detection.
	assert.Contains(t, snap.Frameworks, "React")
	assert.Contains(t, snap.Frameworks, "Express")

	// Dependencies.
	depNames := make(map[string]bool)
	for _, dep := range snap.Dependencies {
		depNames[dep.Name] = true
	}
	assert.True(t, depNames["react"], "expected react dependency")
	assert.True(t, depNames["express"], "expected express dependency")
	assert.True(t, depNames["jest"], "expected jest devDependency")

	// Test dirs.
	foundTestDir := false
	for _, td := range snap.TestDirs {
		if strings.Contains(td, "__tests__") {
			foundTestDir = true
		}
	}
	assert.True(t, foundTestDir, "expected __tests__ directory detected")
}

func TestRunStructuralScan_PythonProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==2.0.1\nrequests>=2.28.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.py"), []byte("from flask import Flask\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "tests"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tests", "test_app.py"), []byte("def test_app(): pass\n"), 0o644))

	initTestRepo(t, dir)

	snap, err := RunStructuralScan(context.Background(), dir)
	require.NoError(t, err)

	// Language detection.
	foundPython := false
	for _, lang := range snap.Languages {
		if lang.Name == "Python" {
			foundPython = true
		}
	}
	assert.True(t, foundPython, "expected Python detected")

	// Framework detection.
	assert.Contains(t, snap.Frameworks, "Flask")

	// Dependencies.
	foundFlask := false
	for _, dep := range snap.Dependencies {
		if dep.Name == "flask" {
			foundFlask = true
			assert.Equal(t, "==2.0.1", dep.Version)
		}
	}
	assert.True(t, foundFlask, "expected flask dependency")

	// Test directories.
	foundTests := false
	for _, td := range snap.TestDirs {
		if strings.Contains(td, "tests") {
			foundTests = true
		}
	}
	assert.True(t, foundTests, "expected tests/ directory detected")
}

func TestRunStructuralScan_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	snap, err := RunStructuralScan(context.Background(), dir)
	require.NoError(t, err)

	assert.Empty(t, snap.FileTree)
	assert.Empty(t, snap.Languages)
	assert.Empty(t, snap.Frameworks)
	assert.Empty(t, snap.EntryPoints)
	assert.Empty(t, snap.Dependencies)
	assert.Equal(t, 0, snap.FileStats.TotalFiles)
	assert.Nil(t, snap.GitHistory, "non-git dir should have nil git history")
}

func TestRunStructuralScan_SkipsHiddenDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden", "secret.go"), []byte("package hidden\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "visible.go"), []byte("package main\n"), 0o644))

	// Use raw walk (no git).
	snap := &StructuralSnapshot{RootDir: dir, IsExisting: true}
	require.NoError(t, rawWalkFileTree(dir, snap))

	for _, f := range snap.FileTree {
		assert.False(t, strings.HasPrefix(f.RelPath, ".hidden"),
			"hidden directory contents should be excluded: %s", f.RelPath)
	}
	assert.Equal(t, 1, len(snap.FileTree), "expected only visible.go")
}

func TestRunStructuralScan_SkipsVendorDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte("//vendor\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.js"), []byte("//app\n"), 0o644))

	// Use raw walk (no git).
	snap := &StructuralSnapshot{RootDir: dir, IsExisting: true}
	require.NoError(t, rawWalkFileTree(dir, snap))

	for _, f := range snap.FileTree {
		assert.False(t, strings.HasPrefix(f.RelPath, "node_modules"),
			"node_modules should be excluded: %s", f.RelPath)
	}
	assert.Equal(t, 1, len(snap.FileTree))
}

func TestWriteFileIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

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
			TotalSize:  300,
			CountByExt: map[string]int{".go": 1, ".mod": 1},
		},
	}

	indexPath := filepath.Join(dir, ".fry", "file-index.txt")
	err := WriteFileIndex(snap, indexPath)
	require.NoError(t, err)

	data, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "# File Index")
	assert.Contains(t, content, "Total files: 2")
	assert.Contains(t, content, "Go (high, go.mod)")
	assert.Contains(t, content, "Cobra")
	assert.Contains(t, content, "cmd/app/main.go")
}

func TestComputeFileStats(t *testing.T) {
	t.Parallel()

	snap := &StructuralSnapshot{
		FileTree: []FileEntry{
			{RelPath: "a.go", Extension: ".go", Size: 100},
			{RelPath: "b.go", Extension: ".go", Size: 200},
			{RelPath: "c.py", Extension: ".py", Size: 50},
			{RelPath: "sub/d.go", Extension: ".go", Size: 300},
		},
	}

	computeFileStats(snap)

	assert.Equal(t, 4, snap.FileStats.TotalFiles)
	assert.Equal(t, int64(650), snap.FileStats.TotalSize)
	assert.Equal(t, 3, snap.FileStats.CountByExt[".go"])
	assert.Equal(t, 1, snap.FileStats.CountByExt[".py"])
	assert.True(t, len(snap.FileStats.LargestFiles) > 0)
	assert.Equal(t, "sub/d.go", snap.FileStats.LargestFiles[0].RelPath, "largest file should be first")
}

func TestDetectLanguages_HighConfidence(t *testing.T) {
	t.Parallel()

	snap := &StructuralSnapshot{
		FileTree: []FileEntry{
			{RelPath: "go.mod", Extension: ".mod", Size: 100},
		},
		FileStats: FileStats{
			CountByExt: map[string]int{".go": 5, ".mod": 1},
		},
	}

	detectLanguages(snap)

	assert.NotEmpty(t, snap.Languages)
	assert.Equal(t, "Go", snap.Languages[0].Name)
	assert.Equal(t, "high", snap.Languages[0].Confidence)
}

func TestDetectLanguages_MediumConfidence(t *testing.T) {
	t.Parallel()

	snap := &StructuralSnapshot{
		FileTree: []FileEntry{}, // No markers.
		FileStats: FileStats{
			CountByExt: map[string]int{".py": 10},
		},
	}

	detectLanguages(snap)

	foundPython := false
	for _, lang := range snap.Languages {
		if lang.Name == "Python" && lang.Confidence == "medium" {
			foundPython = true
		}
	}
	assert.True(t, foundPython, "expected medium-confidence Python detection")
}

func TestParseGoMod(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	goMod := `module example.com/test

go 1.22

require (
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
	github.com/some/indirect v0.1.0 // indirect
)
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644))

	snap := &StructuralSnapshot{}
	parseGoMod(dir, snap)

	// Should include direct deps, exclude indirect.
	assert.Equal(t, 2, len(snap.Dependencies))
	names := make(map[string]bool)
	for _, dep := range snap.Dependencies {
		names[dep.Name] = true
	}
	assert.True(t, names["github.com/spf13/cobra"])
	assert.True(t, names["github.com/stretchr/testify"])
	assert.False(t, names["github.com/some/indirect"])
}

func TestParsePackageJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	pkgJSON := `{
  "name": "test",
  "dependencies": {
    "react": "^18.0.0",
    "express": "^4.18.2"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644))

	snap := &StructuralSnapshot{}
	parsePackageJSON(dir, snap)

	assert.Equal(t, 3, len(snap.Dependencies))
	names := make(map[string]bool)
	for _, dep := range snap.Dependencies {
		names[dep.Name] = true
	}
	assert.True(t, names["react"])
	assert.True(t, names["express"])
	assert.True(t, names["jest"])
}

func TestParseRequirementsTxt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	req := `# Python deps
flask==2.0.1
requests>=2.28.0
-r other.txt
click
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(req), 0o644))

	snap := &StructuralSnapshot{}
	parseRequirementsTxt(dir, snap)

	assert.Equal(t, 3, len(snap.Dependencies))

	depMap := make(map[string]string)
	for _, dep := range snap.Dependencies {
		depMap[dep.Name] = dep.Version
	}
	assert.Equal(t, "==2.0.1", depMap["flask"])
	assert.Equal(t, ">=2.28.0", depMap["requests"])
	assert.Equal(t, "", depMap["click"])
}

func TestReadReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	readme := "# My Project\nThis is a test project.\nLine 3.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644))

	snap := &StructuralSnapshot{}
	readReadme(dir, snap)

	assert.Contains(t, snap.ReadmeContent, "# My Project")
	assert.Contains(t, snap.ReadmeContent, "This is a test project.")
}

func TestReadReadme_NoReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	snap := &StructuralSnapshot{}
	readReadme(dir, snap)

	assert.Empty(t, snap.ReadmeContent)
}

func TestCollectGitHistory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		require.NoError(t, cmd.Run())
	}

	run("init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	run("add", ".")
	run("commit", "-m", "first commit")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644))
	run("add", ".")
	run("commit", "-m", "second commit")

	history, err := collectGitHistory(ctx, dir)
	require.NoError(t, err)

	assert.Equal(t, 2, history.TotalCommits)
	assert.NotEmpty(t, history.RecentLog)
	assert.NotEmpty(t, history.HotFiles)
}

func TestCollectGitHistory_NonGitDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, err := collectGitHistory(context.Background(), dir)
	assert.Error(t, err)
}

func TestFindEntryPoints(t *testing.T) {
	t.Parallel()

	snap := &StructuralSnapshot{
		FileTree: []FileEntry{
			{RelPath: "cmd/app/main.go"},
			{RelPath: "src/index.ts"},
			{RelPath: "lib/utils.go"},
			{RelPath: "server.py"},
		},
	}

	findEntryPoints(snap)

	assert.Contains(t, snap.EntryPoints, "cmd/app/main.go")
	assert.Contains(t, snap.EntryPoints, "src/index.ts")
	assert.Contains(t, snap.EntryPoints, "server.py")
	assert.Equal(t, 3, len(snap.EntryPoints))
}

func TestFindTestDirs(t *testing.T) {
	t.Parallel()

	snap := &StructuralSnapshot{
		FileTree: []FileEntry{
			{RelPath: "internal/foo/foo_test.go"},
			{RelPath: "tests/test_app.py"},
			{RelPath: "__tests__/app.test.js"},
			{RelPath: "src/utils.go"},
		},
	}

	findTestDirs(snap)

	assert.NotEmpty(t, snap.TestDirs)
	// Should detect dirs from test file naming conventions.
	testDirSet := make(map[string]bool)
	for _, td := range snap.TestDirs {
		testDirSet[td] = true
	}
	assert.True(t, testDirSet["internal/foo"], "expected internal/foo from _test.go naming")
	assert.True(t, testDirSet["tests"], "expected tests/ directory")
	assert.True(t, testDirSet["__tests__"], "expected __tests__/ directory")
}

func TestDedup(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"a", "b", "c"}, dedup([]string{"a", "b", "a", "c", "b"}))
	assert.Equal(t, []string(nil), dedup(nil))
	assert.Equal(t, []string{}, dedup([]string{}))
}
