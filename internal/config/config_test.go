package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yevgetman/fry/internal/config"
)

func TestDefaults(t *testing.T) {
	assert.Equal(t, "github.com/yevgetman/fry", "github.com/yevgetman/fry")
	assert.Equal(t, "codex", config.DefaultEngine)
	assert.Equal(t, "0.1.0", config.Version)
}
