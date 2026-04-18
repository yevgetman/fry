//go:build darwin

package scheduler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/state"
)

func TestPlistLabel(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "com.fry.demo", plistLabel("demo"))
	assert.Equal(t, "com.fry.my-mission-123", plistLabel("my-mission-123"))
}

func TestPlistDst_IncludesLaunchAgentsPath(t *testing.T) {
	t.Parallel()

	got, err := plistDst("demo")
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(got, "/Library/LaunchAgents/com.fry.demo.plist"),
		"plistDst must point to user's LaunchAgents dir, got %q", got)
}

func TestPlistDst_RespectsHomeEnv(t *testing.T) {
	// Not parallel: mutates HOME.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := plistDst("m")
	require.NoError(t, err)

	expected := filepath.Join(tmp, "Library", "LaunchAgents", "com.fry.m.plist")
	assert.Equal(t, expected, got)
}

func TestNew_ReturnsDarwinScheduler(t *testing.T) {
	t.Parallel()
	s := New()
	_, ok := s.(darwinScheduler)
	assert.True(t, ok, "New() on darwin must return darwinScheduler, got %T", s)
}

// copyFileSched is an internal helper; confirm it copies bytes faithfully.
func TestCopyFileSched_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "src.plist")
	dst := filepath.Join(dir, "dst.plist")
	payload := []byte("<?xml version=\"1.0\"?>\n<plist />\n")
	require.NoError(t, os.WriteFile(src, payload, 0o644))

	require.NoError(t, copyFileSched(src, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestCopyFileSched_MissingSrcFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.plist")

	err := copyFileSched(filepath.Join(dir, "no-such-source"), dst)
	require.Error(t, err)
}

// Status must not error even when the plist was never loaded — it just reports Running=false.
func TestStatus_NotRunningForUnknownMission(t *testing.T) {
	t.Parallel()
	s := darwinScheduler{}
	m := &state.Mission{MissionID: "nonexistent-for-test-" + t.Name()}

	got, err := s.Status(m)
	require.NoError(t, err)
	assert.False(t, got.Running, "Status for a mission that was never started must return Running=false")
}
