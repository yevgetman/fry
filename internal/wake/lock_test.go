package wake

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquire_Succeeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	lk, err := Acquire(dir)
	require.NoError(t, err)
	require.NotNil(t, lk)

	// Lock directory must exist on disk.
	info, err := os.Stat(filepath.Join(dir, "lock"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestAcquire_SecondCallReturnsErrLocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	lk1, err := Acquire(dir)
	require.NoError(t, err)
	defer func() { _ = lk1.Release() }()

	lk2, err := Acquire(dir)
	assert.Nil(t, lk2)
	assert.True(t, errors.Is(err, ErrLocked), "expected ErrLocked, got %v", err)
}

func TestRelease_RemovesLockDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	lk, err := Acquire(dir)
	require.NoError(t, err)
	require.NoError(t, lk.Release())

	// After release, lock dir must be gone.
	_, err = os.Stat(filepath.Join(dir, "lock"))
	assert.True(t, os.IsNotExist(err), "lock dir should not exist after Release; err=%v", err)
}

func TestAcquire_AfterReleaseSucceeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	lk1, err := Acquire(dir)
	require.NoError(t, err)
	require.NoError(t, lk1.Release())

	// A second Acquire after release must succeed (lock is gone).
	lk2, err := Acquire(dir)
	require.NoError(t, err)
	require.NotNil(t, lk2)
	_ = lk2.Release()
}

func TestAcquire_OnMissingParentFails(t *testing.T) {
	t.Parallel()
	// Pass a path whose parent does not exist — os.Mkdir should fail,
	// and the returned error is NOT ErrLocked (which means the lock isn't held;
	// it means the lock cannot even be attempted).
	_, err := Acquire("/nonexistent/deeply/nested/mission")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrLocked),
		"Acquire on missing parent must not report ErrLocked — there's no lock to speak of")
}
