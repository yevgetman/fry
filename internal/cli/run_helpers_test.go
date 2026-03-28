package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/color"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/sprint"
)

func TestFormatAffectedSprints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []int
		want  string
	}{
		{"nil slice", nil, "unknown"},
		{"empty slice", []int{}, "unknown"},
		{"single element", []int{5}, "5"},
		{"multiple elements", []int{1, 2, 3}, "1, 2, 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatAffectedSprints(tt.input))
		})
	}
}

func TestColorizeStatus(t *testing.T) {
	// Not parallel: mutates global color state via color.SetEnabled.
	color.SetEnabled(true)
	t.Cleanup(func() { color.SetEnabled(false) })

	tests := []struct {
		name         string
		input        string
		wantContains string
	}{
		{"PASS prefix", "PASS", "\033[32m"},
		{"PASS with detail", "PASS (3/3)", "\033[32m"},
		{"FAIL prefix", "FAIL", "\033[31m"},
		{"FAIL with detail", "FAIL (sanity)", "\033[31m"},
		{"SKIPPED", sprint.StatusSkipped, "\033[33m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := colorizeStatus(tt.input)
			assert.Contains(t, got, tt.wantContains)
			assert.Contains(t, got, tt.input)
		})
	}

	t.Run("unknown status returned unchanged", func(t *testing.T) {
		assert.Equal(t, "UNKNOWN", colorizeStatus("UNKNOWN"))
	})
}

func TestIsPassStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"PASS", "PASS", true},
		{"PASS with detail", "PASS (3/3)", true},
		{"FAIL", "FAIL", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isPassStatus(tt.input))
		})
	}
}

func TestStartSprintCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start, end int
		want       int
	}{
		{"full range", 1, 5, 5},
		{"single sprint", 3, 3, 1},
		{"partial range", 2, 4, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, startSprintCount(tt.start, tt.end))
		})
	}
}

func TestInitializeSprintResults(t *testing.T) {
	t.Parallel()

	ep := &epic.Epic{
		Sprints: []epic.Sprint{
			{Number: 1, Name: "Sprint 1"},
			{Number: 2, Name: "Sprint 2"},
			{Number: 3, Name: "Sprint 3"},
		},
		TotalSprints: 3,
	}

	tests := []struct {
		name       string
		start, end int
		wantLen    int
		wantFirst  int
		wantLast   int
	}{
		{"all sprints", 1, 3, 3, 1, 3},
		{"single sprint", 2, 2, 1, 2, 2},
		{"partial range", 2, 3, 2, 2, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := initializeSprintResults(ep, tt.start, tt.end)
			require.Len(t, results, tt.wantLen)
			assert.Equal(t, tt.wantFirst, results[0].Number)
			assert.Equal(t, tt.wantLast, results[len(results)-1].Number)
			for _, r := range results {
				assert.Equal(t, sprint.StatusSkipped, r.Status)
			}
		})
	}

	t.Run("sprint names populated from epic", func(t *testing.T) {
		t.Parallel()
		results := initializeSprintResults(ep, 1, 3)
		assert.Equal(t, "Sprint 1", results[0].Name)
		assert.Equal(t, "Sprint 2", results[1].Name)
		assert.Equal(t, "Sprint 3", results[2].Name)
	})
}

func TestResolvePrepareEngine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		prepareFlag string
		runFlag     string
		want        string
	}{
		{"prepare flag set", "claude", "codex", "claude"},
		{"prepare empty, run set", "", "codex", "codex"},
		{"whitespace prepare, run set", "  ", "codex", "codex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolvePrepareEngine(tt.prepareFlag, tt.runFlag))
		})
	}

}

func TestResolvePrepareEngine_EnvFallback(t *testing.T) {
	t.Setenv("FRY_ENGINE", "test-engine")
	got := resolvePrepareEngine("", "")
	assert.Equal(t, "test-engine", got)
}

func TestTruncateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		maxBytes int
		want     string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"long string truncated", "hello world", 5, "hello... [truncated]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, truncateString(tt.input, tt.maxBytes))
		})
	}
}

func TestReadOptionalFile(t *testing.T) {
	t.Parallel()

	t.Run("existing file returns content", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		p := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(p, []byte("hello\n"), 0o644))

		got, err := readOptionalFile(p)
		require.NoError(t, err)
		assert.Equal(t, "hello\n", got)
	})

	t.Run("non-existent file returns empty string", func(t *testing.T) {
		t.Parallel()
		got, err := readOptionalFile("/nonexistent/path/file.txt")
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})
}

func TestUserPromptSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		prompt        string
		flagValue     string
		fileFlagValue string
		want          string
	}{
		{"empty prompt", "", "", "", ""},
		{"whitespace-only prompt", "   ", "", "", ""},
		{"prompt with file flag", "do stuff", "", "plan.txt", "--user-prompt-file plan.txt"},
		{"prompt with flag value", "do stuff", "inline", "", "--user-prompt flag"},
		{"prompt only, no flags", "do stuff", "", "", config.UserPromptFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, userPromptSource(tt.prompt, tt.flagValue, tt.fileFlagValue))
		})
	}
}

func TestTelemetryBoolPtr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ptr := telemetryBoolPtr(tt.input)
			require.NotNil(t, ptr)
			assert.Equal(t, tt.input, *ptr)
		})
	}
}
