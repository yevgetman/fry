package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func writeConfigFile(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, config.ProjectConfigFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestLoad_NoFile(t *testing.T) {
	t.Parallel()

	settings, err := Load(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, settings.Engine)
}

func TestLoad_ValidFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfigFile(t, dir, `{"engine":"codex"}`)

	settings, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "codex", settings.Engine)
}

func TestLoad_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfigFile(t, dir, `not json`)

	_, err := Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestLoad_InvalidEngine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfigFile(t, dir, `{"engine":"bogus"}`)

	_, err := Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported engine")
}

func TestSave_AndGetEngine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, Save(dir, Settings{Engine: "claude"}))

	data, err := os.ReadFile(filepath.Join(dir, config.ProjectConfigFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"engine": "claude"`)

	engineName, err := GetEngine(dir)
	require.NoError(t, err)
	assert.Equal(t, "claude", engineName)
}

func TestSetEngine_PreservesFutureFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfigFile(t, dir, "{\n  \"extra\": true\n}\n")

	require.NoError(t, SetEngine(dir, "codex"))

	data, err := os.ReadFile(filepath.Join(dir, config.ProjectConfigFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"engine": "codex"`)
	assert.Contains(t, string(data), `"extra": true`)
}

func TestSetEngine_Invalid(t *testing.T) {
	t.Parallel()

	err := SetEngine(t.TempDir(), "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported engine")
}
