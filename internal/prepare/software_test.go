package prepare

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yevgetman/fry/internal/epic"
)

func TestSoftwarePromptBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fn     func() string
		marker string
	}{
		{
			"Step0",
			func() string { return SoftwareStep0Prompt("executive content", "", "") },
			"executive",
		},
		{
			"Step1",
			func() string { return SoftwareStep1Prompt("plan content", "executive content", "") },
			"AGENTS.md",
		},
		{
			"Step2",
			func() string {
				return SoftwareStep2Prompt("plan content", "agents content", "epic-example.md", "generate-epic.md", "", epic.EffortHigh, false, "", "")
			},
			"@epic",
		},
		{
			"Step3",
			func() string {
				return SoftwareStep3Prompt("plan content", "epic content", "verification-example.md", "", "")
			},
			"verification",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := tc.fn()
			assert.NotEmpty(t, result)
			assert.Contains(t, result, tc.marker)
		})
	}
}
