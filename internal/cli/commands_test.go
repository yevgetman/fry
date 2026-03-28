package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

// ---------------------------------------------------------------------------
// cleanCmd
// ---------------------------------------------------------------------------

func TestCleanCmd(t *testing.T) {
	t.Parallel()

	t.Run("force with fry dir archives successfully", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("@epic Test\n"), 0o644))

		cmd := newTestCmd(t, dir)
		cmd.Flags().Bool("force", false, "")
		_ = cmd.Flags().Set("force", "true")
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := cleanCmd.RunE(cmd, []string{})
		require.NoError(t, err)

		// .fry/ should have been moved to .fry-archive/
		_, err = os.Stat(fryDir)
		assert.True(t, os.IsNotExist(err), ".fry/ should be gone after archive")

		archiveDir := filepath.Join(dir, config.ArchiveDir)
		entries, err := os.ReadDir(archiveDir)
		require.NoError(t, err)
		assert.NotEmpty(t, entries, "archive directory should contain an entry")
	})

	t.Run("force without fry dir returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		cmd := newTestCmd(t, dir)
		cmd.Flags().Bool("force", false, "")
		_ = cmd.Flags().Set("force", "true")

		err := cleanCmd.RunE(cmd, []string{})
		require.Error(t, err, "should error when .fry/ does not exist")
	})

	t.Run("lock warning printed with force", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("@epic Test\n"), 0o644))

		// Create lock file with current PID so lock.IsLocked returns true
		lockPath := filepath.Join(dir, config.LockFile)
		require.NoError(t, os.WriteFile(lockPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644))

		cmd := newTestCmd(t, dir)
		cmd.Flags().Bool("force", false, "")
		_ = cmd.Flags().Set("force", "true")
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)

		err := cleanCmd.RunE(cmd, []string{})
		require.NoError(t, err)

		assert.Contains(t, errBuf.String(), "warning", "lock warning should be printed to stderr")
	})
}

// ---------------------------------------------------------------------------
// identityCmd
// ---------------------------------------------------------------------------

func TestIdentityCmd(t *testing.T) {
	t.Parallel()

	t.Run("without full flag", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		cmd.Flags().Bool("full", false, "")
		cmd.SetContext(context.Background())
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := identityCmd.RunE(cmd, []string{})
		require.NoError(t, err)
		assert.NotEmpty(t, buf.String(), "identity output should be non-empty")
	})

	t.Run("with full flag", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		cmd.Flags().Bool("full", false, "")
		require.NoError(t, cmd.Flags().Set("full", "true"))
		cmd.SetContext(context.Background())
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := identityCmd.RunE(cmd, []string{})
		require.NoError(t, err)
		assert.NotEmpty(t, buf.String(), "full identity output should be non-empty")
	})
}

// ---------------------------------------------------------------------------
// prepareCmd --validate-only
// ---------------------------------------------------------------------------

func TestPrepareCmd_ValidateOnly(t *testing.T) {
	t.Parallel()

	t.Run("valid epic validates successfully", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))

		epicContent := "@epic Test Epic\n" +
			"@sprint 1\n" +
			"@name Setup\n" +
			"@max_iterations 2\n" +
			"@promise DONE\n" +
			"@prompt\n" +
			"Do the thing.\n"
		require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte(epicContent), 0o644))

		cmd := newTestCmd(t, dir)
		cmd.Flags().Bool("validate-only", false, "")
		_ = cmd.Flags().Set("validate-only", "true")
		cmd.Flags().String("user-prompt", "", "")
		cmd.Flags().String("user-prompt-file", "", "")
		cmd.Flags().String("effort", "", "")
		cmd.Flags().String("mode", "", "")
		cmd.Flags().Bool("planning", false, "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := prepareCmd.RunE(cmd, []string{})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Epic validation passed")
	})

	t.Run("missing epic returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		cmd := newTestCmd(t, dir)
		cmd.Flags().Bool("validate-only", false, "")
		_ = cmd.Flags().Set("validate-only", "true")
		cmd.Flags().String("user-prompt", "", "")
		cmd.Flags().String("user-prompt-file", "", "")
		cmd.Flags().String("effort", "", "")
		cmd.Flags().String("mode", "", "")
		cmd.Flags().Bool("planning", false, "")

		err := prepareCmd.RunE(cmd, []string{})
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// replanCmd --dry-run
// ---------------------------------------------------------------------------

func TestReplanCmd_DryRun(t *testing.T) {
	t.Parallel()

	t.Run("dry-run with deviation spec but no epic returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		specPath := filepath.Join(dir, "deviation.md")
		require.NoError(t, os.WriteFile(specPath, []byte("Some deviation text"), 0o644))

		cmd := newTestCmd(t, dir)
		cmd.Flags().Bool("dry-run", false, "")
		_ = cmd.Flags().Set("dry-run", "true")
		cmd.Flags().String("epic", filepath.Join(config.FryDir, "epic.md"), "")

		err := replanCmd.RunE(cmd, []string{specPath})
		require.Error(t, err)
	})

	t.Run("dry-run with deviation spec and epic succeeds", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		specPath := filepath.Join(dir, "deviation.md")
		require.NoError(t, os.WriteFile(specPath, []byte("deviation text"), 0o644))

		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		epicContent := "@epic Test\n@sprint 1\n@name S1\n@max_iterations 1\n@promise DONE\n@prompt\nDo it.\n"
		require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte(epicContent), 0o644))

		cmd := newTestCmd(t, dir)
		cmd.Flags().Bool("dry-run", false, "")
		_ = cmd.Flags().Set("dry-run", "true")
		cmd.Flags().String("epic", filepath.Join(config.FryDir, "epic.md"), "")
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := replanCmd.RunE(cmd, []string{specPath})
		require.NoError(t, err)
		assert.NotEmpty(t, buf.String(), "dry-run output should contain the replan prompt")
	})
}
