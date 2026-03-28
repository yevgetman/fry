package cli

import (
	"bytes"
	"context"
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

// t.Parallel() omitted: test mutates package-global variables (projectDir, cleanForce)
func TestCleanCmd(t *testing.T) {
	t.Run("force with fry dir archives successfully", func(t *testing.T) {
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("@epic Test\n"), 0o644))

		oldProjectDir := projectDir
		oldCleanForce := cleanForce
		t.Cleanup(func() {
			projectDir = oldProjectDir
			cleanForce = oldCleanForce
		})
		projectDir = dir
		cleanForce = true

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
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
		dir := t.TempDir()

		oldProjectDir := projectDir
		oldCleanForce := cleanForce
		t.Cleanup(func() {
			projectDir = oldProjectDir
			cleanForce = oldCleanForce
		})
		projectDir = dir
		cleanForce = true

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())

		err := cleanCmd.RunE(cmd, []string{})
		require.Error(t, err, "should error when .fry/ does not exist")
	})
}

// ---------------------------------------------------------------------------
// identityCmd
// ---------------------------------------------------------------------------

// t.Parallel() omitted: test reassigns os.Stdout (global state)
func TestIdentityCmd(t *testing.T) {
	t.Run("without full flag", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer r.Close()
		os.Stdout = w
		t.Cleanup(func() { os.Stdout = oldStdout })

		cmd := &cobra.Command{}
		cmd.Flags().Bool("full", false, "")
		cmd.SetContext(context.Background())

		err = identityCmd.RunE(cmd, []string{})
		require.NoError(t, err)

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		assert.NotEmpty(t, buf.String(), "identity output should be non-empty")
	})

	t.Run("with full flag", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer r.Close()
		os.Stdout = w
		t.Cleanup(func() { os.Stdout = oldStdout })

		cmd := &cobra.Command{}
		cmd.Flags().Bool("full", false, "")
		require.NoError(t, cmd.Flags().Set("full", "true"))
		cmd.SetContext(context.Background())

		err = identityCmd.RunE(cmd, []string{})
		require.NoError(t, err)

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		assert.NotEmpty(t, buf.String(), "full identity output should be non-empty")
	})
}

// ---------------------------------------------------------------------------
// prepareCmd --validate-only
// ---------------------------------------------------------------------------

// t.Parallel() omitted: test mutates package-global variables (projectDir, prepareValidateOnly, etc.)
func TestPrepareCmd_ValidateOnly(t *testing.T) {
	t.Run("valid epic validates successfully", func(t *testing.T) {
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

		oldProjectDir := projectDir
		oldValidateOnly := prepareValidateOnly
		oldPrepareUserPrompt := prepareUserPrompt
		oldPrepareUserPromptFile := prepareUserPromptFile
		oldPrepareEffort := prepareEffort
		oldPrepareMode := prepareMode
		oldPreparePlanning := preparePlanning
		t.Cleanup(func() {
			projectDir = oldProjectDir
			prepareValidateOnly = oldValidateOnly
			prepareUserPrompt = oldPrepareUserPrompt
			prepareUserPromptFile = oldPrepareUserPromptFile
			prepareEffort = oldPrepareEffort
			prepareMode = oldPrepareMode
			preparePlanning = oldPreparePlanning
		})

		projectDir = dir
		prepareValidateOnly = true
		prepareUserPrompt = ""
		prepareUserPromptFile = ""
		prepareEffort = ""
		prepareMode = ""
		preparePlanning = false

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := prepareCmd.RunE(cmd, []string{})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Epic validation passed")
	})

	t.Run("missing epic returns error", func(t *testing.T) {
		dir := t.TempDir()

		oldProjectDir := projectDir
		oldValidateOnly := prepareValidateOnly
		oldPrepareUserPrompt := prepareUserPrompt
		oldPrepareUserPromptFile := prepareUserPromptFile
		oldPrepareEffort := prepareEffort
		oldPrepareMode := prepareMode
		oldPreparePlanning := preparePlanning
		t.Cleanup(func() {
			projectDir = oldProjectDir
			prepareValidateOnly = oldValidateOnly
			prepareUserPrompt = oldPrepareUserPrompt
			prepareUserPromptFile = oldPrepareUserPromptFile
			prepareEffort = oldPrepareEffort
			prepareMode = oldPrepareMode
			preparePlanning = oldPreparePlanning
		})

		projectDir = dir
		prepareValidateOnly = true
		prepareUserPrompt = ""
		prepareUserPromptFile = ""
		prepareEffort = ""
		prepareMode = ""
		preparePlanning = false

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())

		err := prepareCmd.RunE(cmd, []string{})
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// replanCmd --dry-run
// ---------------------------------------------------------------------------

// t.Parallel() omitted: test mutates package-global variables (projectDir, replanDryRun, replanEpic)
func TestReplanCmd_DryRun(t *testing.T) {
	t.Run("dry-run with deviation spec but no epic returns error", func(t *testing.T) {
		dir := t.TempDir()

		specPath := filepath.Join(dir, "deviation.md")
		require.NoError(t, os.WriteFile(specPath, []byte("Some deviation text"), 0o644))

		oldProjectDir := projectDir
		oldReplanDryRun := replanDryRun
		oldReplanEpic := replanEpic
		t.Cleanup(func() {
			projectDir = oldProjectDir
			replanDryRun = oldReplanDryRun
			replanEpic = oldReplanEpic
		})

		projectDir = dir
		replanDryRun = true
		replanEpic = filepath.Join(config.FryDir, "epic.md")

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())

		err := replanCmd.RunE(cmd, []string{specPath})
		require.Error(t, err)
	})

	t.Run("dry-run with deviation spec and epic succeeds", func(t *testing.T) {
		dir := t.TempDir()

		specPath := filepath.Join(dir, "deviation.md")
		require.NoError(t, os.WriteFile(specPath, []byte("deviation text"), 0o644))

		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		epicContent := "@epic Test\n@sprint 1\n@name S1\n@max_iterations 1\n@promise DONE\n@prompt\nDo it.\n"
		require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte(epicContent), 0o644))

		oldProjectDir := projectDir
		oldReplanDryRun := replanDryRun
		oldReplanEpic := replanEpic
		t.Cleanup(func() {
			projectDir = oldProjectDir
			replanDryRun = oldReplanDryRun
			replanEpic = oldReplanEpic
		})

		projectDir = dir
		replanDryRun = true
		replanEpic = filepath.Join(config.FryDir, "epic.md")

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := replanCmd.RunE(cmd, []string{specPath})
		require.NoError(t, err)
		assert.NotEmpty(t, buf.String(), "dry-run output should contain the replan prompt")
	})
}
