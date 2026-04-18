package mission

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/state"
)

func TestScaffoldPrompt(t *testing.T) {
	// Create a temp prompt file
	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	require.NoError(t, os.WriteFile(promptFile, []byte("# Test mission"), 0o644))

	baseDir := t.TempDir()
	opts := NewOptions{
		Name:       "testmission",
		BaseDir:    baseDir,
		PromptFile: promptFile,
		Effort:     "fast",
		Interval:   5 * time.Minute,
		Duration:   30 * time.Minute,
		Overtime:   0,
	}

	missionDir, err := Scaffold(opts)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(baseDir, "testmission"), missionDir)

	// Check all expected files exist
	for _, f := range []string{
		"state.json", "notes.md", "runner.sh", "scheduler.plist",
		"wake_log.jsonl", "supervisor_log.jsonl", "prompt.md",
	} {
		_, err := os.Stat(filepath.Join(missionDir, f))
		assert.NoError(t, err, "missing file: %s", f)
	}

	// Check subdirs
	for _, d := range []string{"artifacts", "lock", "logs"} {
		info, err := os.Stat(filepath.Join(missionDir, d))
		require.NoError(t, err, "missing dir: %s", d)
		assert.True(t, info.IsDir())
	}

	// Validate state.json
	m, err := state.Load(missionDir)
	require.NoError(t, err)
	assert.Equal(t, "testmission", m.MissionID)
	assert.Equal(t, state.StatusActive, m.Status)
	assert.Equal(t, "fast", m.Effort)
	assert.Equal(t, 300, m.IntervalSeconds)
	assert.Equal(t, 0, m.CurrentWake)
}

func TestScaffoldDuplicateNameFails(t *testing.T) {
	promptFile := filepath.Join(t.TempDir(), "p.md")
	require.NoError(t, os.WriteFile(promptFile, []byte("x"), 0o644))
	baseDir := t.TempDir()

	opts := NewOptions{
		Name: "dup", BaseDir: baseDir, PromptFile: promptFile,
		Effort: "standard", Interval: 10 * time.Minute, Duration: time.Hour,
	}
	_, err := Scaffold(opts)
	require.NoError(t, err)

	// Second scaffold with same name should fail
	_, err = Scaffold(opts)
	assert.Error(t, err)
}

func TestScaffoldValidation(t *testing.T) {
	base := t.TempDir()

	cases := []struct {
		name string
		opts NewOptions
	}{
		{"no name", NewOptions{BaseDir: base, PromptFile: "x", Effort: "fast", Interval: time.Minute, Duration: time.Hour}},
		{"no input", NewOptions{Name: "m", BaseDir: base, Effort: "fast", Interval: time.Minute, Duration: time.Hour}},
		{"bad effort", NewOptions{Name: "m", BaseDir: base, PromptFile: "x", Effort: "turbo", Interval: time.Minute, Duration: time.Hour}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Scaffold(tc.opts)
			assert.Error(t, err)
		})
	}
}

func TestInputMode(t *testing.T) {
	assert.Equal(t, "prompt", NewOptions{PromptFile: "p"}.InputMode())
	assert.Equal(t, "plan", NewOptions{PlanFile: "l"}.InputMode())
	assert.Equal(t, "prompt+plan", NewOptions{PromptFile: "p", PlanFile: "l"}.InputMode())
	assert.Equal(t, "spec-dir", NewOptions{SpecDir: "d"}.InputMode())
	assert.Equal(t, "", NewOptions{}.InputMode())
}
