package media

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yevgetman/fry/internal/config"
)

// MaxAssets is the maximum number of files the scanner will collect before
// truncating. This prevents runaway memory usage on very large media dirs.
const MaxAssets = 10000

// Asset represents a single file in the media directory.
type Asset struct {
	// RelPath is the path relative to the project root (e.g. "media/logo.png").
	RelPath  string
	Category string
	Size     int64
}

var categoryExtensions = map[string][]string{
	"image":    {".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".bmp", ".tiff", ".tif"},
	"document": {".pdf", ".docx", ".doc", ".xlsx", ".xls", ".pptx", ".ppt", ".csv", ".tsv"},
	"design":   {".fig", ".sketch", ".psd", ".ai", ".xd"},
	"data":     {".json", ".yaml", ".yml", ".xml", ".toml"},
	"video":    {".mp4", ".mov", ".avi", ".webm", ".mkv"},
	"audio":    {".mp3", ".wav", ".ogg", ".flac", ".aac"},
	"font":     {".ttf", ".otf", ".woff", ".woff2", ".eot"},
}

// extToCategory is a flat lookup table built at init for O(1) categorization.
var extToCategory = func() map[string]string {
	m := make(map[string]string)
	for cat, exts := range categoryExtensions {
		for _, ext := range exts {
			m[ext] = cat
		}
	}
	return m
}()

func categorize(ext string) string {
	if cat, ok := extToCategory[strings.ToLower(ext)]; ok {
		return cat
	}
	return "other"
}

// Scan walks the media directory and returns all assets found.
// Returns nil, false, nil if the media directory does not exist.
// Symlinks are skipped. Hidden files/directories (names starting with '.') are skipped.
// At most MaxAssets files are returned; the truncated return value indicates whether
// more files existed beyond the cap.
func Scan(projectDir string) (assets []Asset, truncated bool, err error) {
	mediaPath := filepath.Join(projectDir, config.MediaDir)
	info, err := os.Lstat(mediaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("scan media: stat: %w", err)
	}
	if !info.IsDir() {
		return nil, false, nil
	}
	// Note: os.Lstat returns ModeSymlink for symlinks, and IsDir() returns
	// false for symlinks (even those pointing to directories), so a symlinked
	// media/ directory is already handled by the IsDir() check above.

	err = filepath.WalkDir(mediaPath, func(path string, d fs.DirEntry, walkErr error) error {
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

		// Skip symlinks. WalkDir does not follow symlinks, so both file
		// and directory symlinks are reported with ModeSymlink set.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Validate the relative path stays within media/.
		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return relErr
		}
		cleaned := filepath.Clean(rel)
		if strings.HasPrefix(cleaned, "..") || !strings.HasPrefix(cleaned, config.MediaDir+string(filepath.Separator)) {
			// Path escaped media/ boundary — skip silently.
			return nil
		}

		// Enforce max file count.
		if len(assets) >= MaxAssets {
			truncated = true
			return filepath.SkipAll
		}

		fi, fiErr := d.Info()
		if fiErr != nil {
			return fiErr
		}

		assets = append(assets, Asset{
			RelPath:  rel,
			Category: categorize(filepath.Ext(name)),
			Size:     fi.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("scan media: walk: %w", err)
	}

	sort.Slice(assets, func(i, j int) bool {
		if assets[i].Category != assets[j].Category {
			return assets[i].Category < assets[j].Category
		}
		return assets[i].RelPath < assets[j].RelPath
	})

	return assets, truncated, nil
}

// BuildManifest generates a formatted manifest string listing all media assets
// grouped by category. Returns empty string if no assets exist.
func BuildManifest(assets []Asset) string {
	if len(assets) == 0 {
		return ""
	}

	var b strings.Builder
	currentCategory := ""
	for _, a := range assets {
		if a.Category != currentCategory {
			if currentCategory != "" {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("## %s\n", capitalize(a.Category)))
			currentCategory = a.Category
		}
		b.WriteString(fmt.Sprintf("- `%s` (%s)\n", a.RelPath, formatSize(a.Size)))
	}
	return b.String()
}

// PromptSection returns the full prompt section for media assets, or empty
// string if no media directory or assets exist.
func PromptSection(projectDir string) string {
	assets, _, err := Scan(projectDir)
	if err != nil || len(assets) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# ===== MEDIA ASSETS =====\n")
	b.WriteString("# The media/ directory contains assets that may be referenced in the plan\n")
	b.WriteString("# or executive context. Use these files as instructed — copy them into the\n")
	b.WriteString("# appropriate locations in the project, reference them in code or documents,\n")
	b.WriteString("# or use them as design/content inputs.\n\n")
	b.WriteString(BuildManifest(assets))
	b.WriteString("\n")
	return b.String()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
