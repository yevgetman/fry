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
