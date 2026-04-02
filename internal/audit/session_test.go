package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/engine"
)

func TestExtractSessionID(t *testing.T) {
	t.Parallel()

	claudeOutput := `{"type":"result","result":"OK","session_id":"34429d2b-d11c-4b8b-b84a-896dd59bcc80"}`
	codexOutput := `{"type":"thread.started","thread_id":"019d5066-f512-7bc1-aba8-e45cf2fb9a84"}`

	assert.Equal(t, "34429d2b-d11c-4b8b-b84a-896dd59bcc80", extractSessionID("claude", claudeOutput))
	assert.Equal(t, "019d5066-f512-7bc1-aba8-e45cf2fb9a84", extractSessionID("codex", codexOutput))
	assert.Equal(t, "", extractSessionID("ollama", codexOutput))
}

func TestSessionContinuityConfigureCaptureAndClear(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	session := newFixSessionContinuity(projectDir, 2, 3, "codex")
	require.NotNil(t, session)

	var runOpts engine.RunOpts
	session.Configure(&runOpts)
	assert.True(t, runOpts.StructuredOutput)
	assert.Equal(t, "", runOpts.SessionID)

	session.Capture(`{"type":"thread.started","thread_id":"019d5066-f512-7bc1-aba8-e45cf2fb9a84"}`)

	var resumed engine.RunOpts
	session.Configure(&resumed)
	assert.True(t, resumed.StructuredOutput)
	assert.Equal(t, "019d5066-f512-7bc1-aba8-e45cf2fb9a84", resumed.SessionID)
	assert.FileExists(t, fixSessionPath(projectDir, 2, 3))

	session.Clear()
	assert.NoFileExists(t, fixSessionPath(projectDir, 2, 3))
}

func TestNewSessionContinuityUnsupportedEngine(t *testing.T) {
	t.Parallel()

	assert.Nil(t, newAuditSessionContinuity(t.TempDir(), 1, "ollama"))
}

func TestCleanupAuditSessions(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, persistSessionID(auditSessionPath(projectDir, 1), "audit"))
	require.NoError(t, persistSessionID(fixSessionPath(projectDir, 1, 2), "fix"))
	require.NoError(t, persistSessionID(auditSessionPath(projectDir, 2), "other"))

	require.NoError(t, cleanupAuditSessions(projectDir, 1))

	assert.NoFileExists(t, auditSessionPath(projectDir, 1))
	assert.NoFileExists(t, fixSessionPath(projectDir, 1, 2))
	assert.FileExists(t, auditSessionPath(projectDir, 2))
}

func TestAgentTranscriptNormalizesStructuredOutput(t *testing.T) {
	t.Parallel()

	claudeTranscript := agentTranscript(`{"type":"result","result":"Recovered audit summary","session_id":"34429d2b-d11c-4b8b-b84a-896dd59bcc80"}`, "")
	codexTranscript := agentTranscript("{\"type\":\"thread.started\",\"thread_id\":\"019d5066-f512-7bc1-aba8-e45cf2fb9a84\"}\n{\"type\":\"item.completed\",\"item\":{\"id\":\"item_0\",\"type\":\"agent_message\",\"text\":\"Recovered verification summary\"}}\n", "")

	assert.Contains(t, claudeTranscript, "Recovered audit summary")
	assert.Contains(t, codexTranscript, "Recovered verification summary")
	assert.Contains(t, claudeTranscript, "assistant")
	assert.Contains(t, codexTranscript, "assistant")
}
