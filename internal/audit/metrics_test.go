package audit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/engine"
	tokenmetrics "github.com/yevgetman/fry/internal/metrics"
)

func TestAuditMetricsRecordAndSummary(t *testing.T) {
	t.Parallel()

	metrics := &AuditMetrics{
		RepeatedUnchangedFindings:    2,
		SuppressedUnchangedFindings:  1,
		ReopenedWithNewEvidence:      1,
		BehaviorUnchangedOutcomes:    3,
		BehaviorUnchangedEscalations: 1,
		LowYieldStrategyChanges:      1,
		LowYieldStopReason:           "low_yield cycle=2",
	}
	metrics.Record(CallMetric{SessionType: engine.SessionAudit, PromptBytes: 100, DurationMs: 10})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditFix, Cycle: 1, PromptBytes: 120, DurationMs: 20, WasNoOp: true, ValidationResult: fixValidationRejected})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditFix, Cycle: 1, PromptBytes: 140, DurationMs: 25, ValidationResult: fixValidationAccepted})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditVerify, Cycle: 1, PromptBytes: 80, DurationMs: 30, Resolutions: 2})
	metrics.RecordCycleSummary(1)

	assert.Equal(t, 4, metrics.TotalCalls())
	assert.Equal(t, int64(85), metrics.TotalDurationMs())
	assert.InDelta(t, 0.5, metrics.NoOpRate(), 0.001)
	assert.Equal(t, 1, metrics.TotalAcceptedFixCalls())
	assert.Equal(t, 1, metrics.TotalRejectedFixCalls())
	assert.Equal(t, 2, metrics.Snapshot().RepeatedUnchanged)
	assert.Equal(t, 1, metrics.Snapshot().SuppressedUnchanged)
	assert.Equal(t, 1, metrics.Snapshot().ReopenedWithNewEvidence)
	assert.Equal(t, 3, metrics.Snapshot().BehaviorUnchanged)
	assert.Equal(t, 1, metrics.Snapshot().BehaviorEscalations)
	assert.Equal(t, 1, metrics.Snapshot().LowYieldStrategyChanges)
	assert.Equal(t, "low_yield cycle=2", metrics.Snapshot().LowYieldStopReason)
	assert.InDelta(t, 1.0, metrics.Snapshot().LastCycleFixYield, 0.001)
	assert.InDelta(t, 2.0, metrics.Snapshot().LastCycleVerifyYield, 0.001)
	assert.InDelta(t, 2.0, metrics.VerifyYield(), 0.001)
	assert.Equal(t, 110, metrics.AvgPromptBytes())
}

