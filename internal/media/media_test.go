package media

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan_NoMediaDir(t *testing.T) {
	t.Parallel()
	assets, _, err := Scan(t.TempDir())
	assert.NoError(t, err)
	assert.Nil(t, assets)
}

func TestScan_EmptyMediaDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "media"), 0o755))
	assets, _, err := Scan(dir)
	assert.NoError(t, err)
	assert.Empty(t, assets)
}

func TestScan_WithFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, "logo.png"), []byte("png data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, "spec.pdf"), []byte("pdf data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, "config.yaml"), []byte("key: val"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, "readme.txt"), []byte("text"), 0o644))

	assets, _, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, assets, 4)

	categories := make(map[string]int)
	for _, a := range assets {
		categories[a.Category]++
	}
	assert.Equal(t, 1, categories["data"])
	assert.Equal(t, 1, categories["document"])
	assert.Equal(t, 1, categories["image"])
	assert.Equal(t, 1, categories["other"])
}

func TestScan_Subdirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "media", "icons")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(subDir, "arrow.svg"), []byte("<svg/>"), 0o644))

	assets, _, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, filepath.Join("media", "icons", "arrow.svg"), assets[0].RelPath)
	assert.Equal(t, "image", assets[0].Category)
}

func TestScan_SkipsDotfiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, ".DS_Store"), []byte("junk"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, ".gitkeep"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, "logo.png"), []byte("img"), 0o644))

	assets, _, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, filepath.Join("media", "logo.png"), assets[0].RelPath)
}

func TestScan_SkipsHiddenDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, "media", ".hidden")
	require.NoError(t, os.MkdirAll(hiddenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "secret.txt"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "media", "visible.png"), []byte("y"), 0o644))

	assets, _, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, filepath.Join("media", "visible.png"), assets[0].RelPath)
}

func TestScan_SkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0o755))

	// Create a real file and a symlink to something outside media/.
	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, "real.png"), []byte("img"), 0o644))
	secretFile := filepath.Join(dir, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("password"), 0o644))
	require.NoError(t, os.Symlink(secretFile, filepath.Join(mediaDir, "evil-link")))

	assets, _, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, filepath.Join("media", "real.png"), assets[0].RelPath)
}

func TestScan_MaxAssetsGuard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0o755))

	// Create MaxAssets + 5 files to test truncation.
	for i := 0; i < MaxAssets+5; i++ {
		name := filepath.Join(mediaDir, fmt.Sprintf("file_%05d.txt", i))
		require.NoError(t, os.WriteFile(name, []byte("x"), 0o644))
	}

	assets, truncated, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, assets, MaxAssets)
	assert.True(t, truncated)
}

func TestScan_MediaIsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// "media" exists but is a regular file, not a directory.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "media"), []byte("x"), 0o644))

	assets, _, err := Scan(dir)
	assert.NoError(t, err)
	assert.Nil(t, assets)
}

func TestBuildManifest_Empty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", BuildManifest(nil))
}

func TestBuildManifest_GroupsByCategory(t *testing.T) {
	t.Parallel()
	assets := []Asset{
		{RelPath: "media/data.json", Category: "data", Size: 512},
		{RelPath: "media/logo.png", Category: "image", Size: 2048},
		{RelPath: "media/icon.svg", Category: "image", Size: 1024},
	}
	manifest := BuildManifest(assets)
	assert.Contains(t, manifest, "## Data")
	assert.Contains(t, manifest, "## Image")
	assert.Contains(t, manifest, "`media/logo.png`")
	assert.Contains(t, manifest, "`media/icon.svg`")
	assert.Contains(t, manifest, "`media/data.json`")
}

func TestPromptSection_NoMedia(t *testing.T) {
	t.Parallel()
	section := PromptSection(t.TempDir())
	assert.Equal(t, "", section)
}

func TestPromptSection_WithMedia(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mediaDir, "wireframe.png"), []byte("img"), 0o644))

	section := PromptSection(dir)
	assert.Contains(t, section, "MEDIA ASSETS")
	assert.Contains(t, section, "media/wireframe.png")
}

func TestPromptSection_ErrorReturnsEmpty(t *testing.T) {
	t.Parallel()
	// Non-existent project dir triggers stat error -> returns empty.
	section := PromptSection("/nonexistent/path/that/does/not/exist")
	assert.Equal(t, "", section)
}

func TestCategorize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ext      string
		expected string
	}{
		{".png", "image"},
		{".PNG", "image"},
		{".jpg", "image"},
		{".svg", "image"},
		{".pdf", "document"},
		{".csv", "document"},
		{".json", "data"},
		{".yaml", "data"},
		{".mp4", "video"},
		{".mp3", "audio"},
		{".ttf", "font"},
		{".woff2", "font"},
		{".xyz", "other"},
		{".go", "other"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, categorize(tt.ext), "ext: %s", tt.ext)
	}
}

func TestFormatSize(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "100 B", formatSize(100))
	assert.Equal(t, "1.5 KB", formatSize(1536))
	assert.Equal(t, "2.0 MB", formatSize(2*1024*1024))
}

