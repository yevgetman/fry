package monitor

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yevgetman/fry/internal/agent"
)

func TestRenderEvent_Basic(t *testing.T) {
	t.Parallel()

	evt := EnrichedEvent{
		BuildEvent: agent.BuildEvent{
			Type:      "sprint_start",
			Timestamp: time.Date(2026, 3, 30, 10, 0, 15, 0, time.UTC),
			Sprint:    1,
			Data:      map[string]string{"name": "Setup"},
		},
		ElapsedBuild: 15 * time.Second,
		SprintOf:     "1/3",
	}

	var buf bytes.Buffer
	RenderEvent(&buf, evt, false)
	output := buf.String()

	assert.Contains(t, output, "[10:00:15]")
	assert.Contains(t, output, "+15s")
	assert.Contains(t, output, "sprint_start")
	assert.Contains(t, output, "1/3")
	assert.Contains(t, output, "name=Setup")
}

func TestRenderEvent_WithPhaseChange(t *testing.T) {
	t.Parallel()

	evt := EnrichedEvent{
		BuildEvent: agent.BuildEvent{
			Type:      "build_start",
			Timestamp: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
		},
		PhaseChange: "prepare -> sprint",
	}

	var buf bytes.Buffer
	RenderEvent(&buf, evt, false)
	output := buf.String()

	assert.Contains(t, output, "[prepare -> sprint]")
}

func TestRenderEvent_Synthetic(t *testing.T) {
	t.Parallel()

	evt := EnrichedEvent{
		BuildEvent: agent.BuildEvent{
			Type:      "monitor:process_exited",
			Timestamp: time.Now(),
		},
		Synthetic: true,
	}

	var buf bytes.Buffer
	RenderEvent(&buf, evt, false)
	output := buf.String()

	assert.Contains(t, output, "*monitor:process_exited")
}

func TestRenderDashboard_Basic(t *testing.T) {
	t.Parallel()

	now := time.Now()
	started := now.Add(-5 * time.Minute)
	finished := now.Add(-2 * time.Minute)

	snap := Snapshot{
		Timestamp:   now,
		ProjectDir:  "/tmp/test",
		BuildActive: true,
		PID:         12345,
		Phase:       "sprint",
		BuildStatus: &agent.BuildStatus{
			Version: 1,
			Build: agent.BuildInfo{
				Epic:         "MyFeature",
				Engine:       "claude",
				Mode:         "software",
				Effort:       "high",
				TotalSprints: 3,
			},
			Sprints: []agent.SprintStatus{
				{Number: 1, Name: "Setup", Status: "PASS", StartedAt: &started, FinishedAt: &finished, DurationSec: 180},
				{Number: 2, Name: "API", Status: "running", StartedAt: &now},
			},
		},
		Events: []EnrichedEvent{
			{
				BuildEvent: agent.BuildEvent{
					Type:      "sprint_start",
					Timestamp: now,
					Sprint:    2,
					Data:      map[string]string{"name": "API"},
				},
				SprintOf: "2/3",
			},
		},
	}

	var buf bytes.Buffer
	RenderDashboard(&buf, snap, false, false)
	output := buf.String()

	assert.Contains(t, output, "Fry Monitor")
	assert.Contains(t, output, "MyFeature")
	assert.Contains(t, output, "claude")
	assert.Contains(t, output, "high")
	assert.Contains(t, output, "Setup")
	assert.Contains(t, output, "PASS")
	assert.Contains(t, output, "API")
	assert.Contains(t, output, "running")
	assert.Contains(t, output, "Sprint 3/3")
	assert.Contains(t, output, "pending")
}

func TestRenderDashboard_ResumeBuildBackfillsCompletedSprints(t *testing.T) {
	t.Parallel()

	now := time.Now()
	finished := now.Add(-2 * time.Minute)

	snap := Snapshot{
		Timestamp:   now,
		ProjectDir:  "/tmp/test",
		BuildActive: true,
		PID:         12345,
		Phase:       "sprint",
		BuildStatus: &agent.BuildStatus{
			Version: 1,
			Build: agent.BuildInfo{
				Epic:         "MyFeature",
				Engine:       "claude",
				Mode:         "software",
				Effort:       "high",
				TotalSprints: 5,
			},
			Sprints: []agent.SprintStatus{
				{Number: 5, Name: "Booking", Status: "PASS", FinishedAt: &finished, DurationSec: 1289},
			},
		},
		EpicProgress: strings.Join([]string{
			"## Sprint 1: Setup — PASS",
			"## Sprint 2: Auth — PASS",
			"## Sprint 3: Calendar — PASS",
			"## Sprint 4: Availability — PASS",
		}, "\n"),
	}

	var buf bytes.Buffer
	RenderDashboard(&buf, snap, false, false)
	output := buf.String()

	assert.Contains(t, output, "Sprint 1/5: Setup")
	assert.Contains(t, output, "Sprint 2/5: Auth")
	assert.Contains(t, output, "Sprint 3/5: Calendar")
	assert.Contains(t, output, "Sprint 4/5: Availability")
	assert.Contains(t, output, "Sprint 5/5: Booking")
	assert.NotContains(t, output, "Sprint 2/5 ....................................... pending")
	assert.NotContains(t, output, "Sprint 5/5 ...................................... pending")
}

