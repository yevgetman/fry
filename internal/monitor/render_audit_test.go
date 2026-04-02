package monitor

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yevgetman/fry/internal/agent"
)

func TestRenderDashboard_ShowsAuditComplexityAndMetrics(t *testing.T) {
	t.Parallel()

	now := time.Now()
	snap := Snapshot{
		Timestamp: now,
		BuildStatus: &agent.BuildStatus{
			Version: 1,
			Build: agent.BuildInfo{
				Epic:         "Audit Improvements",
				Engine:       "codex",
				Mode:         "writing",
				Effort:       "high",
				TotalSprints: 3,
			},
			Sprints: []agent.SprintStatus{{
				Number: 1,
				Name:   "Pricing",
				Status: "running",
				Audit: &agent.AuditStatus{
					Active:       true,
					Stage:        "verifying",
					CurrentCycle: 2,
					MaxCycles:    8,
					CurrentFix:   1,
					MaxFixes:     7,
					Complexity:   "high",
					Metrics: &agent.AuditMetricsSnapshot{
						TotalCalls:   5,
						NoOpRate:     0.25,
						VerifyYield:  1.5,
						DurationMs:   1200,
						VerifyCalls:  2,
						NoOpFixCalls: 1,
					},
				},
			}},
		},
	}

	var buf bytes.Buffer
	RenderDashboard(&buf, snap, false, false)
	output := buf.String()

	assert.Contains(t, output, "complexity high")
	assert.Contains(t, output, "5 calls")
	assert.Contains(t, output, "25% no-op")
	assert.Contains(t, output, "1.5 verify yield")
}
