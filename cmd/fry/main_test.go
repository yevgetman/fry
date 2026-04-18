package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execRoot drives rootCmd() with the given args, capturing stdout.
// It returns the captured output and any error from Execute.
func execRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := rootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// writePrompt writes a trivial prompt.md into a temp dir and returns its path.
func writePrompt(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prompt.md")
	require.NoError(t, os.WriteFile(path, []byte("# Test mission\nDo work.\n"), 0o644))
	return path
}

func TestCmd_Version(t *testing.T) {
	t.Parallel()
	// The version subcommand prints via fmt.Println to os.Stdout, bypassing
	// cmd.SetOut, so we can't easily capture stdout. Just verify it wires
	// up and runs without error.
	root := rootCmd()
	root.SetArgs([]string{"version"})
	require.NoError(t, root.Execute())
}

func TestCmd_New_ScaffoldsMission(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	prompt := writePrompt(t)

	// Note: `fry new` prints to os.Stdout (via fmt.Printf), not cmd.OutOrStdout.
	// We can't easily capture that without refactoring. Instead, assert on filesystem.
	_, err := execRoot(t, "new", "demo",
		"--prompt", prompt,
		"--base-dir", base,
		"--interval", "30s",
		"--duration", "1m",
		"--effort", "fast",
	)
	require.NoError(t, err)

	// Mission dir must exist with expected files.
	missionDir := filepath.Join(base, "demo")
	for _, f := range []string{"state.json", "notes.md", "prompt.md", "runner.sh", "scheduler.plist"} {
		_, err := os.Stat(filepath.Join(missionDir, f))
		assert.NoError(t, err, "missing file after `fry new`: %s", f)
	}
}

func TestCmd_New_RejectsDuplicateName(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	prompt := writePrompt(t)

	_, err := execRoot(t, "new", "dup",
		"--prompt", prompt, "--base-dir", base,
		"--interval", "30s", "--duration", "1m", "--effort", "fast")
	require.NoError(t, err)

	// Second attempt must fail.
	_, err = execRoot(t, "new", "dup",
		"--prompt", prompt, "--base-dir", base,
		"--interval", "30s", "--duration", "1m", "--effort", "fast")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCmd_New_RejectsMissingInput(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	_, err := execRoot(t, "new", "noinput",
		"--base-dir", base,
		"--interval", "30s", "--duration", "1m", "--effort", "fast")
	require.Error(t, err, "`fry new` with no --prompt/--plan/--spec-dir must fail")
}

func TestCmd_New_RejectsBadEffort(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	prompt := writePrompt(t)

	_, err := execRoot(t, "new", "badeff",
		"--prompt", prompt, "--base-dir", base,
		"--interval", "30s", "--duration", "1m", "--effort", "turbo")
	require.Error(t, err)
}

func TestCmd_New_RejectsBadInterval(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	prompt := writePrompt(t)

	_, err := execRoot(t, "new", "badint",
		"--prompt", prompt, "--base-dir", base,
		"--interval", "not-a-duration", "--duration", "1m", "--effort", "fast")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --interval")
}

func TestCmd_List_EmptyBaseDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	// list prints to os.Stdout via fmt.Println; we can at least confirm no error.
	_, err := execRoot(t, "list", "--base-dir", base)
	require.NoError(t, err)
}

func TestCmd_List_NonexistentBaseDir(t *testing.T) {
	t.Parallel()
	_, err := execRoot(t, "list", "--base-dir", filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err, "missing base-dir must print friendly message, not error")
}

func TestCmd_Status_MissingMissionErrors(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	_, err := execRoot(t, "status", "nope", "--base-dir", base)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot load mission")
}

func TestCmd_Status_ExistingMission(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	prompt := writePrompt(t)
	_, err := execRoot(t, "new", "s1",
		"--prompt", prompt, "--base-dir", base,
		"--interval", "30s", "--duration", "1m", "--effort", "fast")
	require.NoError(t, err)

	_, err = execRoot(t, "status", "s1", "--base-dir", base)
	require.NoError(t, err)
}

func TestCmd_Logs_MissingMissionErrors(t *testing.T) {
	t.Parallel()
	_, err := execRoot(t, "logs", "nope", "--base-dir", t.TempDir())
	require.Error(t, err)
}

func TestCmd_Logs_NoEntriesOk(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	prompt := writePrompt(t)
	_, err := execRoot(t, "new", "l1",
		"--prompt", prompt, "--base-dir", base,
		"--interval", "30s", "--duration", "1m", "--effort", "fast")
	require.NoError(t, err)

	_, err = execRoot(t, "logs", "l1", "--base-dir", base)
	require.NoError(t, err)
}

func TestCmd_RootHelp(t *testing.T) {
	t.Parallel()
	out, err := execRoot(t, "--help")
	require.NoError(t, err)
	// Cobra help must list every subcommand we registered.
	for _, sub := range []string{"new", "list", "status", "start", "stop", "wake", "logs", "chat", "version"} {
		assert.Contains(t, out, sub, "--help output must mention subcommand %q", sub)
	}
}

func TestParseDurationHours(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in        string
		wantHours float64
		wantErr   bool
	}{
		{"12h", 12.0, false},
		{"1h30m", 1.5, false},
		{"0", 0, false},
		{"0h", 0, false},
		{"", 0, false},
		{"not-a-duration", 0, true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.in), func(t *testing.T) {
			got, err := parseDurationHours(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tc.wantHours, got.Hours(), 1e-9)
		})
	}
}

func TestTailJSON_TailsLastN(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.jsonl")
	content := `{"wake_number":1}
{"wake_number":2}
{"wake_number":3}
{"wake_number":4}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	got := tailJSON(path, 2)
	require.Len(t, got, 2)
	// Should be entries 3 and 4.
	assert.EqualValues(t, 3, got[0]["wake_number"])
	assert.EqualValues(t, 4, got[1]["wake_number"])
}

func TestTailJSON_MissingFileReturnsNil(t *testing.T) {
	t.Parallel()
	got := tailJSON(filepath.Join(t.TempDir(), "does-not-exist.jsonl"), 5)
	assert.Nil(t, got)
}
