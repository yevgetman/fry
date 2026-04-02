package audit

import (
	"encoding/json"

	"github.com/yevgetman/fry/internal/engine"
	tokenmetrics "github.com/yevgetman/fry/internal/metrics"
)

// CallMetric captures the observable outcome of one audit, fix, or verify call.
type CallMetric struct {
	SessionType engine.SessionType      `json:"session_type"`
	Cycle       int                     `json:"cycle"`
	Iteration   int                     `json:"iteration"`
	PromptBytes int                     `json:"prompt_bytes"`
	OutputBytes int                     `json:"output_bytes"`
	DurationMs  int64                   `json:"duration_ms"`
	Model       string                  `json:"model,omitempty"`
	WasNoOp     bool                    `json:"was_no_op,omitempty"`
	Resolutions int                     `json:"resolutions,omitempty"`
	Tokens      tokenmetrics.TokenUsage `json:"tokens"`
}

// AuditMetricsSnapshot is the small, monitor-friendly subset surfaced in build status.
type AuditMetricsSnapshot struct {
	TotalCalls        int     `json:"total_calls"`
	DurationMs        int64   `json:"duration_ms"`
	NoOpFixCalls      int     `json:"no_op_fix_calls"`
	NoOpRate          float64 `json:"no_op_rate"`
	VerifyCalls       int     `json:"verify_calls"`
	VerifyResolutions int     `json:"verify_resolutions"`
	VerifyYield       float64 `json:"verify_yield"`
}

// AuditMetrics accumulates per-call audit telemetry for one RunAuditLoop invocation.
type AuditMetrics struct {
	Calls               []CallMetric   `json:"calls"`
	OuterCycles         int            `json:"outer_cycles"`
	ContentComplexity   ComplexityTier `json:"content_complexity,omitempty"`
	ConvergedAtCycle    int            `json:"converged_at_cycle,omitempty"`
	FinalFindingCount   int            `json:"final_finding_count"`
	EscapedToBuildAudit int            `json:"escaped_to_build_audit,omitempty"`
}

func (m *AuditMetrics) Record(cm CallMetric) {
	if m == nil {
		return
	}
	m.Calls = append(m.Calls, cm)
}

func (m *AuditMetrics) TotalCalls() int {
	if m == nil {
		return 0
	}
	return len(m.Calls)
}

func (m *AuditMetrics) TotalDurationMs() int64 {
	if m == nil {
		return 0
	}
	var total int64
	for _, call := range m.Calls {
		total += call.DurationMs
	}
	return total
}

func (m *AuditMetrics) AvgPromptBytes() int {
	if m == nil || len(m.Calls) == 0 {
		return 0
	}
	total := 0
	for _, call := range m.Calls {
		total += call.PromptBytes
	}
	return total / len(m.Calls)
}

func (m *AuditMetrics) TotalNoOpFixCalls() int {
	if m == nil {
		return 0
	}
	total := 0
	for _, call := range m.Calls {
		if call.SessionType == engine.SessionAuditFix && call.WasNoOp {
			total++
		}
	}
	return total
}

func (m *AuditMetrics) TotalFixCalls() int {
	if m == nil {
		return 0
	}
	total := 0
	for _, call := range m.Calls {
		if call.SessionType == engine.SessionAuditFix {
			total++
		}
	}
	return total
}

func (m *AuditMetrics) TotalVerifyCalls() int {
	if m == nil {
		return 0
	}
	total := 0
	for _, call := range m.Calls {
		if call.SessionType == engine.SessionAuditVerify {
			total++
		}
	}
	return total
}

func (m *AuditMetrics) TotalVerifyResolutions() int {
	if m == nil {
		return 0
	}
	total := 0
	for _, call := range m.Calls {
		if call.SessionType == engine.SessionAuditVerify {
			total += call.Resolutions
		}
	}
	return total
}

func (m *AuditMetrics) NoOpRate() float64 {
	fixCalls := m.TotalFixCalls()
	if fixCalls == 0 {
		return 0
	}
	return float64(m.TotalNoOpFixCalls()) / float64(fixCalls)
}

func (m *AuditMetrics) VerifyYield() float64 {
	verifyCalls := m.TotalVerifyCalls()
	if verifyCalls == 0 {
		return 0
	}
	return float64(m.TotalVerifyResolutions()) / float64(verifyCalls)
}

func (m *AuditMetrics) Snapshot() AuditMetricsSnapshot {
	if m == nil {
		return AuditMetricsSnapshot{}
	}
	return AuditMetricsSnapshot{
		TotalCalls:        m.TotalCalls(),
		DurationMs:        m.TotalDurationMs(),
		NoOpFixCalls:      m.TotalNoOpFixCalls(),
		NoOpRate:          m.NoOpRate(),
		VerifyCalls:       m.TotalVerifyCalls(),
		VerifyResolutions: m.TotalVerifyResolutions(),
		VerifyYield:       m.VerifyYield(),
	}
}

func (m *AuditMetrics) MarshalJSON() ([]byte, error) {
	type auditMetricsJSON struct {
		Calls               []CallMetric         `json:"calls"`
		OuterCycles         int                  `json:"outer_cycles"`
		ContentComplexity   ComplexityTier       `json:"content_complexity,omitempty"`
		ConvergedAtCycle    int                  `json:"converged_at_cycle,omitempty"`
		FinalFindingCount   int                  `json:"final_finding_count"`
		EscapedToBuildAudit int                  `json:"escaped_to_build_audit,omitempty"`
		Summary             AuditMetricsSnapshot `json:"summary"`
	}

	payload := auditMetricsJSON{
		Summary: m.Snapshot(),
	}
	if m != nil {
		payload.Calls = m.Calls
		payload.OuterCycles = m.OuterCycles
		payload.ContentComplexity = m.ContentComplexity
		payload.ConvergedAtCycle = m.ConvergedAtCycle
		payload.FinalFindingCount = m.FinalFindingCount
		payload.EscapedToBuildAudit = m.EscapedToBuildAudit
	}
	return json.Marshal(payload)
}
