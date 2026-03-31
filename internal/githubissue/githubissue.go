package githubissue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

const (
	maxBodyRunes        = 12000
	maxCommentBodyRunes = 3000
	maxCommentsIncluded = 10
)

type execDeps struct {
	lookPath           func(string) (string, error)
	execCommandContext func(context.Context, string, ...string) *exec.Cmd
}

type Issue struct {
	URL       string
	Host      string
	Owner     string
	Repo      string
	Number    int
	Title     string
	Body      string
	State     string
	Author    string
	Labels    []string
	Comments  []Comment
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Comment struct {
	Author    string
	Body      string
	URL       string
	CreatedAt time.Time
}

type issueRef struct {
	RawURL string
	Host   string
	Owner  string
	Repo   string
	Number int
}

type ghIssuePayload struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	URL       string `json:"url"`
	Number    int    `json:"number"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Author    struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Comments []struct {
		Body      string `json:"body"`
		URL       string `json:"url"`
		CreatedAt string `json:"createdAt"`
		Author    struct {
			Login string `json:"login"`
		} `json:"author"`
	} `json:"comments"`
}

var defaultDeps = execDeps{
	lookPath:           exec.LookPath,
	execCommandContext: exec.CommandContext,
}

func ResolvePrompt(ctx context.Context, projectDir, issueURL string, persist bool) (string, *Issue, error) {
	return resolvePrompt(ctx, projectDir, issueURL, persist, defaultDeps)
}

func resolvePrompt(ctx context.Context, projectDir, issueURL string, persist bool, deps execDeps) (string, *Issue, error) {
	ref, err := parseIssueURL(issueURL)
	if err != nil {
		return "", nil, err
	}
	if err := ensureGHReady(ctx, ref.Host, deps); err != nil {
		return "", nil, err
	}
	issue, err := fetchIssue(ctx, ref, deps)
	if err != nil {
		return "", nil, err
	}
	prompt := renderPrompt(issue)
	if persist {
		if err := os.MkdirAll(filepath.Join(projectDir, config.FryDir), 0o755); err != nil {
			return "", nil, fmt.Errorf("github issue: create fry dir: %w", err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, config.UserPromptFile), []byte(prompt), 0o644); err != nil {
			return "", nil, fmt.Errorf("github issue: write user prompt: %w", err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, config.GitHubIssueFile), []byte(renderMarkdown(issue)), 0o644); err != nil {
			return "", nil, fmt.Errorf("github issue: write issue context: %w", err)
		}
	}
	return prompt, issue, nil
}

func parseIssueURL(raw string) (issueRef, error) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return issueRef{}, fmt.Errorf("github issue: parse URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return issueRef{}, fmt.Errorf("github issue: URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return issueRef{}, fmt.Errorf("github issue: URL host is required")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "issues" {
		return issueRef{}, fmt.Errorf("github issue: URL must match https://<host>/<owner>/<repo>/issues/<number>")
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return issueRef{}, fmt.Errorf("github issue: invalid issue number in URL")
	}
	return issueRef{
		RawURL: trimmed,
		Host:   parsed.Host,
		Owner:  parts[0],
		Repo:   parts[1],
		Number: number,
	}, nil
}

func ensureGHReady(ctx context.Context, host string, deps execDeps) error {
	if _, err := deps.lookPath("gh"); err != nil {
		return fmt.Errorf("github issue: gh CLI not found on PATH")
	}
	cmd := deps.execCommandContext(ctx, "gh", "auth", "status", "--hostname", host)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("github issue: gh auth required for %s: %s", host, msg)
	}
	return nil
}

func fetchIssue(ctx context.Context, ref issueRef, deps execDeps) (*Issue, error) {
	cmd := deps.execCommandContext(ctx, "gh", "issue", "view", ref.RawURL, "--json", "title,body,state,url,number,createdAt,updatedAt,author,labels,comments")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("github issue: fetch %s: %s", ref.RawURL, msg)
	}

	var payload ghIssuePayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return nil, fmt.Errorf("github issue: decode gh response: %w", err)
	}

	issue := &Issue{
		URL:    fallbackString(payload.URL, ref.RawURL),
		Host:   ref.Host,
		Owner:  ref.Owner,
		Repo:   ref.Repo,
		Number: ref.Number,
		Title:  strings.TrimSpace(payload.Title),
		Body:   strings.TrimSpace(payload.Body),
		State:  strings.TrimSpace(payload.State),
		Author: strings.TrimSpace(payload.Author.Login),
	}
	issue.Number = maxInt(issue.Number, payload.Number)
	for _, label := range payload.Labels {
		name := strings.TrimSpace(label.Name)
		if name != "" {
			issue.Labels = append(issue.Labels, name)
		}
	}
	for _, rawComment := range payload.Comments {
		comment := Comment{
			Author: strings.TrimSpace(rawComment.Author.Login),
			Body:   strings.TrimSpace(rawComment.Body),
			URL:    strings.TrimSpace(rawComment.URL),
		}
		if ts, err := time.Parse(time.RFC3339, rawComment.CreatedAt); err == nil {
			comment.CreatedAt = ts
		}
		issue.Comments = append(issue.Comments, comment)
	}
	if ts, err := time.Parse(time.RFC3339, payload.CreatedAt); err == nil {
		issue.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, payload.UpdatedAt); err == nil {
		issue.UpdatedAt = ts
	}
	if issue.Title == "" {
		return nil, fmt.Errorf("github issue: fetched issue is missing a title")
	}
	return issue, nil
}

func renderPrompt(issue *Issue) string {
	var b strings.Builder
	b.WriteString("Treat this GitHub issue as the primary task definition for the build.\n")
	b.WriteString("Generate Fry artifacts and execute work that resolves the issue completely.\n")
	b.WriteString("Preserve explicit requirements, constraints, and acceptance criteria from the issue body and comments.\n")
	b.WriteString("If comments refine the scope, prefer the most recent maintainer guidance.\n\n")
	b.WriteString("GitHub issue metadata:\n")
	b.WriteString(fmt.Sprintf("- URL: %s\n", issue.URL))
	b.WriteString(fmt.Sprintf("- Repository: %s/%s\n", issue.Owner, issue.Repo))
	b.WriteString(fmt.Sprintf("- Issue: #%d\n", issue.Number))
	b.WriteString(fmt.Sprintf("- Title: %s\n", issue.Title))
	if issue.State != "" {
		b.WriteString(fmt.Sprintf("- State: %s\n", issue.State))
	}
	if issue.Author != "" {
		b.WriteString(fmt.Sprintf("- Author: %s\n", issue.Author))
	}
	if len(issue.Labels) > 0 {
		b.WriteString(fmt.Sprintf("- Labels: %s\n", strings.Join(issue.Labels, ", ")))
	}
	b.WriteString("\nIssue body:\n")
	body := strings.TrimSpace(issue.Body)
	if body == "" {
		b.WriteString("(No issue body was provided.)\n")
	} else {
		b.WriteString(truncateRunes(body, maxBodyRunes))
		b.WriteString("\n")
	}
	appendComments(&b, issue.Comments)
	return strings.TrimSpace(b.String())
}

func renderMarkdown(issue *Issue) string {
	var b strings.Builder
	b.WriteString("# GitHub Issue Context\n\n")
	b.WriteString(fmt.Sprintf("- URL: %s\n", issue.URL))
	b.WriteString(fmt.Sprintf("- Repository: %s/%s\n", issue.Owner, issue.Repo))
	b.WriteString(fmt.Sprintf("- Issue: #%d\n", issue.Number))
	b.WriteString(fmt.Sprintf("- Title: %s\n", issue.Title))
	if issue.State != "" {
		b.WriteString(fmt.Sprintf("- State: %s\n", issue.State))
	}
	if issue.Author != "" {
		b.WriteString(fmt.Sprintf("- Author: %s\n", issue.Author))
	}
	if !issue.CreatedAt.IsZero() {
		b.WriteString(fmt.Sprintf("- Created: %s\n", issue.CreatedAt.UTC().Format(time.RFC3339)))
	}
	if !issue.UpdatedAt.IsZero() {
		b.WriteString(fmt.Sprintf("- Updated: %s\n", issue.UpdatedAt.UTC().Format(time.RFC3339)))
	}
	if len(issue.Labels) > 0 {
		b.WriteString(fmt.Sprintf("- Labels: %s\n", strings.Join(issue.Labels, ", ")))
	}
	b.WriteString("\n## Body\n\n")
	if strings.TrimSpace(issue.Body) == "" {
		b.WriteString("_No body provided._\n")
	} else {
		b.WriteString(issue.Body)
		if !strings.HasSuffix(issue.Body, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Comments\n\n")
	if len(issue.Comments) == 0 {
		b.WriteString("_No comments._\n")
		return b.String()
	}
	start := 0
	if len(issue.Comments) > maxCommentsIncluded {
		start = len(issue.Comments) - maxCommentsIncluded
		b.WriteString(fmt.Sprintf("_Showing the most recent %d of %d comments._\n\n", maxCommentsIncluded, len(issue.Comments)))
	}
	for idx, comment := range issue.Comments[start:] {
		b.WriteString(fmt.Sprintf("### Comment %d\n\n", start+idx+1))
		if comment.Author != "" {
			b.WriteString(fmt.Sprintf("- Author: %s\n", comment.Author))
		}
		if !comment.CreatedAt.IsZero() {
			b.WriteString(fmt.Sprintf("- Created: %s\n", comment.CreatedAt.UTC().Format(time.RFC3339)))
		}
		if comment.URL != "" {
			b.WriteString(fmt.Sprintf("- URL: %s\n", comment.URL))
		}
		b.WriteString("\n")
		if comment.Body == "" {
			b.WriteString("_No comment body._\n\n")
			continue
		}
		b.WriteString(comment.Body)
		if !strings.HasSuffix(comment.Body, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func appendComments(b *strings.Builder, comments []Comment) {
	if len(comments) == 0 {
		return
	}
	start := 0
	if len(comments) > maxCommentsIncluded {
		start = len(comments) - maxCommentsIncluded
	}
	b.WriteString("\nRecent issue comments:\n")
	for _, comment := range comments[start:] {
		b.WriteString("\n---\n")
		header := []string{}
		if comment.Author != "" {
			header = append(header, "author: "+comment.Author)
		}
		if !comment.CreatedAt.IsZero() {
			header = append(header, "created: "+comment.CreatedAt.UTC().Format(time.RFC3339))
		}
		if len(header) > 0 {
			b.WriteString(strings.Join(header, " | "))
			b.WriteString("\n")
		}
		body := strings.TrimSpace(comment.Body)
		if body == "" {
			b.WriteString("(No comment body.)\n")
			continue
		}
		b.WriteString(truncateRunes(body, maxCommentBodyRunes))
		b.WriteString("\n")
	}
	if len(comments) > maxCommentsIncluded {
		b.WriteString(fmt.Sprintf("\n(Only the most recent %d comments are included here.)\n", maxCommentsIncluded))
	}
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "..."
}

func fallbackString(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
