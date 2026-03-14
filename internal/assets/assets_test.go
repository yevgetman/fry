package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan_NoAssetsDir(t *testing.T) {
	t.Parallel()
	result, err := Scan(t.TempDir())
	assert.NoError(t, err)
	assert.Empty(t, result.Assets)
	assert.False(t, result.Truncated)
}

func TestScan_EmptyAssetsDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o755))
	result, err := Scan(dir)
	assert.NoError(t, err)
	assert.Empty(t, result.Assets)
}

func TestScan_WithTextFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "spec.yaml"), []byte("openapi: 3.0.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "notes.md"), []byte("# Notes\nSome notes.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "data.json"), []byte(`{"key":"val"}`), 0o644))

	result, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, result.Assets, 3)

	// Sorted alphabetically by RelPath.
	assert.Equal(t, filepath.Join("assets", "data.json"), result.Assets[0].RelPath)
	assert.Equal(t, `{"key":"val"}`, result.Assets[0].Content)

	assert.Equal(t, filepath.Join("assets", "notes.md"), result.Assets[1].RelPath)
	assert.Contains(t, result.Assets[1].Content, "# Notes")

	assert.Equal(t, filepath.Join("assets", "spec.yaml"), result.Assets[2].RelPath)
	assert.Contains(t, result.Assets[2].Content, "openapi")
}

func TestScan_SkipsBinaryExtensions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "logo.png"), []byte("png data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "photo.jpg"), []byte("jpg data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "readme.md"), []byte("# Hello"), 0o644))

	result, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, filepath.Join("assets", "readme.md"), result.Assets[0].RelPath)
}

func TestScan_SkipsDotfiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, ".DS_Store"), []byte("junk"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, ".gitkeep"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "spec.yaml"), []byte("key: val"), 0o644))

	result, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, filepath.Join("assets", "spec.yaml"), result.Assets[0].RelPath)
}

func TestScan_SkipsHiddenDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, "assets", ".hidden")
	require.NoError(t, os.MkdirAll(hiddenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "secret.txt"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "visible.txt"), []byte("y"), 0o644))

	result, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, filepath.Join("assets", "visible.txt"), result.Assets[0].RelPath)
}

func TestScan_SkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "real.txt"), []byte("real"), 0o644))
	secretFile := filepath.Join(dir, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("password"), 0o644))
	require.NoError(t, os.Symlink(secretFile, filepath.Join(assetsDir, "evil-link.txt")))

	result, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, filepath.Join("assets", "real.txt"), result.Assets[0].RelPath)
}

func TestScan_MaxFileSizeGuard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))

	// Create a file just over MaxFileSize.
	bigContent := strings.Repeat("x", MaxFileSize+1)
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "big.txt"), []byte(bigContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "small.txt"), []byte("ok"), 0o644))

	result, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, filepath.Join("assets", "small.txt"), result.Assets[0].RelPath)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "exceeds limit")
}

func TestScan_MaxTotalSizeGuard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))

	// Create files that together exceed MaxTotalSize.
	chunkSize := MaxFileSize // 512KB each, so 5 files = 2.5MB > 2MB limit
	chunk := strings.Repeat("a", chunkSize)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("file_%d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(assetsDir, name), []byte(chunk), 0o644))
	}

	result, err := Scan(dir)
	require.NoError(t, err)
	assert.True(t, result.Truncated)
	// 4 files of 512KB each = 2MB exactly fits; the 5th triggers truncation.
	assert.Len(t, result.Assets, 4)
}

func TestScan_MaxFilesGuard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))

	for i := 0; i < MaxFiles+5; i++ {
		name := fmt.Sprintf("file_%05d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(assetsDir, name), []byte("x"), 0o644))
	}

	result, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, result.Assets, MaxFiles)
	assert.True(t, result.Truncated)
}

func TestScan_Subdirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "assets", "specs")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(subDir, "api.yaml"), []byte("openapi: 3.0.0"), 0o644))

	result, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, filepath.Join("assets", "specs", "api.yaml"), result.Assets[0].RelPath)
	assert.Contains(t, result.Assets[0].Content, "openapi")
}

