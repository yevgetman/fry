package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/githubissue"
)

func TestPrepareValidateOnlySkipsGitHubIssueResolution(t *testing.T) {
	oldResolver := resolveGitHubIssuePrompt
	t.Cleanup(func() { resolveGitHubIssuePrompt = oldResolver })

	resolveGitHubIssuePrompt = func(context.Context, string, string, bool) (string, *githubissue.Issue, error) {
		t.Fatal("github issue resolver should not run during --validate-only")
		return "", nil, nil
	}

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry", "epic.md"), []byte(`
@epic Validate Only
@engine codex
@verification .fry/verification.md

@sprint 1
@name Validate
@max_iterations 1
@promise VALIDATE_ONLY
@prompt
Validate only.
@end
`), 0o644))

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{
		"prepare",
		"--project-dir", dir,
		"--validate-only",
		"--gh-issue", "https://github.com/yevgetman/fry/issues/74",
	})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Epic validation passed")
}
