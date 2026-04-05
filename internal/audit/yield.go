package audit

import (
	"fmt"
	"strings"

	"github.com/yevgetman/fry/internal/config"
)

const (
	strategyTriggerBehaviorUnchanged     = "behavior_unchanged"
	strategyTriggerNoOpRate              = "no_op_rate"
	strategyTriggerLowYield              = "low_yield"
	strategyTriggerTokenBurn             = "token_burn"
	strategyTriggerCachePressure         = "cache_pressure"
	strategyActionNarrowBatch            = "narrow_batch"
	lowYieldStrategySingleIssueNextCycle = "single_issue_next_cycle"
	strategyActionRefreshAuditSession    = "refresh_audit_session"
	strategyActionRefreshFixSession      = "refresh_fix_session"
	strategyActionStopAudit              = "stop_audit"
	strategyActionStopFixLoop            = "stop_fix_loop"
	lowYieldSingleIssueBatchLimit        = 1
)

func isLowYieldStrategyCycle(current CycleProductivity) bool {
	if current.FixCalls < config.AuditLowYieldMinFixCalls {
		return false
	}
	if current.FixYield <= config.AuditLowYieldFixYieldFloor {
		return true
	}
	if current.VerifyCalls > 0 && current.VerifyYield <= config.AuditLowYieldVerifyYieldFloor {
		return true
	}
	// High no-op rate is a low-yield signal only when issues aren't being resolved
	// through the verify path. When the fix agent produces rejected or no-op diffs
	// but verify confirms resolutions ("rejected-but-landed" pattern), the cycle
	// is productive and should not be classified as low-yield.
	if current.NoOpRate >= config.AuditLowYieldNoOpRateFloor && current.VerifyResolutions == 0 {
		return true
	}
	return false
}

func shouldStopForLowYieldCycle(current, trailing CycleProductivity, streak, stopThreshold int) bool {
	if streak < stopThreshold {
		return false
	}
	if current.FixCalls < config.AuditLowYieldMinFixCalls {
		return false
	}
	trailingLow := trailing.FixCalls >= config.AuditLowYieldMinFixCalls &&
		(trailing.FixYield <= config.AuditLowYieldFixYieldFloor ||
			(trailing.VerifyCalls > 0 && trailing.VerifyYield <= config.AuditLowYieldVerifyYieldFloor) ||
			(trailing.NoOpRate >= config.AuditLowYieldNoOpRateFloor && trailing.VerifyResolutions == 0))
	currentLow := current.FixYield <= config.AuditLowYieldFixYieldFloor ||
		(current.VerifyCalls > 0 && current.VerifyYield <= config.AuditLowYieldVerifyYieldFloor)
	return currentLow && trailingLow
}

func formatLowYieldStopReason(current, trailing CycleProductivity) string {
	parts := []string{
		fmt.Sprintf("low_yield cycle=%d", current.Cycle),
		fmt.Sprintf("fix_yield=%.2f", current.FixYield),
		fmt.Sprintf("verify_yield=%.2f", current.VerifyYield),
		fmt.Sprintf("no_op_rate=%.0f%%", current.NoOpRate*100),
	}
	if trailing.Cycle > 0 {
		parts = append(parts,
			fmt.Sprintf("trailing_fix_yield=%.2f", trailing.FixYield),
			fmt.Sprintf("trailing_verify_yield=%.2f", trailing.VerifyYield),
			fmt.Sprintf("trailing_no_op_rate=%.0f%%", trailing.NoOpRate*100),
		)
	}
	return strings.Join(parts, " ")
}

func hasTokenBurnPressure(current CycleProductivity) bool {
	return current.TokenTotal >= config.AuditGovernorTokenBurnThreshold
}

func hasCachePressure(current CycleProductivity) bool {
	if current.CacheReadInput >= config.AuditGovernorCacheReadThreshold {
		return true
	}
	if current.TokenTotal <= 0 {
		return false
	}
	return float64(current.CacheReadInput) >= float64(current.TokenTotal)*config.AuditGovernorCachePressureMultiplier
}

func formatStrategyShift(shift StrategyShift) string {
	parts := []string{shift.Trigger, shift.Action}
	if shift.Detail != "" {
		parts = append(parts, shift.Detail)
	}
	return strings.Join(parts, ": ")
}
