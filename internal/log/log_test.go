package log

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetLogFile(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	l := NewLogger(&buf)

	l.Log("test message %d", 42)

	output := buf.String()
	assert.Contains(t, output, "test message 42")
	assert.Contains(t, output, "[") // timestamp bracket
}

func TestLog_NilLogFile(t *testing.T) {
	t.Parallel()

	l := NewLogger(nil)

	require.NotPanics(t, func() {
		l.Log("safe message")
	})
}

func TestAgentBanner_DefaultModel(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	l := NewLogger(&buf)

	l.AgentBanner(2, 5, "Auth", 1, 3, "claude", "")

	output := buf.String()
	assert.Contains(t, output, "AGENT")
	assert.Contains(t, output, "Sprint 2/5")
	assert.Contains(t, output, `"Auth"`)
	assert.Contains(t, output, "iter 1/3")
	assert.Contains(t, output, "engine=claude")
	assert.Contains(t, output, "model=default")
}

func TestAgentBanner_CustomModel(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	l := NewLogger(&buf)

	l.AgentBanner(1, 1, "Solo", 1, 1, "codex", "gpt-5.4")

	output := buf.String()
	assert.Contains(t, output, "model=gpt-5.4")
	assert.NotContains(t, output, "model=default")
}
