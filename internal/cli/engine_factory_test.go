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

func TestBuildOneShot_NoFallbackForOllama(t *testing.T) {
	t.Parallel()

	p := &enginePlanner{activeName: "ollama"}
	_, _, err := p.BuildOneShot()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no fallback engine available")
}

func TestBuildOneShot_ResolvesFallback(t *testing.T) {
	t.Parallel()

	p := &enginePlanner{activeName: "claude"}
	eng, name, err := p.BuildOneShot()
	require.NoError(t, err)
	assert.Equal(t, "codex", name)
	assert.NotNil(t, eng)
}

func TestBuildOneShot_DoesNotPinEngine(t *testing.T) {
	t.Parallel()

	p := &enginePlanner{activeName: "claude"}
	_, _, err := p.BuildOneShot()
	require.NoError(t, err)

	// Active engine should still be claude — not pinned to the fallback
	assert.Equal(t, "claude", p.Current())
	assert.False(t, p.pinned)
}

func TestBuildOneShot_RespectsExplicitFallback(t *testing.T) {
	t.Parallel()

	p := &enginePlanner{activeName: "claude", fallbackName: "codex"}
	eng, name, err := p.BuildOneShot()
	require.NoError(t, err)
	assert.Equal(t, "codex", name)
	assert.NotNil(t, eng)
}
