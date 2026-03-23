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
)

func newTestCmd(t *testing.T, projectDir string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String("project-dir", projectDir, "")
	cmd.SetContext(context.Background())
	return cmd
}

func TestStatusCommandNoBuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No active build")
}

func TestStatusCommandWithBuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fryDir := filepath.Join(dir, ".fry")
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("@epic Test Epic\n"), 0o644))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}
