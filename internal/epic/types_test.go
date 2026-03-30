package epic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEffortLevel_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected EffortLevel
	}{
		{"low", EffortLow},
		{"medium", EffortMedium},
		{"high", EffortHigh},
		{"max", EffortMax},
		{"", EffortLevel("")},
		{"  ", EffortLevel("")},
	}
	for _, tc := range tests {
		level, err := ParseEffortLevel(tc.input)
		require.NoError(t, err, "input: %q", tc.input)
		assert.Equal(t, tc.expected, level, "input: %q", tc.input)
	}
}

func TestParseEffortLevel_Invalid(t *testing.T) {
	t.Parallel()

	invalids := []string{"extreme", "123", "mega", "lo", "hi"}
	for _, input := range invalids {
		_, err := ParseEffortLevel(input)
		require.Error(t, err, "input: %q", input)
		assert.Contains(t, err.Error(), "invalid effort level")
	}
}

func TestParseEffortLevel_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected EffortLevel
	}{
		{"Low", EffortLow},
		{"LOW", EffortLow},
		{"lOw", EffortLow},
		{"MEDIUM", EffortMedium},
		{"High", EffortHigh},
		{"MAX", EffortMax},
	}
	for _, tc := range tests {
		level, err := ParseEffortLevel(tc.input)
		require.NoError(t, err, "input: %q", tc.input)
		assert.Equal(t, tc.expected, level, "input: %q", tc.input)
	}
}

func TestEffortLevel_String(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "auto", EffortLevel("").String())
	assert.Equal(t, "low", EffortLow.String())
	assert.Equal(t, "medium", EffortMedium.String())
	assert.Equal(t, "high", EffortHigh.String())
	assert.Equal(t, "max", EffortMax.String())
}

func TestEffortLevel_DefaultMaxIterations(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 12, EffortLow.DefaultMaxIterations())
	assert.Equal(t, 20, EffortMedium.DefaultMaxIterations())
	assert.Equal(t, 25, EffortHigh.DefaultMaxIterations())
	assert.Equal(t, 40, EffortMax.DefaultMaxIterations())
	assert.Equal(t, 25, EffortLevel("").DefaultMaxIterations()) // default = high
}

func TestEffortLevel_MaxSprintCount(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 2, EffortLow.MaxSprintCount())
	assert.Equal(t, 4, EffortMedium.MaxSprintCount())
	assert.Equal(t, 10, EffortHigh.MaxSprintCount())
	assert.Equal(t, 10, EffortMax.MaxSprintCount())
	assert.Equal(t, 10, EffortLevel("").MaxSprintCount()) // default
}

func TestEffortLevel_DefaultMaxHealAttempts(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, EffortLow.DefaultMaxHealAttempts())
	assert.Equal(t, 3, EffortMedium.DefaultMaxHealAttempts())
	assert.Equal(t, 10, EffortHigh.DefaultMaxHealAttempts())
	assert.Equal(t, 0, EffortMax.DefaultMaxHealAttempts()) // unlimited, governed by progress
	assert.Equal(t, 3, EffortLevel("").DefaultMaxHealAttempts()) // auto = medium default
}

func TestEffortLevel_DefaultMaxFailPercent(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 20, EffortLow.DefaultMaxFailPercent())
	assert.Equal(t, 20, EffortMedium.DefaultMaxFailPercent())
	assert.Equal(t, 20, EffortHigh.DefaultMaxFailPercent())
	assert.Equal(t, 10, EffortMax.DefaultMaxFailPercent()) // stricter
	assert.Equal(t, 20, EffortLevel("").DefaultMaxFailPercent())
}

func TestEffortLevel_HealUsesProgressDetection(t *testing.T) {
	t.Parallel()

	assert.False(t, EffortLow.HealUsesProgressDetection())
	assert.False(t, EffortMedium.HealUsesProgressDetection())
	assert.True(t, EffortHigh.HealUsesProgressDetection())
	assert.True(t, EffortMax.HealUsesProgressDetection())
}

func TestEffortLevel_HealStuckThreshold(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, EffortLow.HealStuckThreshold())
	assert.Equal(t, 0, EffortMedium.HealStuckThreshold())
	assert.Equal(t, 2, EffortHigh.HealStuckThreshold())
	assert.Equal(t, 3, EffortMax.HealStuckThreshold())
}

func TestEffortLevel_HealHasHardCap(t *testing.T) {
	t.Parallel()

	assert.True(t, EffortLow.HealHasHardCap())
	assert.True(t, EffortMedium.HealHasHardCap())
	assert.True(t, EffortHigh.HealHasHardCap())
	assert.False(t, EffortMax.HealHasHardCap())
}

func TestEffortLevel_deviationScopeUnlimited(t *testing.T) {
	t.Parallel()

	assert.False(t, EffortLow.deviationScopeUnlimited())
	assert.True(t, EffortMedium.deviationScopeUnlimited())
	assert.True(t, EffortHigh.deviationScopeUnlimited())
	assert.True(t, EffortMax.deviationScopeUnlimited())
	assert.True(t, EffortLevel("").deviationScopeUnlimited()) // auto-detect
}
