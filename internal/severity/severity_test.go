package severity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRank(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  int
	}{
		{"CRITICAL", 4},
		{"HIGH", 3},
		{"MODERATE", 2},
		{"LOW", 1},
		{"", 0},
		{"UNKNOWN", 0},
		{"critical", 0},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, Rank(tc.input))
		})
	}
}
