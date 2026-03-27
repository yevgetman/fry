package archive

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanArchives_NoArchiveDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	summaries, err := ScanArchives(dir)
	require.NoError(t, err)
	assert.Nil(t, summaries)
}

func TestScanArchives_EmptyArchiveDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry-archive"), 0o755))

	summaries, err := ScanArchives(dir)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestScanArchives_MultipleBuilds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archiveRoot := filepath.Join(dir, ".fry-archive")

	// Create three archived builds with different states
	builds := []struct {
		name       string
		epicMD     string
		progress   string
		mode       string
		exitReason string
	}{
		{
			name:   ".fry--build--20260321-093826",
			epicMD: "@epic First Build\n@sprint Setup\n@sprint Auth\n",
			progress: "# Epic Progress\n## Sprint 1: Setup \u2014 PASS\n## Sprint 2: Auth \u2014 PASS\n",
			mode:   "software",
		},
		{
			name:       ".fry--build--20260322-120000",
			epicMD:     "@epic Second Build\n@sprint Setup\n@sprint API\n@sprint UI\n",
			progress:   "# Epic Progress\n## Sprint 1: Setup \u2014 PASS\n## Sprint 2: API \u2014 FAIL (audit: HIGH)\n",
			mode:       "software",
			exitReason: "After sprint 2: audit failed",
		},
		{
			name:   ".fry--build--20260323-150000",
			epicMD: "@epic Third Build\n@sprint Planning\n",
			mode:   "planning",
		},
	}

	for _, b := range builds {
		bDir := filepath.Join(archiveRoot, b.name)
		require.NoError(t, os.MkdirAll(bDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bDir, "epic.md"), []byte(b.epicMD), 0o644))
		if b.progress != "" {
			require.NoError(t, os.WriteFile(filepath.Join(bDir, "epic-progress.txt"), []byte(b.progress), 0o644))
		}
		if b.mode != "" {
			require.NoError(t, os.WriteFile(filepath.Join(bDir, "build-mode.txt"), []byte(b.mode), 0o644))
		}
		if b.exitReason != "" {
			require.NoError(t, os.WriteFile(filepath.Join(bDir, "build-exit-reason.txt"), []byte(b.exitReason), 0o644))
		}
	}

	summaries, err := ScanArchives(dir)
	require.NoError(t, err)
	require.Len(t, summaries, 3)

	// Should be newest-first
	assert.Equal(t, "Third Build", summaries[0].EpicName)
	assert.Equal(t, 1, summaries[0].TotalSprints)
	assert.Equal(t, 0, summaries[0].CompletedCount)
	assert.Equal(t, "planning", summaries[0].Mode)

	assert.Equal(t, "Second Build", summaries[1].EpicName)
	assert.Equal(t, 3, summaries[1].TotalSprints)
	assert.Equal(t, 1, summaries[1].CompletedCount)
	assert.Equal(t, 1, summaries[1].FailedCount)
	assert.Equal(t, "After sprint 2: audit failed", summaries[1].ExitReason)

	assert.Equal(t, "First Build", summaries[2].EpicName)
	assert.Equal(t, 2, summaries[2].TotalSprints)
	assert.Equal(t, 2, summaries[2].CompletedCount)
	assert.Equal(t, 0, summaries[2].FailedCount)
}

func TestScanArchives_CorruptArchiveSkipped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archiveRoot := filepath.Join(dir, ".fry-archive")

	// One valid, one without epic.md
	validDir := filepath.Join(archiveRoot, ".fry--build--20260321-100000")
	require.NoError(t, os.MkdirAll(validDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(validDir, "epic.md"), []byte("@epic Valid\n@sprint S1\n"), 0o644))

	corruptDir := filepath.Join(archiveRoot, ".fry--build--20260322-100000")
	require.NoError(t, os.MkdirAll(corruptDir, 0o755))
	// No epic.md here

	summaries, err := ScanArchives(dir)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, "Valid", summaries[0].EpicName)
}

func TestScanArchives_IgnoresNonArchiveEntries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archiveRoot := filepath.Join(dir, ".fry-archive")

	// Create a directory without the archive prefix
	require.NoError(t, os.MkdirAll(filepath.Join(archiveRoot, "random-dir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(archiveRoot, "random-dir", "epic.md"), []byte("@epic Nope\n"), 0o644))

	// Create a file (not dir) with the prefix
	require.NoError(t, os.MkdirAll(archiveRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(archiveRoot, ".fry--build--20260101-000000"), []byte("not a dir"), 0o644))

	summaries, err := ScanArchives(dir)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestScanBuildDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epic.md"), []byte("@epic My Epic\n@sprint Setup\n@sprint Build\n@sprint Deploy\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epic-progress.txt"), []byte("## Sprint 1: Setup \u2014 PASS\n## Sprint 2: Build \u2014 FAIL\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build-mode.txt"), []byte("software\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build-exit-reason.txt"), []byte("audit failed\n"), 0o644))

	summary, err := ScanBuildDir(dir)
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, "My Epic", summary.EpicName)
	assert.Equal(t, 3, summary.TotalSprints)
	assert.Equal(t, 1, summary.CompletedCount)
	assert.Equal(t, 1, summary.FailedCount)
	assert.Equal(t, "software", summary.Mode)
	assert.Equal(t, "audit failed", summary.ExitReason)
}

func TestScanBuildDir_NoEpic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	summary, err := ScanBuildDir(dir)
	require.NoError(t, err)
	assert.Nil(t, summary)
}

func TestScanBuildDir_EmptyEpicName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// epic.md exists but has no @epic directive
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epic.md"),
		[]byte("# Some heading\n@sprint S1\n@sprint S2\n"), 0o644))

	summary, err := ScanBuildDir(dir)
	require.NoError(t, err)
	assert.Nil(t, summary)
}

func TestParseTimestamp(t *testing.T) {
	t.Parallel()

	ts, err := parseTimestamp(".fry--build--20260327-074606")
	require.NoError(t, err)
	assert.Equal(t, 2026, ts.Year())
	assert.Equal(t, time.March, ts.Month())
	assert.Equal(t, 27, ts.Day())
	assert.Equal(t, 7, ts.Hour())
	assert.Equal(t, 46, ts.Minute())
	assert.Equal(t, 6, ts.Second())
}

func TestParseTimestamp_Invalid(t *testing.T) {
	t.Parallel()

	_, err := parseTimestamp(".fry--build--not-a-date")
	assert.Error(t, err)
}
