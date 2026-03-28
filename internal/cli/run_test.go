package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/sprint"
	"github.com/yevgetman/fry/internal/verify"
)

// ---------------------------------------------------------------------------
// writeDeferredFailuresArtifact / readDeferredFailuresArtifact
// ---------------------------------------------------------------------------

func TestWriteDeferredFailuresArtifact(t *testing.T) {
	t.Parallel()

	t.Run("non-empty entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		entries := []deferredEntry{
			{
				SprintNumber: 2,
				SprintName:   "Build widgets",
				Failures: []verify.CheckResult{
					{Check: verify.Check{Type: 1, Path: "widget.go"}, Passed: false, Output: "missing file"},
				},
			},
		}
		writeDeferredFailuresArtifact(dir, entries)

		path := filepath.Join(dir, config.DeferredFailuresFile)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "# Deferred Sanity Check Failures")
		assert.Contains(t, content, "## Sprint 2: Build widgets")
	})

	t.Run("empty entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeDeferredFailuresArtifact(dir, nil)

		path := filepath.Join(dir, config.DeferredFailuresFile)
		_, err := os.Stat(path)
		assert.NoError(t, err, "file should be created even with empty entries")
	})
}

func TestReadDeferredFailuresArtifact(t *testing.T) {
	t.Parallel()

	t.Run("absent file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		got := readDeferredFailuresArtifact(dir)
		assert.Equal(t, "", got)
	})

	t.Run("round-trip", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		entries := []deferredEntry{
			{
				SprintNumber: 1,
				SprintName:   "Init",
				Failures: []verify.CheckResult{
					{Check: verify.Check{Type: 1, Path: "a.go"}, Passed: false, Output: "err"},
				},
			},
		}
		writeDeferredFailuresArtifact(dir, entries)
		got := readDeferredFailuresArtifact(dir)
		assert.Contains(t, got, "# Deferred Sanity Check Failures")
		assert.Contains(t, got, "## Sprint 1: Init")
	})
}

// ---------------------------------------------------------------------------
// writeExitReason
// ---------------------------------------------------------------------------

func TestWriteExitReason(t *testing.T) {
	t.Parallel()

	t.Run("nil error removes file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, config.BuildExitReasonFile)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("old reason"), 0o644))

		writeExitReason(dir, nil, 0)

		_, err := os.Stat(path)
		assert.True(t, os.IsNotExist(err), "file should be removed on nil error")
	})

	t.Run("error with sprint", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeExitReason(dir, errors.New("timeout"), 3)

		path := filepath.Join(dir, config.BuildExitReasonFile)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "After sprint 3: timeout", string(data))
	})

	t.Run("error without sprint", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeExitReason(dir, errors.New("bad config"), 0)

		path := filepath.Join(dir, config.BuildExitReasonFile)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "bad config", string(data))
	})
}

// ---------------------------------------------------------------------------
// allSprintsCompleted
// ---------------------------------------------------------------------------

func TestAllSprintsCompleted(t *testing.T) {
	t.Parallel()

	t.Run("startSprint 1 always true", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		assert.True(t, allSprintsCompleted(dir, 5, 1, nil))
	})

	t.Run("all prior sprints in epic-progress", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		// Write epic-progress.txt with sprints 1 and 2 completed.
		// The regex is: ^## Sprint (\d+):\s*(.+?)\s*—\s*(PASS.*)$
		content := "# Epic Progress\n\n" +
			"## Sprint 1: Setup — PASS\n\n" +
			"## Sprint 2: Build — PASS\n\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.EpicProgressFile), []byte(content), 0o644))

		// Starting at sprint 3 with total 3 — sprints 1 and 2 covered by epic-progress,
		// sprint 3 covered by current results.
		results := []sprint.SprintResult{{Number: 3, Status: "PASS"}}
		assert.True(t, allSprintsCompleted(dir, 3, 3, results))
	})

	t.Run("missing sprint returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		// Only sprint 1, but sprint 2 is missing.
		content := "# Epic Progress\n\n## Sprint 1: Setup — PASS\n\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.EpicProgressFile), []byte(content), 0o644))

		results := []sprint.SprintResult{{Number: 3, Status: "PASS"}}
		assert.False(t, allSprintsCompleted(dir, 3, 3, results))
	})

	t.Run("no epic-progress file returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		assert.False(t, allSprintsCompleted(dir, 3, 2, nil))
	})
}

// ---------------------------------------------------------------------------
// resolveProjectDir
// ---------------------------------------------------------------------------

func TestResolveProjectDir(t *testing.T) {
	t.Parallel()

	t.Run("empty string returns abs of cwd", func(t *testing.T) {
		t.Parallel()
		got, err := resolveProjectDir("")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got))
		assert.NotEmpty(t, got)
	})

	t.Run("dot returns absolute path", func(t *testing.T) {
		t.Parallel()
		got, err := resolveProjectDir(".")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got))
	})

	t.Run("absolute path returned as-is", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		got, err := resolveProjectDir(dir)
		require.NoError(t, err)
		assert.Equal(t, dir, got)
	})
}

