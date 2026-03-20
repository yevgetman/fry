package continuerun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
)

func TestSevRank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected int
	}{
		{"CRITICAL", 4},
		{"HIGH", 3},
		{"MODERATE", 2},
		{"LOW", 1},
		{"", 0},
		{"UNKNOWN", 0},
		{"low", 0},       // case-sensitive
		{"critical", 0},  // case-sensitive
		{"Medium", 0},    // not a valid severity
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, sevRank(tt.input))
		})
	}
}

func TestExtractMaxSeverity_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "no severity lines",
			content:  "This is just regular text.\nNo findings here.\n",
			expected: "",
		},
		{
			name:     "severity keyword without label",
			content:  "This CRITICAL bug needs fixing.\nHIGH priority task.\n",
			expected: "",
		},
		{
			name:     "multiple severities on same line picks first match",
			content:  "**Severity:** LOW to HIGH",
			expected: "LOW",
		},
		{
			name:     "critical short-circuits",
			content:  "Severity: LOW\nSeverity: CRITICAL\nSeverity: HIGH\n",
			expected: "CRITICAL",
		},
		{
			name:     "moderate only",
			content:  "**Severity:** MODERATE\n",
			expected: "MODERATE",
		},
		{
			name:     "case insensitive label match",
			content:  "severity: HIGH\n",
			expected: "HIGH",
		},
		{
			name:     "mixed case label",
			content:  "Severity Level: LOW\n",
			expected: "LOW",
		},
		{
			name:     "empty string",
			content:  "",
			expected: "",
		},
		{
			name:     "whitespace only",
			content:  "   \n\n  \n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, extractMaxSeverity(tt.content))
		})
	}
}

func TestCountDeviations(t *testing.T) {
	t.Parallel()

	t.Run("no file returns zero", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		assert.Equal(t, 0, countDeviations(dir))
	})

	t.Run("empty file returns zero", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.DeviationLogFile), []byte(""), 0o644))

		assert.Equal(t, 0, countDeviations(dir))
	})

	t.Run("counts DEVIATE entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))

		content := "## Sprint 1\n**Decision**: DEVIATE\nSome details.\n\n" +
			"## Sprint 2\n**Decision**: CONTINUE\nAll good.\n\n" +
			"## Sprint 3\n**Decision**: DEVIATE\nMore changes.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.DeviationLogFile), []byte(content), 0o644))

		assert.Equal(t, 2, countDeviations(dir))
	})

	t.Run("no deviations", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))

		content := "## Sprint 1\n**Decision**: CONTINUE\nAll good.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.DeviationLogFile), []byte(content), 0o644))

		assert.Equal(t, 0, countDeviations(dir))
	})
}

func TestSprintProgressMentionsSprint(t *testing.T) {
	t.Parallel()

	t.Run("no file returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		assert.False(t, sprintProgressMentionsSprint(dir, 1))
	})

	t.Run("matching sprint returns true", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))

		content := "# Sprint 3: API — Progress\n\nSome work done.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte(content), 0o644))

		assert.True(t, sprintProgressMentionsSprint(dir, 3))
	})

	t.Run("non-matching sprint returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))

		content := "# Sprint 3: API — Progress\n\nSome work done.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte(content), 0o644))

		assert.False(t, sprintProgressMentionsSprint(dir, 1))
		assert.False(t, sprintProgressMentionsSprint(dir, 30))
	})

	t.Run("does not match partial number", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))

		// File contains "# Sprint 10:" — should NOT match sprint 1
		content := "# Sprint 10: Dashboard — Progress\n\nWorking.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte(content), 0o644))

		assert.False(t, sprintProgressMentionsSprint(dir, 1))
		assert.True(t, sprintProgressMentionsSprint(dir, 10))
	})

	t.Run("empty file returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fryDir := filepath.Join(dir, config.FryDir)
		require.NoError(t, os.MkdirAll(fryDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte(""), 0o644))

		assert.False(t, sprintProgressMentionsSprint(dir, 1))
	})
}

func TestCheckRequiredTools(t *testing.T) {
	t.Parallel()

	t.Run("empty list returns empty", func(t *testing.T) {
		t.Parallel()
		result := checkRequiredTools(nil)
		assert.Empty(t, result)
	})

	t.Run("common tools are available", func(t *testing.T) {
		t.Parallel()
		// bash and git should be available in any CI/dev environment
		result := checkRequiredTools([]string{"bash", "git"})
		require.Len(t, result, 2)
		assert.Equal(t, "bash", result[0].Name)
		assert.True(t, result[0].Available)
		assert.Equal(t, "git", result[1].Name)
		assert.True(t, result[1].Available)
	})

	t.Run("nonexistent tool is unavailable", func(t *testing.T) {
		t.Parallel()
		result := checkRequiredTools([]string{"fry_nonexistent_tool_xyz_12345"})
		require.Len(t, result, 1)
		assert.Equal(t, "fry_nonexistent_tool_xyz_12345", result[0].Name)
		assert.False(t, result[0].Available)
	})

	t.Run("mixed available and unavailable", func(t *testing.T) {
		t.Parallel()
		result := checkRequiredTools([]string{"git", "fry_nonexistent_tool_xyz_12345", "bash"})
		require.Len(t, result, 3)
		assert.True(t, result[0].Available)
		assert.False(t, result[1].Available)
		assert.True(t, result[2].Available)
	})
}
