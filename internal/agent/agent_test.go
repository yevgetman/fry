package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ArtifactSchema tests ---

func TestArtifactSchema_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()
	schema := ArtifactSchema()
	assert.NotEmpty(t, schema)
	for _, a := range schema {
		assert.NotEmpty(t, a.Path, "artifact path should not be empty")
		assert.NotEmpty(t, a.Format, "artifact format should not be empty")
		assert.NotEmpty(t, a.Description, "artifact description should not be empty")
		assert.NotEmpty(t, a.Lifecycle, "artifact lifecycle should not be empty")
	}
}

func TestArtifactSchema_ContainsKeyArtifacts(t *testing.T) {
	t.Parallel()
	schema := ArtifactSchema()
	paths := make(map[string]bool)
	for _, a := range schema {
		paths[a.Path] = true
	}
	assert.True(t, paths[".fry/observer/events.jsonl"], "should include events.jsonl")
	assert.True(t, paths[".fry/sprint-progress.txt"], "should include sprint-progress.txt")
	assert.True(t, paths[".fry/epic-progress.txt"], "should include epic-progress.txt")
	assert.True(t, paths[".fry/build-logs"], "should include build-logs")
}

// --- ReadBuildState tests ---

func TestReadBuildState_NoFryDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	state, err := ReadBuildState(dir)
	require.NoError(t, err)
	assert.False(t, state.Active)
	assert.Equal(t, "idle", state.Status)
}

func TestReadBuildState_WithEvents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create .fry/observer/ directory
	obsDir := filepath.Join(dir, ".fry", "observer")
	require.NoError(t, os.MkdirAll(obsDir, 0o755))

	// Create a simple epic
	fryDir := filepath.Join(dir, ".fry")
	epicContent := "# Test Epic\n\n## Sprint 1: Setup\n\n@prompt\nDo stuff\n@end\n\n## Sprint 2: Build\n\n@prompt\nMore stuff\n@end\n"
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte(epicContent), 0o644))

	// Write events
	events := []string{
		`{"ts":"2026-03-27T10:00:00Z","type":"build_start","data":{"effort":"standard","epic":"Test Epic","total_sprints":"2"}}`,
		`{"ts":"2026-03-27T10:05:00Z","type":"sprint_start","sprint":1,"data":{"name":"Setup"}}`,
		`{"ts":"2026-03-27T10:10:00Z","type":"sprint_complete","sprint":1,"data":{"status":"PASS","duration":"5m"}}`,
	}
	eventsContent := strings.Join(events, "\n") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(obsDir, "events.jsonl"), []byte(eventsContent), 0o644))

	state, err := ReadBuildState(dir)
	require.NoError(t, err)
	assert.Equal(t, "Test Epic", state.Epic)
	assert.Equal(t, 2, state.TotalSprints)
	assert.Equal(t, 1, state.CurrentSprint)
	assert.NotNil(t, state.LastEvent)
	assert.Equal(t, "sprint_complete", state.LastEvent.Type)
}

func TestReadBuildState_CompletedBuild(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	obsDir := filepath.Join(dir, ".fry", "observer")
	require.NoError(t, os.MkdirAll(obsDir, 0o755))
	fryDir := filepath.Join(dir, ".fry")

	epicContent := "# Done Epic\n\n## Sprint 1: Only\n\n@prompt\nDo it\n@end\n"
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte(epicContent), 0o644))

	events := `{"ts":"2026-03-27T10:00:00Z","type":"build_start","data":{"effort":"fast","epic":"Done Epic","total_sprints":"1"}}
{"ts":"2026-03-27T10:05:00Z","type":"build_end","data":{"outcome":"success"}}
`
	require.NoError(t, os.WriteFile(filepath.Join(obsDir, "events.jsonl"), []byte(events), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "build-exit-reason.txt"), []byte("success"), 0o644))

	state, err := ReadBuildState(dir)
	require.NoError(t, err)
	assert.False(t, state.Active)
	assert.Equal(t, "completed", state.Status)
}

func TestReadBuildState_EngineFailoverEventStaysRunning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	obsDir := filepath.Join(dir, ".fry", "observer")
	require.NoError(t, os.MkdirAll(obsDir, 0o755))
	fryDir := filepath.Join(dir, ".fry")

	epicContent := "# Test Epic\n\n## Sprint 1: Only\n\n@prompt\nDo it\n@end\n"
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte(epicContent), 0o644))

	events := `{"ts":"2026-03-27T10:00:00Z","type":"build_start","data":{"effort":"standard","epic":"Test Epic","total_sprints":"1"}}
{"ts":"2026-03-27T10:02:00Z","type":"engine_failover","data":{"from":"claude","to":"codex"}}
`
	require.NoError(t, os.WriteFile(filepath.Join(obsDir, "events.jsonl"), []byte(events), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, ".fry.lock"), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644))

	state, err := ReadBuildState(dir)
	require.NoError(t, err)
	assert.Equal(t, "running", state.Status)
	require.NotNil(t, state.LastEvent)
	assert.Equal(t, "engine_failover", state.LastEvent.Type)
}

