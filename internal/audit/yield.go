package audit

import (
	"fmt"
	"strings"

	"github.com/yevgetman/fry/internal/config"
)

const (
	lowYieldStrategySingleIssueNextCycle = "single_issue_next_cycle"
	lowYieldSingleIssueBatchLimit        = 1
)

func isLowYieldStrategyCycle(current CycleProductivity) bool {
	if current.FixCalls < config.AuditLowYieldMinFixCalls {
		return false
	}
	return current.FixYield <= config.AuditLowYieldFixYieldFloor ||
		(current.VerifyCalls > 0 && current.VerifyYield <= config.AuditLowYieldVerifyYieldFloor) ||
		current.NoOpRate >= config.AuditLowYieldNoOpRateFloor
}

func shouldStopForLowYieldCycle(current, trailing CycleProductivity, streak int) bool {
	if streak < config.AuditLowYieldStopCycles {
		return false
	}
	if current.FixCalls < config.AuditLowYieldMinFixCalls {
		return false
	}
	trailingLow := trailing.FixCalls >= config.AuditLowYieldMinFixCalls &&
		(trailing.FixYield <= config.AuditLowYieldFixYieldFloor ||
			(trailing.VerifyCalls > 0 && trailing.VerifyYield <= config.AuditLowYieldVerifyYieldFloor) ||
			trailing.NoOpRate >= config.AuditLowYieldNoOpRateFloor)
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
