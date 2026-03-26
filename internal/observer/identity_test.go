package observer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadIdentity_ReturnsEmbedded(t *testing.T) {
	t.Parallel()

	content, err := ReadIdentity()
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.Contains(t, content, "Fry")
	assert.Contains(t, content, "Core Identity")
	assert.Contains(t, content, "Disposition")
}

func TestReadIdentity_NonEmpty(t *testing.T) {
	t.Parallel()

	content, err := ReadIdentity()
	require.NoError(t, err)
	assert.Greater(t, len(content), 100, "identity should be substantial")
}
