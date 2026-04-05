package audit

import (
	"encoding/json"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	tokenmetrics "github.com/yevgetman/fry/internal/metrics"
)

// CallMetric captures the observable outcome of one audit, fix, or verify call.
type CallMetric struct {
	SessionType          engine.SessionType      `json:"session_type"`
	Cycle                int                     `json:"cycle"`
	Iteration            int                     `json:"iteration"`
	ClusterID            int                     `json:"cluster_id,omitempty"`
	SessionRefreshReason string                  `json:"session_refresh_reason,omitempty"`
	IssueIDs             []int                   `json:"issue_ids,omitempty"`
	PromptBytes          int                     `json:"prompt_bytes"`
	OutputBytes          int                     `json:"output_bytes"`
	DurationMs           int64                   `json:"duration_ms"`
	Model                string                  `json:"model,omitempty"`
	WasNoOp              bool                    `json:"was_no_op,omitempty"`
	DeclaredTargetFiles  []string                `json:"declared_target_files,omitempty"`
	ChangedFiles         []string                `json:"changed_files,omitempty"`
	DiffClassification   string                  `json:"diff_classification,omitempty"`
	ValidationResult     string                  `json:"validation_result,omitempty"`
	AlreadyFixedClaim    bool                    `json:"already_fixed_claim,omitempty"`
	Resolutions          int                     `json:"resolutions,omitempty"`
	Tokens               tokenmetrics.TokenUsage `json:"tokens"`
}

type StrategyShift struct {
	Cycle     int    `json:"cycle"`
	Iteration int    `json:"iteration,omitempty"`
	Trigger   string `json:"trigger"`
	Action    string `json:"action"`
	Detail    string `json:"detail,omitempty"`
}

// CycleProductivity summarizes audit productivity for one completed outer cycle.
type CycleProductivity struct {
	Cycle              int     `json:"cycle"`
	TotalCalls         int     `json:"total_calls"`
	DurationMs         int64   `json:"duration_ms"`
	FixCalls           int     `json:"fix_calls"`
	NoOpFixCalls       int     `json:"no_op_fix_calls"`
	VerifyCalls        int     `json:"verify_calls"`
	VerifyResolutions  int     `json:"verify_resolutions"`
	InputTokens        int     `json:"input_tokens"`
	OutputTokens       int     `json:"output_tokens"`
	TokenTotal         int     `json:"token_total"`
	CacheReadInput     int     `json:"cache_read_input"`
	CacheCreationInput int     `json:"cache_creation_input"`
	FixYield           float64 `json:"fix_yield"`
	VerifyYield        float64 `json:"verify_yield"`
	NoOpRate           float64 `json:"no_op_rate"`
	MsPerResolution    float64 `json:"ms_per_resolution"`
}

// AuditMetricsSnapshot is the small, monitor-friendly subset surfaced in build status.
type AuditMetricsSnapshot struct {
	TotalCalls               int     `json:"total_calls"`
	DurationMs               int64   `json:"duration_ms"`
	NoOpFixCalls             int     `json:"no_op_fix_calls"`
	AcceptedFixCalls         int     `json:"accepted_fix_calls"`
	RejectedFixCalls         int     `json:"rejected_fix_calls"`
	RepeatedUnchanged        int     `json:"repeated_unchanged_findings"`
	SuppressedUnchanged      int     `json:"suppressed_unchanged_findings"`
	ReopenedWithNewEvidence  int     `json:"reopened_with_new_evidence"`
	BehaviorUnchanged        int     `json:"behavior_unchanged_outcomes"`
	BehaviorEscalations      int     `json:"behavior_unchanged_escalations"`
	SessionRefreshes         int     `json:"session_refreshes"`
	NoOpRate                 float64 `json:"no_op_rate"`
	VerifyCalls              int     `json:"verify_calls"`
	VerifyResolutions        int     `json:"verify_resolutions"`
	VerifyYield              float64 `json:"verify_yield"`
	LastCycleFixYield        float64 `json:"last_cycle_fix_yield"`
	LastCycleVerifyYield     float64 `json:"last_cycle_verify_yield"`
	LastCycleNoOpRate        float64 `json:"last_cycle_no_op_rate"`
	LastCycleMsPerResolution float64 `json:"last_cycle_ms_per_resolution"`
	TrailingFixYield         float64 `json:"trailing_fix_yield"`
	TrailingVerifyYield      float64 `json:"trailing_verify_yield"`
	TrailingNoOpRate         float64 `json:"trailing_no_op_rate"`
	TrailingMsPerResolution  float64 `json:"trailing_ms_per_resolution"`
	StrategyShiftCount       int     `json:"strategy_shift_count"`
	LastStrategyShift        string  `json:"last_strategy_shift,omitempty"`
	LowYieldStrategyChanges  int     `json:"low_yield_strategy_changes"`
	LowYieldStopReason       string  `json:"low_yield_stop_reason,omitempty"`
}

