package scan

// StructuralSnapshot holds the result of a deterministic, no-LLM scan of an
// existing project directory. It captures file layout, language/framework
// detection, dependency manifests, and git history — the raw material that a
// semantic scan (Phase 2) will feed to an LLM for deeper understanding.
type StructuralSnapshot struct {
	RootDir      string
	IsExisting   bool
	Languages    []Language
	Frameworks   []string
	EntryPoints  []string
	Dependencies []Dependency
	TestDirs     []string
	DocFiles     []string
	FileTree     []FileEntry
	FileStats    FileStats
	GitHistory   *GitHistory
	// ReadmeContent holds the first MaxReadmeLines lines of the project README
	// (if present). Empty string if no README was found.
	ReadmeContent string
}

// Language represents a detected programming language and the evidence for it.
type Language struct {
	Name       string // e.g. "Go", "Python", "TypeScript"
	Confidence string // "high" (marker file) or "medium" (extension count)
	Marker     string // e.g. "go.mod", "package.json", or "*.py (42 files)"
}

// Dependency represents a parsed dependency from a project manifest.
type Dependency struct {
	Name    string // e.g. "github.com/spf13/cobra", "react"
	Version string // e.g. "v1.10.2", "^18.0.0"
	Source  string // manifest file it was parsed from, e.g. "go.mod"
}

// FileEntry represents a single file in the project tree.
type FileEntry struct {
	RelPath   string // relative to project root
	Extension string // e.g. ".go", ".ts"
	Size      int64
	IsDir     bool
}

// FileStats holds aggregate statistics about the file tree.
type FileStats struct {
	TotalFiles     int
	TotalDirs      int
	TotalSize      int64
	CountByExt     map[string]int // extension → file count
	LargestFiles   []FileEntry    // top 10 by size
	TopDirectories []DirStat      // top 10 directories by file count
}

// DirStat holds file count for a single directory.
type DirStat struct {
	RelPath   string
	FileCount int
}

// GitHistory holds information extracted from the git log.
type GitHistory struct {
	TotalCommits int
	RecentLog    []string  // git log --oneline, most recent N
	HotFiles     []HotFile // files that change most frequently
	TopAuthors   []string  // git shortlog -sn, top contributors
}

// HotFile is a file ranked by how often it appears in recent commits.
type HotFile struct {
	RelPath     string
	ChangeCount int
}
