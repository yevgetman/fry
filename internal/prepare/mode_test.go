package prepare

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMode_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  Mode
	}{
		{"software", ModeSoftware},
		{"planning", ModePlanning},
		{"writing", ModeWriting},
		{"SOFTWARE", ModeSoftware},
		{"Planning", ModePlanning},
		{"WRITING", ModeWriting},
		{" writing ", ModeWriting},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMode(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseMode_Empty(t *testing.T) {
	t.Parallel()

	got, err := ParseMode("")
	require.NoError(t, err)
	assert.Equal(t, ModeSoftware, got)
}

func TestParseMode_Invalid(t *testing.T) {
	t.Parallel()

	_, err := ParseMode("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mode")
}
