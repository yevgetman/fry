package scan

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

const (
	// MaxTreeFiles caps the number of files collected during the tree walk
	// to prevent runaway memory on very large repositories.
	MaxTreeFiles = 50000

	// MaxReadmeLines is the maximum number of README lines included in the
	// snapshot for downstream LLM consumption.
	MaxReadmeLines = 500

	// recentLogCount is how many recent git log entries to capture.
	recentLogCount = 30

	// hotFileCount is how many "hot files" (most-changed) to capture.
	hotFileCount = 20

	// topAuthorsCount limits the author shortlog.
	topAuthorsCount = 10

	// topDirsCount limits the top-directories-by-file-count list.
	topDirsCount = 10

	// largestFilesCount limits the largest-files list.
	largestFilesCount = 10
)

// readmeNames are filenames recognized as project READMEs, in priority order.
var readmeNames = []string{
	"README.md", "README.MD", "readme.md",
	"README", "README.txt", "README.rst",
}

// knownTestDirNames are directory names that typically contain tests.
var knownTestDirNames = map[string]bool{
	"test": true, "tests": true, "spec": true, "specs": true,
	"__tests__": true, "__test__": true, "test_": true,
	"testing": true, "e2e": true, "integration": true,
}

// knownDocFiles are filenames recognized as project documentation.
var knownDocFiles = map[string]bool{
	"README.md": true, "README.MD": true, "readme.md": true,
	"README": true, "README.txt": true, "README.rst": true,
	"CONTRIBUTING.md": true, "CHANGELOG.md": true, "CHANGES.md": true,
	"LICENSE": true, "LICENSE.md": true, "LICENSE.txt": true,
	"ARCHITECTURE.md": true, "DESIGN.md": true,
	"AGENTS.md": true, "CLAUDE.md": true,
}

// entryPointNames are filenames commonly used as application entry points.
var entryPointNames = map[string]bool{
	"main.go": true, "main.py": true, "main.rs": true, "main.ts": true, "main.js": true,
	"app.py": true, "app.ts": true, "app.js": true, "app.rb": true,
	"index.ts": true, "index.js": true, "index.html": true,
	"server.go": true, "server.py": true, "server.ts": true, "server.js": true,
	"manage.py": true, "Program.cs": true,
}

// RunStructuralScan walks the project directory and collects a StructuralSnapshot.
// This is a pure filesystem + git operation — no LLM calls are made.
func RunStructuralScan(ctx context.Context, dir string) (*StructuralSnapshot, error) {
	snap := &StructuralSnapshot{
		RootDir:    dir,
		IsExisting: true,
	}

	if err := walkFileTree(ctx, dir, snap); err != nil {
		return nil, fmt.Errorf("structural scan: walk: %w", err)
	}

	computeFileStats(snap)
	detectLanguages(snap)
	detectFrameworks(dir, snap)
	findEntryPoints(snap)
	findTestDirs(snap)
	findDocFiles(dir, snap)
	parseDependencies(dir, snap)
	readReadme(dir, snap)

	if gitHistory, err := collectGitHistory(ctx, dir); err == nil {
		snap.GitHistory = gitHistory
	}

	return snap, nil
}

// walkFileTree populates snap.FileTree by walking the directory tree,
// respecting .gitignore via git ls-files when possible.
func walkFileTree(ctx context.Context, dir string, snap *StructuralSnapshot) error {
	// Try git-aware listing first; fall back to raw walk.
	if entries, err := gitAwareFileList(ctx, dir); err == nil && len(entries) > 0 {
		snap.FileTree = entries
		return nil
	}
	return rawWalkFileTree(dir, snap)
}

// gitAwareFileList uses git ls-files to list tracked + untracked files,
// which naturally respects .gitignore.
func gitAwareFileList(ctx context.Context, dir string) ([]FileEntry, error) {
	// Tracked files.
	tracked, err := gitOutputCtx(ctx, dir, "ls-files")
	if err != nil {
		return nil, err
	}
	// Untracked but not ignored.
	untracked, err := gitOutputCtx(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		untracked = ""
	}

	seen := make(map[string]bool)
	var entries []FileEntry

	for _, block := range []string{tracked, untracked} {
		scanner := bufio.NewScanner(strings.NewReader(block))
		for scanner.Scan() {
			relPath := strings.TrimSpace(scanner.Text())
			if relPath == "" || seen[relPath] {
				continue
			}
			seen[relPath] = true

			if len(entries) >= MaxTreeFiles {
				break
			}

			absPath := filepath.Join(dir, relPath)
			info, err := os.Lstat(absPath)
			if err != nil {
				continue
			}
			// Skip symlinks.
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			entries = append(entries, FileEntry{
				RelPath:   relPath,
				Extension: strings.ToLower(filepath.Ext(relPath)),
				Size:      info.Size(),
				IsDir:     false,
			})
		}
	}

	return entries, nil
}

