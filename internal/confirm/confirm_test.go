package confirm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestWritePrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	p := &Prompt{
		Type:    PromptTriageConfirm,
		Message: "Accept this classification?",
		Data:    map[string]any{"complexity": "SIMPLE", "effort": "low"},
		Options: []string{"accept", "adjust", "reject"},
	}
	require.NoError(t, WritePrompt(dir, p))

	path := filepath.Join(dir, config.ConfirmPromptFile)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var read Prompt
	require.NoError(t, json.Unmarshal(data, &read))
	assert.Equal(t, PromptTriageConfirm, read.Type)
	assert.Equal(t, "Accept this classification?", read.Message)
	assert.Equal(t, []string{"accept", "adjust", "reject"}, read.Options)

	// No temp file left behind
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestWritePrompt_CreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	p := &Prompt{Type: PromptTriageConfirm, Message: "test"}
	require.NoError(t, WritePrompt(dir, p))

	info, err := os.Stat(filepath.Join(dir, ".fry"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestWaitForResponse_Immediate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Pre-write the response file before calling WaitForResponse
	resp := Response{Action: "accept"}
	data, _ := json.Marshal(resp)
	respPath := filepath.Join(dir, config.ConfirmResponseFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(respPath), 0o755))
	require.NoError(t, os.WriteFile(respPath, data, 0o644))

	// Also write prompt so cleanup has something to remove
	promptPath := filepath.Join(dir, config.ConfirmPromptFile)
	require.NoError(t, os.WriteFile(promptPath, []byte("{}"), 0o644))

	got, err := WaitForResponse(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, "accept", got.Action)

	// Both files should be cleaned up
	_, err = os.Stat(respPath)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(promptPath)
	assert.True(t, os.IsNotExist(err))
}

func TestWaitForResponse_WithAdjustments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	resp := Response{
		Action:      "adjust",
		Adjustments: map[string]any{"effort": "high"},
	}
	data, _ := json.Marshal(resp)
	respPath := filepath.Join(dir, config.ConfirmResponseFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(respPath), 0o755))
	require.NoError(t, os.WriteFile(respPath, data, 0o644))

	got, err := WaitForResponse(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, "adjust", got.Action)
	assert.Equal(t, "high", got.Adjustments["effort"])
}

func TestWaitForResponse_Timeout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry"), 0o755))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := WaitForResponse(ctx, dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestCleanup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	promptPath := filepath.Join(dir, config.ConfirmPromptFile)
	respPath := filepath.Join(dir, config.ConfirmResponseFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(respPath, []byte("{}"), 0o644))

	Cleanup(dir)

	_, err := os.Stat(promptPath)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(respPath)
	assert.True(t, os.IsNotExist(err))
}

func TestCleanup_NoFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Should not panic when files don't exist
	Cleanup(dir)
}
