package consciousness

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstanceID_NotEmpty(t *testing.T) {
	t.Parallel()

	id := InstanceID()
	assert.NotEmpty(t, id)
}

func TestInstanceID_Stable(t *testing.T) {
	t.Parallel()

	id1 := InstanceID()
	id2 := InstanceID()
	assert.Equal(t, id1, id2)
}

func TestInstanceID_Length(t *testing.T) {
	t.Parallel()

	id := InstanceID()
	if id == "unknown" {
		return // acceptable fallback
	}
	assert.Len(t, id, 16)
}

func TestInstanceID_IsHex(t *testing.T) {
	t.Parallel()

	id := InstanceID()
	if id == "unknown" {
		return
	}
	matched, err := regexp.MatchString(`^[0-9a-f]{16}$`, id)
	assert.NoError(t, err)
	assert.True(t, matched, "expected hex string, got %q", id)
}