// rawWalkFileTree is the fallback when git is unavailable.
func rawWalkFileTree(dir string, snap *StructuralSnapshot) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		name := d.Name()

		// Skip hidden directories and files.
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip known generated/vendor directories.
		if d.IsDir() && isSkippableDir(name) {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Skip symlinks.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if len(snap.FileTree) >= MaxTreeFiles {
			return filepath.SkipAll
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		snap.FileTree = append(snap.FileTree, FileEntry{
			RelPath:   rel,
			Extension: strings.ToLower(filepath.Ext(name)),
			Size:      info.Size(),
			IsDir:     false,
		})

		return nil
	})
}

// skippableDirs is the set of directory names skipped during raw file tree
// walks (vendor, build output, fry artifacts). Package-level to avoid
// allocation on every WalkDir callback.
var skippableDirs = map[string]bool{
	"node_modules": true, "vendor": true, "__pycache__": true,
	"dist": true, "build": true, "target": true, "bin": true,
	".fry": true, ".fry-archive": true, ".fry-worktrees": true,
}

// isSkippableDir returns true for directories that should be skipped during
// raw file tree walks (vendor, build output, etc.).
func isSkippableDir(name string) bool {
	return skippableDirs[name]
}

// computeFileStats fills in snap.FileStats from snap.FileTree.
func computeFileStats(snap *StructuralSnapshot) {
	stats := FileStats{
		CountByExt: make(map[string]int),
	}

	dirSet := make(map[string]int) // dir → file count

	for _, f := range snap.FileTree {
		stats.TotalFiles++
		stats.TotalSize += f.Size

		if f.Extension != "" {
			stats.CountByExt[f.Extension]++
		}

		dir := filepath.Dir(f.RelPath)
		dirSet[dir]++
	}

	// Count distinct directories.
	stats.TotalDirs = len(dirSet)

	// Top directories by file count.
	type kv struct {
		dir   string
		count int
	}
	var dirs []kv
	for d, c := range dirSet {
		dirs = append(dirs, kv{d, c})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].count > dirs[j].count })
	for i := 0; i < len(dirs) && i < topDirsCount; i++ {
		stats.TopDirectories = append(stats.TopDirectories, DirStat{
			RelPath:   dirs[i].dir,
			FileCount: dirs[i].count,
		})
	}

	// Largest files.
	sorted := make([]FileEntry, len(snap.FileTree))
	copy(sorted, snap.FileTree)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Size > sorted[j].Size })
	for i := 0; i < len(sorted) && i < largestFilesCount; i++ {
		stats.LargestFiles = append(stats.LargestFiles, sorted[i])
	}

	snap.FileStats = stats
}

// detectLanguages populates snap.Languages from project markers and extension counts.
func detectLanguages(snap *StructuralSnapshot) {
	seen := make(map[string]bool)

	// High-confidence: project marker files present in the tree.
	for _, f := range snap.FileTree {
		base := filepath.Base(f.RelPath)
		if lang, ok := projectMarkers[base]; ok && !seen[lang] {
			seen[lang] = true
			snap.Languages = append(snap.Languages, Language{
				Name:       lang,
				Confidence: "high",
				Marker:     base,
			})
		}
	}

	// Medium-confidence: significant extension counts.
	extToLang := map[string]string{
		".go": "Go", ".py": "Python", ".js": "JavaScript", ".ts": "TypeScript",
		".rb": "Ruby", ".rs": "Rust", ".java": "Java", ".kt": "Kotlin",
		".cs": "C#", ".cpp": "C++", ".c": "C", ".swift": "Swift",
		".php": "PHP", ".ex": "Elixir", ".exs": "Elixir", ".dart": "Dart",
		".scala": "Scala", ".hs": "Haskell", ".lua": "Lua", ".r": "R",
		".tsx": "TypeScript", ".jsx": "JavaScript",
	}
	threshold := 3
	for ext, count := range snap.FileStats.CountByExt {
		if lang, ok := extToLang[ext]; ok && count >= threshold && !seen[lang] {
			seen[lang] = true
			snap.Languages = append(snap.Languages, Language{
				Name:       lang,
				Confidence: "medium",
				Marker:     fmt.Sprintf("%s (%d files)", ext, count),
			})
		}
	}

	sort.Slice(snap.Languages, func(i, j int) bool {
		if snap.Languages[i].Confidence != snap.Languages[j].Confidence {
			return snap.Languages[i].Confidence == "high"
		}
		return snap.Languages[i].Name < snap.Languages[j].Name
	})
}

