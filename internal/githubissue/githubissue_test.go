package githubissue

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestParseIssueURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantRef issueRef
		wantErr string
	}{
		{
			name:  "github.com issue",
			input: "https://github.com/yevgetman/fry/issues/74",
			wantRef: issueRef{
				RawURL: "https://github.com/yevgetman/fry/issues/74",
				Host:   "github.com",
				Owner:  "yevgetman",
				Repo:   "fry",
				Number: 74,
			},
		},
		{
			name:    "missing scheme",
			input:   "github.com/yevgetman/fry/issues/74",
			wantErr: "URL must start",
		},
		{
			name:    "pull request URL rejected",
			input:   "https://github.com/yevgetman/fry/pull/74",
			wantErr: "must match",
		},
		{
			name:    "invalid number",
			input:   "https://github.com/yevgetman/fry/issues/nope",
			wantErr: "invalid issue number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseIssueURL(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRef, got)
		})
	}
}

func TestEnsureGHReady(t *testing.T) {
	t.Parallel()

	t.Run("missing gh binary", func(t *testing.T) {
		t.Parallel()

		err := ensureGHReady(context.Background(), "github.com", execDeps{
			lookPath: func(string) (string, error) { return "", errors.New("missing") },
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gh CLI not found")
	})

	t.Run("auth failure", func(t *testing.T) {
		t.Parallel()

		err := ensureGHReady(context.Background(), "github.com", execDeps{
			lookPath: func(string) (string, error) { return "/usr/bin/gh", nil },
			execCommandContext: func(_ context.Context, _ string, _ ...string) *exec.Cmd {
				return exec.Command("sh", "-c", "echo not logged in 1>&2; exit 1")
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gh auth required")
		assert.Contains(t, err.Error(), "not logged in")
	})
}

func TestResolvePrompt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	issueJSON := `{
		"title": "Support GitHub issue input",
		"body": "Allow Fry to start from an issue URL.",
		"state": "OPEN",
		"url": "https://github.com/yevgetman/fry/issues/74",
		"number": 74,
		"createdAt": "2026-03-30T10:00:00Z",
		"updatedAt": "2026-03-31T11:00:00Z",
		"author": {"login": "julie"},
		"labels": [{"name": "enhancement"}, {"name": "cli"}],
		"comments": [
			{
				"body": "Please make this work from the root command too.",
				"url": "https://github.com/yevgetman/fry/issues/74#issuecomment-1",
				"createdAt": "2026-03-31T11:30:00Z",
				"author": {"login": "maintainer"}
			}
		]
	}`

	prompt, issue, err := resolvePrompt(context.Background(), dir, "https://github.com/yevgetman/fry/issues/74", true, execDeps{
		lookPath: func(name string) (string, error) {
			require.Equal(t, "gh", name)
			return "/usr/bin/gh", nil
		},
		execCommandContext: func(_ context.Context, name string, args ...string) *exec.Cmd {
			require.Equal(t, "gh", name)
			cmdline := strings.Join(args, " ")
			switch {
			case strings.Contains(cmdline, "auth status"):
				return exec.Command("sh", "-c", "exit 0")
			case strings.Contains(cmdline, "issue view"):
				return exec.Command("sh", "-c", "cat <<'JSON'\n"+issueJSON+"\nJSON\n")
			default:
				t.Fatalf("unexpected gh command: %s %v", name, args)
				return nil
			}
		},
	})
	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Contains(t, prompt, "Support GitHub issue input")
	assert.Contains(t, prompt, "Please make this work from the root command too.")
	assert.Equal(t, "julie", issue.Author)
	assert.Equal(t, []string{"enhancement", "cli"}, issue.Labels)

	userPromptBytes, err := os.ReadFile(filepath.Join(dir, config.UserPromptFile))
	require.NoError(t, err)
	assert.Equal(t, prompt, string(userPromptBytes))

	issueBytes, err := os.ReadFile(filepath.Join(dir, config.GitHubIssueFile))
	require.NoError(t, err)
	assert.Contains(t, string(issueBytes), "# GitHub Issue Context")
	assert.Contains(t, string(issueBytes), "https://github.com/yevgetman/fry/issues/74")
}

func TestRenderMarkdownTruncatesCommentsToMostRecentWindow(t *testing.T) {
	t.Parallel()

	issue := &Issue{
		URL:    "https://github.com/yevgetman/fry/issues/74",
		Owner:  "yevgetman",
		Repo:   "fry",
		Number: 74,
		Title:  "Test",
	}
	for i := 0; i < maxCommentsIncluded+2; i++ {
		issue.Comments = append(issue.Comments, Comment{
			Author:    "user",
			Body:      "comment body",
			CreatedAt: time.Date(2026, 3, 31, 12, i, 0, 0, time.UTC),
		})
	}

	markdown := renderMarkdown(issue)
	assert.Contains(t, markdown, "most recent")
	assert.NotContains(t, markdown, "### Comment 1\n")
	assert.NotContains(t, markdown, "### Comment 2\n")
	assert.Contains(t, markdown, "### Comment 12")
}
