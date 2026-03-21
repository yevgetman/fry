package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaEngineName(t *testing.T) {
	t.Parallel()

	eng := &OllamaEngine{}
	assert.Equal(t, "ollama", eng.Name())
}

func TestOllamaArgs_DefaultModel(t *testing.T) {
	t.Parallel()

	args := ollamaArgs(RunOpts{})
	assert.Equal(t, []string{"run", "llama3"}, args)
}

func TestOllamaArgs_ExplicitModel(t *testing.T) {
	t.Parallel()

	args := ollamaArgs(RunOpts{Model: "mistral"})
	assert.Equal(t, []string{"run", "mistral"}, args)
}

func TestOllamaArgs_WithExtraFlags(t *testing.T) {
	t.Parallel()

	args := ollamaArgs(RunOpts{Model: "codellama", ExtraFlags: []string{"--verbose"}})
	assert.Equal(t, []string{"run", "codellama", "--verbose"}, args)
}

func TestOllamaRunBinaryNotFound(t *testing.T) {
	// t.Parallel() intentionally omitted: t.Setenv panics after t.Parallel() (Go 1.17+).
	t.Setenv("PATH", t.TempDir())
	eng := &OllamaEngine{}
	_, exitCode, err := eng.Run(context.Background(), "hello", RunOpts{})
	require.Error(t, err)
	assert.Equal(t, -1, exitCode)
}

func TestOllamaRunContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eng := &OllamaEngine{}
	_, _, err := eng.Run(ctx, "hello", RunOpts{})
	require.Error(t, err)
}

func TestNewEngine_Ollama(t *testing.T) {
	t.Parallel()

	eng, err := NewEngine("ollama")
	require.NoError(t, err)
	_, ok := eng.(*OllamaEngine)
	assert.True(t, ok)
}

func TestResolveEngine_Ollama(t *testing.T) {
	t.Parallel()

	name, err := ResolveEngine("ollama", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "ollama", name)
}

func TestValidateModel_Ollama(t *testing.T) {
	t.Parallel()

	err := ValidateModel("ollama", "my-custom-model:7b")
	assert.NoError(t, err)
}

func TestModelsForEngine_Ollama(t *testing.T) {
	t.Parallel()

	models, err := ModelsForEngine("ollama")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
}

func TestTierModel_Ollama(t *testing.T) {
	t.Parallel()

	result := TierModel("ollama", TierFrontier)
	assert.Equal(t, "llama3", result)
}
