package observer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestEnsureIdentity_CreatesFromSeed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	content, err := EnsureIdentity(dir)
	require.NoError(t, err)
	assert.Contains(t, content, "# Observer Identity")
	assert.Contains(t, content, "metacognitive")

	// Verify file exists on disk
	data, err := os.ReadFile(filepath.Join(dir, config.ObserverIdentityFile))
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestEnsureIdentity_PreservesExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	identityPath := filepath.Join(dir, config.ObserverIdentityFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(identityPath), 0o755))

	customContent := "# My Custom Identity\nI have evolved.\n"
	require.NoError(t, os.WriteFile(identityPath, []byte(customContent), 0o644))

	content, err := EnsureIdentity(dir)
	require.NoError(t, err)
	assert.Equal(t, customContent, content)
}

func TestReadIdentity_Missing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	content, err := ReadIdentity(dir)
	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestWriteIdentity_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	original := "# Updated Identity\nI observe patterns now.\n"

	err := WriteIdentity(dir, original)
	require.NoError(t, err)

	content, err := ReadIdentity(dir)
	require.NoError(t, err)
	assert.Equal(t, original, content)
}

func TestIdentitySeed_NotEmpty(t *testing.T) {
	t.Parallel()

	seed, err := identitySeed()
	require.NoError(t, err)
	assert.NotEmpty(t, seed)
	assert.Contains(t, seed, "Observer Identity")
}
