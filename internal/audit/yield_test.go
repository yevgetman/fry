package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yevgetman/fry/internal/config"
)

func TestIsLowYieldStrategyCycle_BelowMinFixCalls(t *testing.T) {
	t.Parallel()
	cycle := CycleProductivity{FixCalls: 1, FixYield: 0.0, NoOpRate: 1.0}
	assert.False(t, isLowYieldStrategyCycle(cycle))
}

func TestIsLowYieldStrategyCycle_LowFixYield(t *testing.T) {
	t.Parallel()
	cycle := CycleProductivity{FixCalls: 4, FixYield: 0.25, VerifyCalls: 2, VerifyYield: 1.0}
	assert.True(t, isLowYieldStrategyCycle(cycle))
}

func TestIsLowYieldStrategyCycle_LowVerifyYield(t *testing.T) {
	t.Parallel()
	cycle := CycleProductivity{FixCalls: 4, FixYield: 1.0, VerifyCalls: 4, VerifyYield: 0.50}
	assert.True(t, isLowYieldStrategyCycle(cycle))
}

func TestIsLowYieldStrategyCycle_HighNoOpWithNoResolutions(t *testing.T) {
	t.Parallel()
	// High no-op rate + zero verify resolutions → low-yield
	cycle := CycleProductivity{FixCalls: 4, NoOpFixCalls: 4, FixYield: 0.0, NoOpRate: 1.0, VerifyResolutions: 0}
	assert.True(t, isLowYieldStrategyCycle(cycle))
}

func TestIsLowYieldStrategyCycle_HighNoOpWithResolutions(t *testing.T) {
	t.Parallel()
	// High no-op rate but verify confirmed resolutions ("rejected-but-landed" pattern).
	// FixYield = 3/4 = 0.75 > floor, VerifyYield = 3/3 = 1.0 > floor, so neither of
	// the first two conditions fire. NoOpRate >= floor but VerifyResolutions > 0, so the
	// third condition should NOT fire. Cycle is productive.
	cycle := CycleProductivity{
		FixCalls:          4,
		NoOpFixCalls:      4,
		NoOpRate:          1.0,
		VerifyCalls:       3,
		VerifyResolutions: 3,
		FixYield:          0.75,
		VerifyYield:       1.0,
	}
	assert.False(t, isLowYieldStrategyCycle(cycle))
}

func TestIsLowYieldStrategyCycle_HighNoOpLowFixYieldStillCaught(t *testing.T) {
	t.Parallel()
	// Even when verify resolves some issues, if FixYield is below floor the cycle
	// is still classified as low-yield via the first condition.
	cycle := CycleProductivity{
		FixCalls:          4,
		NoOpFixCalls:      4,
		NoOpRate:          1.0,
		VerifyCalls:       2,
		VerifyResolutions: 1,
		FixYield:          0.25, // 1/4 — below floor
		VerifyYield:       0.50,
	}
	assert.True(t, isLowYieldStrategyCycle(cycle))
}

func TestShouldStopForLowYieldCycle_BelowThreshold(t *testing.T) {
	t.Parallel()
	current := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	trailing := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	assert.False(t, shouldStopForLowYieldCycle(current, trailing, 1, config.AuditLowYieldStopCycles))
}

func TestShouldStopForLowYieldCycle_StandardEffort(t *testing.T) {
	t.Parallel()
	current := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	trailing := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	// Standard threshold = 2, streak = 2 → stop
	assert.True(t, shouldStopForLowYieldCycle(current, trailing, 2, config.AuditLowYieldStopCycles))
}

func TestShouldStopForLowYieldCycle_HighEffortDelaysStop(t *testing.T) {
	t.Parallel()
	current := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	trailing := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	// High threshold = 3, streak = 2 → don't stop yet
	assert.False(t, shouldStopForLowYieldCycle(current, trailing, 2, config.AuditLowYieldStopCyclesHigh))
	// streak = 3 → stop
	assert.True(t, shouldStopForLowYieldCycle(current, trailing, 3, config.AuditLowYieldStopCyclesHigh))
}

func TestShouldStopForLowYieldCycle_MaxEffortDelaysStop(t *testing.T) {
	t.Parallel()
	current := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	trailing := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	// Max threshold = 5
	assert.False(t, shouldStopForLowYieldCycle(current, trailing, 4, config.AuditLowYieldStopCyclesMax))
	assert.True(t, shouldStopForLowYieldCycle(current, trailing, 5, config.AuditLowYieldStopCyclesMax))
}

func TestShouldStopForLowYieldCycle_TrailingNoOpWithResolutionsNotLow(t *testing.T) {
	t.Parallel()
	// Trailing has high no-op rate but also verify resolutions — trailing should NOT
	// be considered low, so the stop condition should not fire.
	current := CycleProductivity{FixCalls: 4, FixYield: 0.25}
	trailing := CycleProductivity{
		FixCalls:          8,
		NoOpRate:          1.0,
		FixYield:          0.75,
		VerifyCalls:       6,
		VerifyYield:       1.0,
		VerifyResolutions: 6,
	}
	assert.False(t, shouldStopForLowYieldCycle(current, trailing, 2, config.AuditLowYieldStopCycles))
}

func TestShouldStopForLowYieldCycle_TrailingNoOpWithoutResolutionsIsLow(t *testing.T) {
	t.Parallel()
	// Trailing has high no-op rate and zero resolutions — trailing IS low.
	current := CycleProductivity{FixCalls: 4, FixYield: 0.0}
	trailing := CycleProductivity{
		FixCalls:          8,
		NoOpRate:          1.0,
		FixYield:          0.0,
		VerifyResolutions: 0,
	}
	assert.True(t, shouldStopForLowYieldCycle(current, trailing, 2, config.AuditLowYieldStopCycles))
}