// ---------------------------------------------------------------------------
// resolveUserPrompt
// ---------------------------------------------------------------------------

func TestResolveUserPrompt(t *testing.T) {
	t.Parallel()

	t.Run("both provided and promptFile returns error", func(t *testing.T) {
		t.Parallel()
		_, err := resolveUserPrompt(t.TempDir(), "hello", "/some/file", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot use both")
	})

	t.Run("promptFile reads file contents", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		f := filepath.Join(dir, "prompt.txt")
		require.NoError(t, os.WriteFile(f, []byte("from file"), 0o644))

		got, err := resolveUserPrompt(dir, "", f, false)
		require.NoError(t, err)
		assert.Equal(t, "from file", got)
	})

	t.Run("provided with persist writes to UserPromptFile", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		got, err := resolveUserPrompt(dir, "my prompt", "", true)
		require.NoError(t, err)
		assert.Equal(t, "my prompt", got)

		data, err := os.ReadFile(filepath.Join(dir, config.UserPromptFile))
		require.NoError(t, err)
		assert.Equal(t, "my prompt", string(data))
	})

	t.Run("provided without persist does not write file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		got, err := resolveUserPrompt(dir, "my prompt", "", false)
		require.NoError(t, err)
		assert.Equal(t, "my prompt", got)

		_, err = os.Stat(filepath.Join(dir, config.UserPromptFile))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("no input reads persisted file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, config.UserPromptFile)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("persisted"), 0o644))

		got, err := resolveUserPrompt(dir, "", "", false)
		require.NoError(t, err)
		assert.Equal(t, "persisted", got)
	})

	t.Run("no input no persisted file returns empty", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		got, err := resolveUserPrompt(dir, "", "", false)
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})
}

// ---------------------------------------------------------------------------
// resolveAuditEngine
// ---------------------------------------------------------------------------

func TestResolveAuditEngine(t *testing.T) {
	t.Parallel()

	t.Run("non-empty auditEngineName uses it", func(t *testing.T) {
		t.Parallel()
		eng, err := resolveAuditEngine("claude", "codex")
		require.NoError(t, err)
		assert.Equal(t, "codex", eng.Name())
	})

	t.Run("empty auditEngineName uses buildEngineName", func(t *testing.T) {
		t.Parallel()
		eng, err := resolveAuditEngine("claude", "")
		require.NoError(t, err)
		assert.Equal(t, "claude", eng.Name())
	})

	t.Run("invalid engine returns error", func(t *testing.T) {
		t.Parallel()
		_, err := resolveAuditEngine("nonexistent", "")
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// resolveReviewEngine
// ---------------------------------------------------------------------------

func TestResolveReviewEngine(t *testing.T) {
	t.Parallel()

	t.Run("non-empty reviewEngineName uses it", func(t *testing.T) {
		t.Parallel()
		eng, err := resolveReviewEngine("claude", "codex")
		require.NoError(t, err)
		assert.Equal(t, "codex", eng.Name())
	})

	t.Run("empty reviewEngineName uses buildEngineName", func(t *testing.T) {
		t.Parallel()
		eng, err := resolveReviewEngine("claude", "")
		require.NoError(t, err)
		assert.Equal(t, "claude", eng.Name())
	})

	t.Run("invalid engine returns error", func(t *testing.T) {
		t.Parallel()
		_, err := resolveReviewEngine("nonexistent", "")
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// printMigrationHintIfNeeded
// ---------------------------------------------------------------------------

func TestPrintMigrationHintIfNeeded(t *testing.T) {
	t.Parallel()

	t.Run("no migration needed", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		var buf []byte
		w := &bytesWriter{buf: &buf}
		// No root-level epic.md → no hint.
		printMigrationHintIfNeeded(w, dir, "epic.md")
		assert.Empty(t, string(*w.buf))
	})

	t.Run("migration hint printed", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Create root-level epic.md (old layout).
		require.NoError(t, os.WriteFile(filepath.Join(dir, "epic.md"), []byte("old"), 0o644))
		// No .fry/epic.md → hint should print.

		var buf []byte
		w := &bytesWriter{buf: &buf}
		printMigrationHintIfNeeded(w, dir, "epic.md")
		assert.Contains(t, string(*w.buf), "old layout")
	})

	t.Run("no hint when both exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Create both root-level and .fry/ epic.md — no hint.
		require.NoError(t, os.WriteFile(filepath.Join(dir, "epic.md"), []byte("old"), 0o644))
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("new"), 0o644))

		var buf []byte
		w := &bytesWriter{buf: &buf}
		printMigrationHintIfNeeded(w, dir, "epic.md")
		assert.Empty(t, string(*w.buf))
	})
}

// bytesWriter is a minimal io.Writer that appends to a byte slice.
type bytesWriter struct {
	buf *[]byte
}

func (bw *bytesWriter) Write(p []byte) (int, error) {
	*bw.buf = append(*bw.buf, p...)
	return len(p), nil
}
