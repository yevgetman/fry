package consciousness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCoreIdentity(t *testing.T) {
	t.Parallel()

	content, err := LoadCoreIdentity()
	require.NoError(t, err)
	assert.NotEmpty(t, content)

	// Should contain core identity content (works for both JSON and .md paths)
	assert.Contains(t, content, "Fry")

	// Should contain disposition content
	assert.Contains(t, content, "Disposition")
}

func TestLoadDisposition(t *testing.T) {
	t.Parallel()

	content, err := LoadDisposition()
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.Contains(t, content, "Disposition")
}

func TestLoadFullIdentity(t *testing.T) {
	t.Parallel()

	content, err := LoadFullIdentity()
	require.NoError(t, err)
	assert.NotEmpty(t, content)

	// Full identity includes both core and disposition
	assert.Contains(t, content, "Fry")
	assert.Contains(t, content, "Disposition")
}

func TestLoadCoreIdentity_ContainsBothLayers(t *testing.T) {
	t.Parallel()

	core, err := LoadCoreIdentity()
	require.NoError(t, err)

	disp, err := LoadDisposition()
	require.NoError(t, err)

	// Core identity should be longer than disposition alone (it includes both)
	assert.Greater(t, len(core), len(disp))
}