// --- ReadProgress tests ---

func TestReadProgress_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content, err := ReadProgress(dir, "sprint")
	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestReadProgress_SprintProgress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry", "sprint-progress.txt"), []byte("progress data"), 0o644))

	content, err := ReadProgress(dir, "sprint")
	require.NoError(t, err)
	assert.Equal(t, "progress data", content)
}

func TestReadProgress_InvalidScope(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := ReadProgress(dir, "invalid")
	assert.Error(t, err)
}

// --- ReadLatestLog tests ---

func TestReadLatestLog_NoLogsDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content, err := ReadLatestLog(dir, "latest", 50)
	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestReadLatestLog_WithLogs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logsDir := filepath.Join(dir, ".fry", "build-logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint1_20260327_100000.log"), []byte("sprint log content\nline 2\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint1_iter1_20260327_100000.log"), []byte("iter log"), 0o644))

	content, err := ReadLatestLog(dir, "sprint", 50)
	require.NoError(t, err)
	assert.Contains(t, content, "sprint log content")
}

func TestReadLatestLog_LastNLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logsDir := filepath.Join(dir, ".fry", "build-logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line " + strings.Repeat("x", i)
	}
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sprint1_20260327_100000.log"), []byte(strings.Join(lines, "\n")), 0o644))

	content, err := ReadLatestLog(dir, "sprint", 5)
	require.NoError(t, err)
	resultLines := strings.Split(content, "\n")
	assert.LessOrEqual(t, len(resultLines), 5)
}

// --- TailEvents tests ---

func TestTailEvents_ReadsExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	obsDir := filepath.Join(dir, ".fry", "observer")
	require.NoError(t, os.MkdirAll(obsDir, 0o755))

	events := `{"ts":"2026-03-27T10:00:00Z","type":"build_start","data":{"effort":"standard"}}
{"ts":"2026-03-27T10:05:00Z","type":"sprint_start","sprint":1,"data":{"name":"Setup"}}
`
	require.NoError(t, os.WriteFile(filepath.Join(obsDir, "events.jsonl"), []byte(events), 0o644))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := TailEvents(ctx, dir)
	require.NoError(t, err)

	var received []BuildEvent
	timeout := time.After(3 * time.Second)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				goto done
			}
			received = append(received, evt)
			if len(received) >= 2 {
				cancel()
				goto done
			}
		case <-timeout:
			cancel()
			goto done
		}
	}
done:
	require.Len(t, received, 2)
	assert.Equal(t, "build_start", received[0].Type)
	assert.Equal(t, "sprint_start", received[1].Type)
	assert.Equal(t, 1, received[1].Sprint)
}

// --- ReadAllEvents tests ---

func TestReadAllEvents_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	events, err := ReadAllEvents(dir)
	require.NoError(t, err)
	assert.Nil(t, events)
}

func TestReadAllEvents_ParsesCorrectly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	obsDir := filepath.Join(dir, ".fry", "observer")
	require.NoError(t, os.MkdirAll(obsDir, 0o755))

	events := `{"ts":"2026-03-27T10:00:00Z","type":"build_start","data":{"effort":"high"}}
{"ts":"2026-03-27T10:01:00Z","type":"sprint_start","sprint":1}
`
	require.NoError(t, os.WriteFile(filepath.Join(obsDir, "events.jsonl"), []byte(events), 0o644))

	result, err := ReadAllEvents(dir)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "build_start", result[0].Type)
	assert.Equal(t, "high", result[0].Data["effort"])
}

// --- BuildAgentSystemPrompt tests ---

func TestBuildAgentSystemPrompt_NonEmpty(t *testing.T) {
	t.Parallel()
	prompt := BuildAgentSystemPrompt()
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "# Identity")
	assert.Contains(t, prompt, "# Role")
	assert.Contains(t, prompt, "# Build Lifecycle")
	assert.Contains(t, prompt, "# Fry Artifacts")
	assert.Contains(t, prompt, "# Event Types")
	assert.Contains(t, prompt, "# Effort Levels")
	assert.Contains(t, prompt, "# Conversation Patterns")
}

func TestBuildAgentSystemPrompt_ContainsArtifactPaths(t *testing.T) {
	t.Parallel()
	prompt := BuildAgentSystemPrompt()
	assert.Contains(t, prompt, ".fry/observer/events.jsonl")
	assert.Contains(t, prompt, ".fry/sprint-progress.txt")
	assert.Contains(t, prompt, ".fry/epic-progress.txt")
}

// --- BuildState JSON serialization ---

func TestBuildState_JSONRoundtrip(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	state := &BuildState{
		Active:            true,
		ProjectDir:        "/tmp/test",
		Epic:              "Test",
		Effort:            "standard",
		Engine:            "claude",
		TotalSprints:      4,
		CurrentSprint:     2,
		CurrentSprintName: "API",
		Status:            "running",
		StartedAt:         &now,
		PID:               12345,
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var decoded BuildState
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, state.Epic, decoded.Epic)
	assert.Equal(t, state.Status, decoded.Status)
	assert.Equal(t, state.PID, decoded.PID)
}
