package lock

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
)

func TestAcquireRelease(t *testing.T) {
	projectDir := t.TempDir()

	require.NoError(t, Acquire(projectDir))
	lockPath := filepath.Join(projectDir, config.LockFile)
	data, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(os.Getpid()), string(data))

	require.NoError(t, Release(projectDir))
	_, err = os.Stat(lockPath)
	assert.True(t, os.IsNotExist(err))
}

func TestAcquireStale(t *testing.T) {
	projectDir := t.TempDir()
	lockPath := filepath.Join(projectDir, config.LockFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(lockPath), 0o755))
	require.NoError(t, os.WriteFile(lockPath, []byte("999999"), 0o644))

	require.NoError(t, Acquire(projectDir))
}

func TestAcquireDryRunSkips(t *testing.T) {
	projectDir := t.TempDir()

	require.NoError(t, AcquireIfNotDryRun(projectDir, true))
	_, err := os.Stat(filepath.Join(projectDir, config.LockFile))
	assert.True(t, os.IsNotExist(err))
}