// AuditMetrics accumulates per-call audit telemetry for one RunAuditLoop invocation.
type AuditMetrics struct {
	Calls                        []CallMetric        `json:"calls"`
	StrategyShifts               []StrategyShift     `json:"strategy_shifts,omitempty"`
	CycleSummaries               []CycleProductivity `json:"cycle_summaries,omitempty"`
	OuterCycles                  int                 `json:"outer_cycles"`
	FixStrategy                  string              `json:"fix_strategy,omitempty"`
	ContentComplexity            ComplexityTier      `json:"content_complexity,omitempty"`
	ConvergedAtCycle             int                 `json:"converged_at_cycle,omitempty"`
	FinalFindingCount            int                 `json:"final_finding_count"`
	EscapedToBuildAudit          int                 `json:"escaped_to_build_audit,omitempty"`
	RepeatedUnchangedFindings    int                 `json:"repeated_unchanged_findings,omitempty"`
	SuppressedUnchangedFindings  int                 `json:"suppressed_unchanged_findings,omitempty"`
	ReopenedWithNewEvidence      int                 `json:"reopened_with_new_evidence,omitempty"`
	BehaviorUnchangedOutcomes    int                 `json:"behavior_unchanged_outcomes,omitempty"`
	BehaviorUnchangedEscalations int                 `json:"behavior_unchanged_escalations,omitempty"`
	SessionRefreshes             int                 `json:"session_refreshes,omitempty"`
	SessionRefreshReasons        map[string]int      `json:"session_refresh_reasons,omitempty"`
	LowYieldStrategyChanges      int                 `json:"low_yield_strategy_changes,omitempty"`
	LowYieldStrategyReasons      map[string]int      `json:"low_yield_strategy_reasons,omitempty"`
	LowYieldStopReason           string              `json:"low_yield_stop_reason,omitempty"`
}

