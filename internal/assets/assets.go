package assets

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yevgetman/fry/internal/config"
)

const (
	// MaxFileSize is the maximum size of a single asset file (512 KB).
	MaxFileSize = 512 * 1024
	// MaxTotalSize is the maximum aggregate size across all asset files (2 MB).
	MaxTotalSize = 2 * 1024 * 1024
	// MaxFiles is the maximum number of files the scanner will collect.
	MaxFiles = 100
	// utf8CheckSize is how many bytes we read to validate UTF-8 encoding.
	utf8CheckSize = 512
)

// Asset represents a single text file in the assets/ directory with its content loaded.
type Asset struct {
	RelPath string // path relative to project root, e.g. "assets/api-spec.yaml"
	Size    int64
	Content string
}

// ScanResult holds the outcome of scanning the assets/ directory.
type ScanResult struct {
	Assets    []Asset
	Truncated bool     // true if MaxFiles or MaxTotalSize was hit
	Warnings  []string // non-fatal issues encountered during scan
}

// allowedExtensions lists file extensions that are treated as readable text.
var allowedExtensions = map[string]bool{
	".md": true, ".txt": true, ".rst": true,
	".csv": true, ".tsv": true,
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
	".html": true, ".htm": true, ".css": true,
	".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".go": true, ".py": true, ".rb": true, ".java": true, ".sql": true,
	".sh": true, ".bash": true, ".zsh": true,
	".cfg": true, ".ini": true, ".conf": true, ".properties": true,
	".log": true,
	".proto": true, ".graphql": true, ".gql": true,
}

// extToLang maps file extensions to markdown fence language identifiers.
var extToLang = map[string]string{
	".md": "markdown", ".txt": "", ".rst": "rst",
	".csv": "csv", ".tsv": "tsv",
	".json": "json", ".yaml": "yaml", ".yml": "yaml", ".toml": "toml", ".xml": "xml",
	".html": "html", ".htm": "html", ".css": "css",
	".js": "javascript", ".ts": "typescript", ".jsx": "jsx", ".tsx": "tsx",
	".go": "go", ".py": "python", ".rb": "ruby", ".java": "java", ".sql": "sql",
	".sh": "bash", ".bash": "bash", ".zsh": "zsh",
	".cfg": "", ".ini": "ini", ".conf": "", ".properties": "properties",
	".log": "",
	".proto": "protobuf", ".graphql": "graphql", ".gql": "graphql",
}

// Scan walks the assets/ directory and returns all readable text assets.
// Returns an empty ScanResult (not an error) if the directory does not exist.
// Symlinks are skipped. Hidden files/directories are skipped.
func Scan(projectDir string) (ScanResult, error) {
	var result ScanResult

	assetsPath := filepath.Join(projectDir, config.AssetsDir)
	info, err := os.Lstat(assetsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, fmt.Errorf("scan assets: stat: %w", err)
	}
	if !info.IsDir() {
		return result, nil
	}

	var totalSize int64

	err = filepath.WalkDir(assetsPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		name := d.Name()

		// Skip hidden files and directories.
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip symlinks.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Validate the relative path stays within assets/.
		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return relErr
		}
		cleaned := filepath.Clean(rel)
		if strings.HasPrefix(cleaned, "..") || !strings.HasPrefix(cleaned, config.AssetsDir+string(filepath.Separator)) {
			return nil
		}

		// Check extension allowlist.
		ext := strings.ToLower(filepath.Ext(name))
		if !allowedExtensions[ext] {
			return nil
		}

		fi, fiErr := d.Info()
		if fiErr != nil {
			return fiErr
		}

		// Enforce per-file size limit.
		if fi.Size() > MaxFileSize {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("skipping %s: file size %s exceeds limit %s",
					rel, formatSize(fi.Size()), formatSize(MaxFileSize)))
			return nil
		}

		// Enforce aggregate size limit.
		if totalSize+fi.Size() > MaxTotalSize {
			result.Truncated = true
			return filepath.SkipAll
		}

		// Enforce max file count.
		if len(result.Assets) >= MaxFiles {
			result.Truncated = true
			return filepath.SkipAll
		}

		// Read the file.
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("skipping %s: read error: %v", rel, readErr))
			return nil
		}

		// Validate UTF-8 encoding on first bytes.
		checkLen := len(data)
		if checkLen > utf8CheckSize {
			checkLen = utf8CheckSize
		}
		if checkLen > 0 && !utf8.Valid(data[:checkLen]) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("skipping %s: file does not appear to be valid UTF-8 text", rel))
			return nil
		}

		result.Assets = append(result.Assets, Asset{
			RelPath: rel,
			Size:    fi.Size(),
			Content: string(data),
		})
		totalSize += fi.Size()

		return nil
	})
	if err != nil {
		return result, fmt.Errorf("scan assets: walk: %w", err)
	}

	sort.Slice(result.Assets, func(i, j int) bool {
		return result.Assets[i].RelPath < result.Assets[j].RelPath
	})

	return result, nil
}

// BuildSection generates the formatted prompt section with all asset contents.
// Returns empty string if no assets were loaded.
func BuildSection(result ScanResult) string {
	if len(result.Assets) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("# ===== SUPPLEMENTARY ASSETS =====\n")
	b.WriteString("# The assets/ directory contains reference documents provided as additional context.\n")
	b.WriteString("# Use this information to inform your design decisions, architecture, and implementation details.\n\n")

	for i, a := range result.Assets {
		if i > 0 {
			b.WriteString("\n")
		}
		ext := strings.ToLower(filepath.Ext(a.RelPath))
		lang := extToLang[ext]
		fence := fenceFor(a.Content)

		b.WriteString(fmt.Sprintf("## File: %s (%s)\n", a.RelPath, formatSize(a.Size)))
		if lang != "" {
			b.WriteString(fmt.Sprintf("%s%s\n", fence, lang))
		} else {
			b.WriteString(fence + "\n")
		}
		b.WriteString(ensureTrailingNewline(a.Content))
		b.WriteString(fence + "\n")
	}

	return b.String()
}

// fenceFor returns a backtick fence string that is safe to use around content.
// It finds the longest consecutive run of backticks in the content and returns
// a fence one backtick longer (minimum 3).
func fenceFor(content string) string {
	maxRun := 0
	currentRun := 0
	for _, ch := range content {
		if ch == '`' {
			currentRun++
			if currentRun > maxRun {
				maxRun = currentRun
			}
		} else {
			currentRun = 0
		}
	}
	fenceLen := maxRun + 1
	if fenceLen < 3 {
		fenceLen = 3
	}
	return strings.Repeat("`", fenceLen)
}

func ensureTrailingNewline(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func formatSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
}
