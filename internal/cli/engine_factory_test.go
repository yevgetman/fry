package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveFallbackEngine(t *testing.T) {
	t.Parallel()

	t.Run("implicit claude fallback is codex", func(t *testing.T) {
		t.Parallel()
		name, explicit, err := resolveFallbackEngine("claude", "")
		require.NoError(t, err)
		assert.Equal(t, "codex", name)
		assert.False(t, explicit)
	})

	t.Run("implicit codex fallback is claude", func(t *testing.T) {
		t.Parallel()
		name, explicit, err := resolveFallbackEngine("codex", "")
		require.NoError(t, err)
		assert.Equal(t, "claude", name)
		assert.False(t, explicit)
	})

	t.Run("ollama has no implicit fallback", func(t *testing.T) {
		t.Parallel()
		name, explicit, err := resolveFallbackEngine("ollama", "")
		require.NoError(t, err)
		assert.Empty(t, name)
		assert.False(t, explicit)
	})

	t.Run("explicit override is normalized", func(t *testing.T) {
		t.Parallel()
		name, explicit, err := resolveFallbackEngine("claude", " CODEX ")
		require.NoError(t, err)
		assert.Equal(t, "codex", name)
		assert.True(t, explicit)
	})

	t.Run("explicit override must differ from primary", func(t *testing.T) {
		t.Parallel()
		_, _, err := resolveFallbackEngine("claude", "claude")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must differ from primary")
	})
}