func TestAuditMetricsMarshalJSON(t *testing.T) {
	t.Parallel()

	metrics := &AuditMetrics{
		Calls: []CallMetric{
			{
				SessionType:          engine.SessionAuditFix,
				PromptBytes:          120,
				DurationMs:           20,
				WasNoOp:              true,
				SessionRefreshReason: "call budget reached (4)",
			},
		},
		OuterCycles:                  2,
		ContentComplexity:            ComplexityModerate,
		FinalFindingCount:            1,
		BehaviorUnchangedOutcomes:    1,
		BehaviorUnchangedEscalations: 1,
		SessionRefreshes:             1,
		LowYieldStrategyChanges:      1,
		LowYieldStopReason:           "low_yield cycle=2",
		SessionRefreshReasons: map[string]int{
			"call budget reached (4)": 1,
		},
	}

	data, err := json.Marshal(metrics)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	summary, ok := payload["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), summary["total_calls"])
	assert.Equal(t, float64(1), summary["no_op_fix_calls"])
	assert.Equal(t, float64(0), summary["accepted_fix_calls"])
	assert.Equal(t, float64(0), summary["rejected_fix_calls"])
	assert.Equal(t, float64(0), summary["repeated_unchanged_findings"])
	assert.Equal(t, float64(0), summary["suppressed_unchanged_findings"])
	assert.Equal(t, float64(0), summary["reopened_with_new_evidence"])
	assert.Equal(t, float64(1), summary["behavior_unchanged_outcomes"])
	assert.Equal(t, float64(1), summary["behavior_unchanged_escalations"])
	assert.Equal(t, float64(1), summary["session_refreshes"])
	assert.Equal(t, float64(1), summary["low_yield_strategy_changes"])
	assert.Equal(t, "low_yield cycle=2", summary["low_yield_stop_reason"])
	assert.Equal(t, float64(1), payload["session_refreshes"])
	assert.Equal(t, float64(1), payload["session_refresh_reasons"].(map[string]any)["call budget reached (4)"])
	assert.Equal(t, float64(1), payload["low_yield_strategy_changes"])
	assert.Equal(t, "low_yield cycle=2", payload["low_yield_stop_reason"])
}

func TestCallMetricTokenParsing(t *testing.T) {
	t.Parallel()

	claude := tokenmetrics.ParseTokens("claude", "input_tokens: 10\noutput_tokens: 4\n")
	codex := tokenmetrics.ParseTokens("codex", "\"prompt_tokens\": 7\n\"completion_tokens\": 3\n")
	ollama := tokenmetrics.ParseTokens("ollama", "tokens unavailable")

	assert.Equal(t, 14, claude.Total)
	assert.Equal(t, 10, codex.Total)
	assert.Equal(t, 0, ollama.Total)
}

func TestAuditMetricsRecordTracksSessionRefreshes(t *testing.T) {
	t.Parallel()

	metrics := &AuditMetrics{}

	metrics.Record(CallMetric{SessionType: engine.SessionAudit, SessionRefreshReason: "call budget reached (3)"})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditFix, SessionRefreshReason: "call budget reached (3)"})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditVerify})

	snapshot := metrics.Snapshot()

	assert.Equal(t, 2, metrics.SessionRefreshes)
	assert.Equal(t, 2, metrics.SessionRefreshReasons["call budget reached (3)"])
	assert.Equal(t, 2, snapshot.SessionRefreshes)
}

func TestAuditMetricsCycleSummariesAndTrailingYield(t *testing.T) {
	t.Parallel()

	metrics := &AuditMetrics{}

	metrics.Record(CallMetric{SessionType: engine.SessionAudit, Cycle: 1, DurationMs: 10})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditFix, Cycle: 1, DurationMs: 20, WasNoOp: true})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditVerify, Cycle: 1, DurationMs: 30, Resolutions: 1})
	metrics.RecordCycleSummary(1)

	metrics.Record(CallMetric{SessionType: engine.SessionAudit, Cycle: 2, DurationMs: 15})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditFix, Cycle: 2, DurationMs: 25})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditVerify, Cycle: 2, DurationMs: 35, Resolutions: 0})
	metrics.RecordCycleSummary(2)

	require.Len(t, metrics.CycleSummaries, 2)
	assert.Equal(t, 1, metrics.CycleSummaries[0].NoOpFixCalls)
	assert.InDelta(t, 0.0, metrics.CycleSummaries[1].FixYield, 0.001)
	assert.InDelta(t, 0.0, metrics.CycleSummaries[1].VerifyYield, 0.001)

	snapshot := metrics.Snapshot()
	assert.InDelta(t, 0.0, snapshot.LastCycleFixYield, 0.001)
	assert.InDelta(t, 0.0, snapshot.LastCycleVerifyYield, 0.001)
	assert.InDelta(t, 0.0, snapshot.LastCycleNoOpRate, 0.001)
	assert.InDelta(t, 0.5, snapshot.TrailingFixYield, 0.001)
	assert.InDelta(t, 0.5, snapshot.TrailingVerifyYield, 0.001)
	assert.InDelta(t, 0.5, snapshot.TrailingNoOpRate, 0.001)
	assert.InDelta(t, 135.0, snapshot.TrailingMsPerResolution, 0.001)
}