// detectFrameworks checks for known framework indicators.
func detectFrameworks(dir string, snap *StructuralSnapshot) {
	// Check package.json for JS/TS frameworks.
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		content := string(data)
		frameworks := map[string]string{
			"react":   "React",
			"vue":     "Vue",
			"angular": "Angular",
			"next":    "Next.js",
			"nuxt":    "Nuxt",
			"express": "Express",
			"fastify": "Fastify",
			"svelte":  "Svelte",
			"nest":    "NestJS",
		}
		for key, name := range frameworks {
			if strings.Contains(content, `"`+key) {
				snap.Frameworks = append(snap.Frameworks, name)
			}
		}
	}

	// Check go.mod for Go frameworks.
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		content := string(data)
		frameworks := map[string]string{
			"github.com/gin-gonic/gin":  "Gin",
			"github.com/labstack/echo":  "Echo",
			"github.com/gofiber/fiber":  "Fiber",
			"github.com/spf13/cobra":    "Cobra",
			"github.com/gorilla/mux":    "Gorilla Mux",
			"github.com/go-chi/chi":     "Chi",
		}
		for key, name := range frameworks {
			if strings.Contains(content, key) {
				snap.Frameworks = append(snap.Frameworks, name)
			}
		}
	}

	// Check requirements.txt / pyproject.toml for Python frameworks.
	for _, pyFile := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		if data, err := os.ReadFile(filepath.Join(dir, pyFile)); err == nil {
			content := strings.ToLower(string(data))
			frameworks := map[string]string{
				"django":  "Django",
				"flask":   "Flask",
				"fastapi": "FastAPI",
			}
			for key, name := range frameworks {
				if strings.Contains(content, key) {
					snap.Frameworks = append(snap.Frameworks, name)
				}
			}
		}
	}

	sort.Strings(snap.Frameworks)
	snap.Frameworks = dedup(snap.Frameworks)
}

// findEntryPoints scans the file tree for common entry point filenames.
func findEntryPoints(snap *StructuralSnapshot) {
	for _, f := range snap.FileTree {
		base := filepath.Base(f.RelPath)
		if entryPointNames[base] {
			snap.EntryPoints = append(snap.EntryPoints, f.RelPath)
		}
	}
	sort.Strings(snap.EntryPoints)
}

// findTestDirs identifies directories that appear to contain tests.
func findTestDirs(snap *StructuralSnapshot) {
	dirs := make(map[string]bool)
	for _, f := range snap.FileTree {
		dir := filepath.Dir(f.RelPath)
		base := filepath.Base(f.RelPath)

		// Check if any path component is a test directory.
		parts := strings.Split(dir, string(filepath.Separator))
		for _, p := range parts {
			if knownTestDirNames[strings.ToLower(p)] {
				dirs[dir] = true
				break
			}
		}

		// Check for test file naming conventions.
		lower := strings.ToLower(base)
		if strings.HasSuffix(lower, "_test.go") ||
			strings.HasPrefix(lower, "test_") ||
			strings.HasSuffix(lower, ".test.ts") ||
			strings.HasSuffix(lower, ".test.js") ||
			strings.HasSuffix(lower, ".test.tsx") ||
			strings.HasSuffix(lower, ".spec.ts") ||
			strings.HasSuffix(lower, ".spec.js") ||
			strings.HasSuffix(lower, "_test.py") ||
			strings.HasSuffix(lower, "_spec.rb") {
			dirs[dir] = true
		}
	}
	for d := range dirs {
		snap.TestDirs = append(snap.TestDirs, d)
	}
	sort.Strings(snap.TestDirs)
}

// findDocFiles identifies documentation files in the project.
func findDocFiles(dir string, snap *StructuralSnapshot) {
	// Top-level doc files.
	for _, f := range snap.FileTree {
		base := filepath.Base(f.RelPath)
		if knownDocFiles[base] {
			snap.DocFiles = append(snap.DocFiles, f.RelPath)
		}
	}

	// Check for docs/ directory.
	docsPath := filepath.Join(dir, "docs")
	if info, err := os.Stat(docsPath); err == nil && info.IsDir() {
		snap.DocFiles = append(snap.DocFiles, "docs/")
	}

	sort.Strings(snap.DocFiles)
	snap.DocFiles = dedup(snap.DocFiles)
}

