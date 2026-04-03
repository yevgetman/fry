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
		RepeatedUnchangedFindings:   2,
		SuppressedUnchangedFindings: 1,
		ReopenedWithNewEvidence:     1,
	}
	metrics.Record(CallMetric{SessionType: engine.SessionAudit, PromptBytes: 100, DurationMs: 10})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditFix, PromptBytes: 120, DurationMs: 20, WasNoOp: true, ValidationResult: fixValidationRejected})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditFix, PromptBytes: 140, DurationMs: 25, ValidationResult: fixValidationAccepted})
	metrics.Record(CallMetric{SessionType: engine.SessionAuditVerify, PromptBytes: 80, DurationMs: 30, Resolutions: 2})

	assert.Equal(t, 4, metrics.TotalCalls())
	assert.Equal(t, int64(85), metrics.TotalDurationMs())
	assert.InDelta(t, 0.5, metrics.NoOpRate(), 0.001)
	assert.Equal(t, 1, metrics.TotalAcceptedFixCalls())
	assert.Equal(t, 1, metrics.TotalRejectedFixCalls())
	assert.Equal(t, 2, metrics.Snapshot().RepeatedUnchanged)
	assert.Equal(t, 1, metrics.Snapshot().SuppressedUnchanged)
	assert.Equal(t, 1, metrics.Snapshot().ReopenedWithNewEvidence)
	assert.InDelta(t, 2.0, metrics.VerifyYield(), 0.001)
	assert.Equal(t, 110, metrics.AvgPromptBytes())
}

func TestAuditMetricsMarshalJSON(t *testing.T) {
	t.Parallel()

	metrics := &AuditMetrics{
		Calls: []CallMetric{
			{SessionType: engine.SessionAuditFix, PromptBytes: 120, DurationMs: 20, WasNoOp: true},
		},
		OuterCycles:       2,
		ContentComplexity: ComplexityModerate,
		FinalFindingCount: 1,
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