func (m *AuditMetrics) Record(cm CallMetric) {
	if m == nil {
		return
	}
	m.Calls = append(m.Calls, cm)
	if reason := strings.TrimSpace(cm.SessionRefreshReason); reason != "" {
		m.SessionRefreshes++
		if m.SessionRefreshReasons == nil {
			m.SessionRefreshReasons = make(map[string]int)
		}
		m.SessionRefreshReasons[reason]++
	}
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

func (m *AuditMetrics) TotalAcceptedFixCalls() int {
	if m == nil {
		return 0
	}
	total := 0
	for _, call := range m.Calls {
		if call.SessionType == engine.SessionAuditFix && call.ValidationResult == fixValidationAccepted {
			total++
		}
	}
	return total
}

func (m *AuditMetrics) TotalRejectedFixCalls() int {
	if m == nil {
		return 0
	}
	total := 0
	for _, call := range m.Calls {
		if call.SessionType == engine.SessionAuditFix && call.ValidationResult == fixValidationRejected {
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

func (m *AuditMetrics) cycleSummaryFromCalls(cycle int) CycleProductivity {
	summary := CycleProductivity{Cycle: cycle}
	if m == nil || cycle <= 0 {
		return summary
	}
	for _, call := range m.Calls {
		if call.Cycle != cycle {
			continue
		}
		summary.TotalCalls++
		summary.DurationMs += call.DurationMs
		switch call.SessionType {
		case engine.SessionAuditFix:
			summary.FixCalls++
			if call.WasNoOp {
				summary.NoOpFixCalls++
			}
		case engine.SessionAuditVerify:
			summary.VerifyCalls++
			summary.VerifyResolutions += call.Resolutions
		}
		summary.InputTokens += call.Tokens.Input
		summary.OutputTokens += call.Tokens.Output
		summary.TokenTotal += call.Tokens.Total
		summary.CacheReadInput += call.Tokens.CacheReadInput
		summary.CacheCreationInput += call.Tokens.CacheCreationInput
	}
	if summary.FixCalls > 0 {
		summary.FixYield = float64(summary.VerifyResolutions) / float64(summary.FixCalls)
		summary.NoOpRate = float64(summary.NoOpFixCalls) / float64(summary.FixCalls)
	}
	if summary.VerifyCalls > 0 {
		summary.VerifyYield = float64(summary.VerifyResolutions) / float64(summary.VerifyCalls)
	}
	if summary.VerifyResolutions > 0 {
		summary.MsPerResolution = float64(summary.DurationMs) / float64(summary.VerifyResolutions)
	}
	return summary
}

func (m *AuditMetrics) RecordCycleSummary(cycle int) {
	if m == nil || cycle <= 0 {
		return
	}
	summary := m.cycleSummaryFromCalls(cycle)
	for i := range m.CycleSummaries {
		if m.CycleSummaries[i].Cycle == cycle {
			m.CycleSummaries[i] = summary
			return
		}
	}
	m.CycleSummaries = append(m.CycleSummaries, summary)
}

func (m *AuditMetrics) LastCycleSummary() (CycleProductivity, bool) {
	if m == nil || len(m.CycleSummaries) == 0 {
		return CycleProductivity{}, false
	}
	return m.CycleSummaries[len(m.CycleSummaries)-1], true
}

func (m *AuditMetrics) TrailingCycleSummary(window int) (CycleProductivity, bool) {
	if m == nil || len(m.CycleSummaries) == 0 || window <= 0 {
		return CycleProductivity{}, false
	}
	if window > len(m.CycleSummaries) {
		window = len(m.CycleSummaries)
	}
	selected := m.CycleSummaries[len(m.CycleSummaries)-window:]
	summary := CycleProductivity{Cycle: selected[len(selected)-1].Cycle}
	for _, cycleSummary := range selected {
		summary.TotalCalls += cycleSummary.TotalCalls
		summary.DurationMs += cycleSummary.DurationMs
		summary.FixCalls += cycleSummary.FixCalls
		summary.NoOpFixCalls += cycleSummary.NoOpFixCalls
		summary.VerifyCalls += cycleSummary.VerifyCalls
		summary.VerifyResolutions += cycleSummary.VerifyResolutions
		summary.InputTokens += cycleSummary.InputTokens
		summary.OutputTokens += cycleSummary.OutputTokens
		summary.TokenTotal += cycleSummary.TokenTotal
		summary.CacheReadInput += cycleSummary.CacheReadInput
		summary.CacheCreationInput += cycleSummary.CacheCreationInput
	}
	if summary.FixCalls > 0 {
		summary.FixYield = float64(summary.VerifyResolutions) / float64(summary.FixCalls)
	}
	if summary.VerifyCalls > 0 {
		summary.VerifyYield = float64(summary.VerifyResolutions) / float64(summary.VerifyCalls)
	}
	if summary.FixCalls > 0 {
		summary.NoOpRate = float64(summary.NoOpFixCalls) / float64(summary.FixCalls)
	}
	if summary.VerifyResolutions > 0 {
		summary.MsPerResolution = float64(summary.DurationMs) / float64(summary.VerifyResolutions)
	}
	return summary, true
}

func (m *AuditMetrics) RecordLowYieldStrategyChange(reason string) {
	if m == nil {
		return
	}
	m.LowYieldStrategyChanges++
	if reason == "" {
		return
	}
	if m.LowYieldStrategyReasons == nil {
		m.LowYieldStrategyReasons = make(map[string]int)
	}
	m.LowYieldStrategyReasons[reason]++
}

func (m *AuditMetrics) RecordStrategyShift(shift StrategyShift) {
	if m == nil {
		return
	}
	m.StrategyShifts = append(m.StrategyShifts, shift)
}

func (m *AuditMetrics) StrategyShiftCount() int {
	if m == nil {
		return 0
	}
	return len(m.StrategyShifts)
}

func (m *AuditMetrics) LastStrategyShift() string {
	if m == nil || len(m.StrategyShifts) == 0 {
		return ""
	}
	last := m.StrategyShifts[len(m.StrategyShifts)-1]
	return formatStrategyShift(last)
}

func (m *AuditMetrics) Snapshot() AuditMetricsSnapshot {
	if m == nil {
		return AuditMetricsSnapshot{}
	}
	lastCycleSummary, _ := m.LastCycleSummary()
	trailingSummary, _ := m.TrailingCycleSummary(config.AuditLowYieldTrailingCycles)
	return AuditMetricsSnapshot{
		TotalCalls:               m.TotalCalls(),
		DurationMs:               m.TotalDurationMs(),
		NoOpFixCalls:             m.TotalNoOpFixCalls(),
		AcceptedFixCalls:         m.TotalAcceptedFixCalls(),
		RejectedFixCalls:         m.TotalRejectedFixCalls(),
		RepeatedUnchanged:        m.RepeatedUnchangedFindings,
		SuppressedUnchanged:      m.SuppressedUnchangedFindings,
		ReopenedWithNewEvidence:  m.ReopenedWithNewEvidence,
		BehaviorUnchanged:        m.BehaviorUnchangedOutcomes,
		BehaviorEscalations:      m.BehaviorUnchangedEscalations,
		SessionRefreshes:         m.SessionRefreshes,
		NoOpRate:                 m.NoOpRate(),
		VerifyCalls:              m.TotalVerifyCalls(),
		VerifyResolutions:        m.TotalVerifyResolutions(),
		VerifyYield:              m.VerifyYield(),
		LastCycleFixYield:        lastCycleSummary.FixYield,
		LastCycleVerifyYield:     lastCycleSummary.VerifyYield,
		LastCycleNoOpRate:        lastCycleSummary.NoOpRate,
		LastCycleMsPerResolution: lastCycleSummary.MsPerResolution,
		TrailingFixYield:         trailingSummary.FixYield,
		TrailingVerifyYield:      trailingSummary.VerifyYield,
		TrailingNoOpRate:         trailingSummary.NoOpRate,
		TrailingMsPerResolution:  trailingSummary.MsPerResolution,
		StrategyShiftCount:       m.StrategyShiftCount(),
		LastStrategyShift:        m.LastStrategyShift(),
		LowYieldStrategyChanges:  m.LowYieldStrategyChanges,
		LowYieldStopReason:       m.LowYieldStopReason,
	}
}

func (m *AuditMetrics) MarshalJSON() ([]byte, error) {
	type auditMetricsJSON struct {
		Calls                        []CallMetric         `json:"calls"`
		StrategyShifts               []StrategyShift      `json:"strategy_shifts,omitempty"`
		CycleSummaries               []CycleProductivity  `json:"cycle_summaries,omitempty"`
		OuterCycles                  int                  `json:"outer_cycles"`
		FixStrategy                  string               `json:"fix_strategy,omitempty"`
		ContentComplexity            ComplexityTier       `json:"content_complexity,omitempty"`
		ConvergedAtCycle             int                  `json:"converged_at_cycle,omitempty"`
		FinalFindingCount            int                  `json:"final_finding_count"`
		EscapedToBuildAudit          int                  `json:"escaped_to_build_audit,omitempty"`
		RepeatedUnchangedFindings    int                  `json:"repeated_unchanged_findings,omitempty"`
		SuppressedUnchangedFindings  int                  `json:"suppressed_unchanged_findings,omitempty"`
		ReopenedWithNewEvidence      int                  `json:"reopened_with_new_evidence,omitempty"`
		BehaviorUnchangedOutcomes    int                  `json:"behavior_unchanged_outcomes,omitempty"`
		BehaviorUnchangedEscalations int                  `json:"behavior_unchanged_escalations,omitempty"`
		SessionRefreshes             int                  `json:"session_refreshes,omitempty"`
		SessionRefreshReasons        map[string]int       `json:"session_refresh_reasons,omitempty"`
		LowYieldStrategyChanges      int                  `json:"low_yield_strategy_changes,omitempty"`
		LowYieldStrategyReasons      map[string]int       `json:"low_yield_strategy_reasons,omitempty"`
		LowYieldStopReason           string               `json:"low_yield_stop_reason,omitempty"`
		Summary                      AuditMetricsSnapshot `json:"summary"`
	}

	payload := auditMetricsJSON{
		Summary: m.Snapshot(),
	}
	if m != nil {
		payload.Calls = m.Calls
		payload.StrategyShifts = m.StrategyShifts
		payload.CycleSummaries = m.CycleSummaries
		payload.OuterCycles = m.OuterCycles
		payload.FixStrategy = m.FixStrategy
		payload.ContentComplexity = m.ContentComplexity
		payload.ConvergedAtCycle = m.ConvergedAtCycle
		payload.FinalFindingCount = m.FinalFindingCount
		payload.EscapedToBuildAudit = m.EscapedToBuildAudit
		payload.RepeatedUnchangedFindings = m.RepeatedUnchangedFindings
		payload.SuppressedUnchangedFindings = m.SuppressedUnchangedFindings
		payload.ReopenedWithNewEvidence = m.ReopenedWithNewEvidence
		payload.BehaviorUnchangedOutcomes = m.BehaviorUnchangedOutcomes
		payload.BehaviorUnchangedEscalations = m.BehaviorUnchangedEscalations
		payload.SessionRefreshes = m.SessionRefreshes
		payload.SessionRefreshReasons = m.SessionRefreshReasons
		payload.LowYieldStrategyChanges = m.LowYieldStrategyChanges
		payload.LowYieldStrategyReasons = m.LowYieldStrategyReasons
		payload.LowYieldStopReason = m.LowYieldStopReason
	}
	return json.Marshal(payload)
}