// parseDependencies reads manifest files and extracts dependency information.
func parseDependencies(dir string, snap *StructuralSnapshot) {
	parseGoMod(dir, snap)
	parsePackageJSON(dir, snap)
	parseRequirementsTxt(dir, snap)
}

func parseGoMod(dir string, snap *StructuralSnapshot) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return
	}
	inRequire := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "require (") || line == "require(" {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]
				// Skip indirect dependencies.
				if len(parts) >= 4 && parts[2] == "//" && parts[3] == "indirect" {
					continue
				}
				snap.Dependencies = append(snap.Dependencies, Dependency{
					Name:    name,
					Version: version,
					Source:  "go.mod",
				})
			}
			continue
		}
		// Single-line require.
		if strings.HasPrefix(line, "require ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				snap.Dependencies = append(snap.Dependencies, Dependency{
					Name:    parts[1],
					Version: parts[2],
					Source:  "go.mod",
				})
			}
		}
	}
}

func parsePackageJSON(dir string, snap *StructuralSnapshot) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return
	}
	// Simple extraction without a JSON parser to avoid adding dependencies.
	// Look for "dependencies" and "devDependencies" sections.
	content := string(data)
	for _, section := range []string{"dependencies", "devDependencies"} {
		idx := strings.Index(content, `"`+section+`"`)
		if idx < 0 {
			continue
		}
		// Find the opening brace.
		braceStart := strings.Index(content[idx:], "{")
		if braceStart < 0 {
			continue
		}
		braceStart += idx
		// Find matching closing brace.
		depth := 0
		braceEnd := -1
		for i := braceStart; i < len(content); i++ {
			if content[i] == '{' {
				depth++
			} else if content[i] == '}' {
				depth--
				if depth == 0 {
					braceEnd = i
					break
				}
			}
		}
		if braceEnd < 0 {
			continue
		}
		block := content[braceStart+1 : braceEnd]
		// Parse "name": "version" pairs.
		lines := strings.Split(block, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimSuffix(line, ",")
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			name := strings.Trim(strings.TrimSpace(parts[0]), `"`)
			version := strings.Trim(strings.TrimSpace(parts[1]), `"`)
			if name != "" && version != "" {
				snap.Dependencies = append(snap.Dependencies, Dependency{
					Name:    name,
					Version: version,
					Source:  "package.json",
				})
			}
		}
	}
}

func parseRequirementsTxt(dir string, snap *StructuralSnapshot) {
	data, err := os.ReadFile(filepath.Join(dir, "requirements.txt"))
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		// Split on ==, >=, ~=, !=, <=, or plain name.
		for _, sep := range []string{"==", ">=", "~=", "!=", "<="} {
			if idx := strings.Index(line, sep); idx > 0 {
				snap.Dependencies = append(snap.Dependencies, Dependency{
					Name:    strings.TrimSpace(line[:idx]),
					Version: sep + strings.TrimSpace(line[idx+len(sep):]),
					Source:  "requirements.txt",
				})
				goto nextLine
			}
		}
		// No version specifier.
		snap.Dependencies = append(snap.Dependencies, Dependency{
			Name:    line,
			Version: "",
			Source:  "requirements.txt",
		})
	nextLine:
	}
}

// readReadme loads the project README content (truncated to MaxReadmeLines).
func readReadme(dir string, snap *StructuralSnapshot) {
	for _, name := range readmeNames {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) > MaxReadmeLines {
			lines = lines[:MaxReadmeLines]
		}
		snap.ReadmeContent = strings.Join(lines, "\n")
		return
	}
}

// collectGitHistory gathers commit log, hot files, and author info.
func collectGitHistory(ctx context.Context, dir string) (*GitHistory, error) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repo")
	}

	history := &GitHistory{}

	// Total commit count.
	if out, err := gitOutputCtx(ctx, dir, "rev-list", "--count", "HEAD"); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(out)); err == nil {
			history.TotalCommits = n
		}
	}

	// Recent log.
	if out, err := gitOutputCtx(ctx, dir, "log", "--oneline", fmt.Sprintf("-%d", recentLogCount)); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				history.RecentLog = append(history.RecentLog, line)
			}
		}
	}

	// Hot files: most frequently changed files in git history.
	if out, err := gitOutputCtx(ctx, dir, "log", "--format=", "--name-only", fmt.Sprintf("-%d", 100)); err == nil {
		fileCounts := make(map[string]int)
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fileCounts[line]++
			}
		}
		type kv struct {
			file  string
			count int
		}
		var sorted []kv
		for f, c := range fileCounts {
			sorted = append(sorted, kv{f, c})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })
		for i := 0; i < len(sorted) && i < hotFileCount; i++ {
			history.HotFiles = append(history.HotFiles, HotFile{
				RelPath:     sorted[i].file,
				ChangeCount: sorted[i].count,
			})
		}
	}

	// Top authors.
	if out, err := gitOutputCtx(ctx, dir, "shortlog", "-sn", "--all", fmt.Sprintf("-%d", topAuthorsCount)); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				history.TopAuthors = append(history.TopAuthors, line)
			}
		}
	}

	return history, nil
}