func TestScan_AssetsIsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// "assets" exists but is a regular file, not a directory.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets"), []byte("x"), 0o644))

	result, err := Scan(dir)
	assert.NoError(t, err)
	assert.Empty(t, result.Assets)
}

func TestScan_NonUTF8Content(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))

	// Write binary content with a text extension.
	binaryContent := []byte{0xff, 0xfe, 0x00, 0x01, 0x80, 0x81, 0x82, 0x83}
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "binary.txt"), binaryContent, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "good.txt"), []byte("hello"), 0o644))

	result, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, filepath.Join("assets", "good.txt"), result.Assets[0].RelPath)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "UTF-8")
}

func TestBuildSection_Empty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", BuildSection(ScanResult{}))
}

func TestBuildSection_FormatsCorrectly(t *testing.T) {
	t.Parallel()
	result := ScanResult{
		Assets: []Asset{
			{RelPath: "assets/spec.yaml", Size: 2048, Content: "openapi: 3.0.0\n"},
			{RelPath: "assets/notes.md", Size: 1024, Content: "# Notes\n"},
		},
	}
	section := BuildSection(result)

	assert.Contains(t, section, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, section, "## File: assets/spec.yaml (2.0 KB)")
	assert.Contains(t, section, "```yaml\n")
	assert.Contains(t, section, "openapi: 3.0.0")
	assert.Contains(t, section, "## File: assets/notes.md (1.0 KB)")
	assert.Contains(t, section, "```markdown\n")
	assert.Contains(t, section, "# Notes")
}

func TestBuildSection_NoLanguageTag(t *testing.T) {
	t.Parallel()
	result := ScanResult{
		Assets: []Asset{
			{RelPath: "assets/data.txt", Size: 5, Content: "hello"},
		},
	}
	section := BuildSection(result)
	// .txt has no language tag — should just be bare ```
	assert.Contains(t, section, "```\nhello")
}

func TestExtToLangMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".py", "python"},
		{".yaml", "yaml"},
		{".yml", "yaml"},
		{".json", "json"},
		{".md", "markdown"},
		{".js", "javascript"},
		{".ts", "typescript"},
		{".sql", "sql"},
		{".sh", "bash"},
		{".html", "html"},
		{".txt", ""},
		{".cfg", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, extToLang[tt.ext], "ext: %s", tt.ext)
	}
}

func TestBuildSection_ContentWithBackticks(t *testing.T) {
	t.Parallel()
	result := ScanResult{
		Assets: []Asset{
			{RelPath: "assets/readme.md", Size: 50, Content: "# Hello\n```go\nfunc main() {}\n```\n"},
		},
	}
	section := BuildSection(result)
	// Outer fence should be ```` (4 backticks) since content has ``` (3).
	assert.Contains(t, section, "````markdown\n")
	assert.Contains(t, section, "\n````\n")
	// The inner backticks should remain intact.
	assert.Contains(t, section, "```go\n")
}

func TestFenceFor(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "```", fenceFor("no backticks here"))
	assert.Equal(t, "```", fenceFor("single ` backtick"))
	assert.Equal(t, "```", fenceFor("double `` backticks"))
	assert.Equal(t, "````", fenceFor("triple ``` backticks"))
	assert.Equal(t, "`````", fenceFor("quad ```` backticks"))
	assert.Equal(t, "```", fenceFor(""))
}

func TestAllowedExtensionsHaveLangMapping(t *testing.T) {
	t.Parallel()
	for ext := range allowedExtensions {
		_, ok := extToLang[ext]
		assert.True(t, ok, "allowedExtensions has %q but extToLang does not", ext)
	}
}

func TestFormatSize(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "100 B", formatSize(100))
	assert.Equal(t, "1.5 KB", formatSize(1536))
	assert.Equal(t, "2.0 MB", formatSize(2*1024*1024))
}