func TestRenderDashboard_ResumeBuildBackfillsFailedSprints(t *testing.T) {
	t.Parallel()

	now := time.Now()

	snap := Snapshot{
		Timestamp:   now,
		ProjectDir:  "/tmp/test",
		BuildActive: true,
		PID:         12345,
		Phase:       "sprint",
		BuildStatus: &agent.BuildStatus{
			Version: 1,
			Build: agent.BuildInfo{
				Epic:         "MyFeature",
				Engine:       "claude",
				Mode:         "software",
				Effort:       "high",
				TotalSprints: 4,
			},
			Sprints: []agent.SprintStatus{
				{Number: 3, Name: "Repair", Status: "running", StartedAt: &now},
			},
		},
		EpicProgress: strings.Join([]string{
			"## Sprint 1: Setup — PASS",
			"## Sprint 2: Auth — FAIL (audit: HIGH)",
		}, "\n"),
	}

	var buf bytes.Buffer
	RenderDashboard(&buf, snap, false, false)
	output := buf.String()

	assert.Contains(t, output, "Sprint 1/4: Setup")
	assert.Contains(t, output, "PASS")
	assert.Contains(t, output, "Sprint 2/4: Auth")
	assert.Contains(t, output, "FAIL (audit: HIGH)")
	assert.Contains(t, output, "Sprint 3/4: Repair")
	assert.Contains(t, output, "running")
	assert.Contains(t, output, "Sprint 4/4")
	assert.Contains(t, output, "pending")
}

func TestRenderDashboard_BuildEnded(t *testing.T) {
	t.Parallel()

	snap := Snapshot{
		Timestamp:  time.Now(),
		BuildEnded: true,
	}

	var buf bytes.Buffer
	RenderDashboard(&buf, snap, false, false)
	output := buf.String()

	assert.Contains(t, output, "Build complete")
}

func TestRenderDashboard_BuildEndedWithError(t *testing.T) {
	t.Parallel()

	snap := Snapshot{
		Timestamp:  time.Now(),
		BuildEnded: true,
		ExitReason: "audit failed",
	}

	var buf bytes.Buffer
	RenderDashboard(&buf, snap, false, false)
	output := buf.String()

	assert.Contains(t, output, "Build ended: audit failed")
}

func TestRenderLogTail_NoLog(t *testing.T) {
	t.Parallel()

	snap := Snapshot{}
	var buf bytes.Buffer
	RenderLogTail(&buf, snap)
	assert.Contains(t, buf.String(), "No active build log")
}

func TestRenderLogTail_WithContent(t *testing.T) {
	t.Parallel()

	snap := Snapshot{
		ActiveLogPath: "/tmp/test/.fry/build-logs/sprint1_iter1.log",
		ActiveLogTail: "working on feature\ncreated handler.go",
	}

	var buf bytes.Buffer
	RenderLogTail(&buf, snap)
	output := buf.String()

	assert.Contains(t, output, "sprint1_iter1.log")
	assert.Contains(t, output, "working on feature")
}

func TestRenderWaiting(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	RenderWaiting(&buf, "/tmp/myproject")
	assert.Contains(t, buf.String(), "Waiting for build")
	assert.Contains(t, buf.String(), "/tmp/myproject")
}

func TestRenderBuildEnded_Success(t *testing.T) {
	t.Parallel()

	snap := Snapshot{}
	var buf bytes.Buffer
	RenderBuildEnded(&buf, snap, false)
	assert.Contains(t, buf.String(), "Build complete")
}

func TestRenderBuildEnded_WithReason(t *testing.T) {
	t.Parallel()

	snap := Snapshot{ExitReason: "sprint 3 failed"}
	var buf bytes.Buffer
	RenderBuildEnded(&buf, snap, false)
	assert.Contains(t, buf.String(), "sprint 3 failed")
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d        time.Duration
		expected string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{5*time.Minute + 10*time.Second, "5m10s"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{-1 * time.Second, "0s"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatDuration(tt.d))
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 6))
	assert.Equal(t, "he", truncate("hello", 2))
}