// WriteFileIndex writes a human-readable file index to the specified path.
func WriteFileIndex(snap *StructuralSnapshot, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create file index dir: %w", err)
	}

	var b strings.Builder
	b.WriteString("# File Index\n")
	b.WriteString("# Generated by fry structural scan\n")
	b.WriteString(fmt.Sprintf("# Total files: %d | Total size: %s\n\n",
		snap.FileStats.TotalFiles, FormatSize(snap.FileStats.TotalSize)))

	// Languages.
	if len(snap.Languages) > 0 {
		b.WriteString("## Languages\n")
		for _, lang := range snap.Languages {
			b.WriteString(fmt.Sprintf("- %s (%s, %s)\n", lang.Name, lang.Confidence, lang.Marker))
		}
		b.WriteString("\n")
	}

	// Frameworks.
	if len(snap.Frameworks) > 0 {
		b.WriteString("## Frameworks\n")
		for _, fw := range snap.Frameworks {
			b.WriteString(fmt.Sprintf("- %s\n", fw))
		}
		b.WriteString("\n")
	}

	// Entry points.
	if len(snap.EntryPoints) > 0 {
		b.WriteString("## Entry Points\n")
		for _, ep := range snap.EntryPoints {
			b.WriteString(fmt.Sprintf("- %s\n", ep))
		}
		b.WriteString("\n")
	}

	// File tree (abbreviated for large projects).
	b.WriteString("## File Tree\n")
	displayCount := len(snap.FileTree)
	if displayCount > 500 {
		displayCount = 500
	}
	for i := 0; i < displayCount; i++ {
		f := snap.FileTree[i]
		b.WriteString(fmt.Sprintf("%s (%s)\n", f.RelPath, FormatSize(f.Size)))
	}
	if len(snap.FileTree) > 500 {
		b.WriteString(fmt.Sprintf("... and %d more files\n", len(snap.FileTree)-500))
	}
	b.WriteString("\n")

	// Stats.
	b.WriteString("## Stats by Extension\n")
	type extCount struct {
		ext   string
		count int
	}
	var exts []extCount
	for ext, count := range snap.FileStats.CountByExt {
		exts = append(exts, extCount{ext, count})
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].count > exts[j].count })
	for _, ec := range exts {
		b.WriteString(fmt.Sprintf("  %s: %d\n", ec.ext, ec.count))
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// RefreshFileIndexIfStale regenerates .fry/file-index.txt if it is older than
// the most recent git commit. Returns true if the index was regenerated.
func RefreshFileIndexIfStale(ctx context.Context, projectDir string) bool {
	indexPath := filepath.Join(projectDir, config.FileIndexFile)
	indexStat, err := os.Stat(indexPath)
	if err != nil {
		// No index file — nothing to refresh (fry init wasn't run).
		return false
	}

	// Get the timestamp of the latest git commit.
	out, gitErr := gitOutputCtx(ctx, projectDir, "log", "-1", "--format=%ct")
	if gitErr != nil {
		return false
	}
	epochStr := strings.TrimSpace(out)
	epoch, parseErr := strconv.ParseInt(epochStr, 10, 64)
	if parseErr != nil {
		return false
	}
	headTime := time.Unix(epoch, 0)

	if headTime.After(indexStat.ModTime()) {
		snap, scanErr := RunStructuralScan(ctx, projectDir)
		if scanErr != nil {
			return false
		}
		if writeErr := WriteFileIndex(snap, indexPath); writeErr != nil {
			return false
		}
		return true
	}
	return false
}

// gitOutputCtx runs a git command with context and returns stdout.
func gitOutputCtx(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}

// FormatSize returns a human-readable file size string.
func FormatSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
}

func dedup(s []string) []string {
	if len(s) == 0 {
		return s
	}
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
