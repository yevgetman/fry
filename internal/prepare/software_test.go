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
			"Step0_mode_marker",
			func() string { return SoftwareStep0Prompt("executive content", "", "") },
			"software architect",
		},
		{
			"Step0_input_propagation",
			func() string { return SoftwareStep0Prompt("executive content", "", "") },
			"executive content",
		},
		{
			"Step0_with_media",
			func() string { return SoftwareStep0Prompt("exec", "MEDIA_SENTINEL", "") },
			"MEDIA_SENTINEL",
		},
		{
			"Step0_with_assets",
			func() string { return SoftwareStep0Prompt("exec", "", "Asset supplement text") },
			"Asset supplement text",
		},
		{
			"Step1_structural",
			func() string { return SoftwareStep1Prompt("plan content", "executive content", "") },
			"AGENTS.md",
		},
		{
			"Step1_plan_input",
			func() string { return SoftwareStep1Prompt("plan content", "executive content", "") },
			"plan content",
		},
		{
			"Step1_executive_input",
			func() string { return SoftwareStep1Prompt("plan content", "executive content", "") },
			"executive content",
		},
		{
			"Step1_media_injection",
			func() string { return SoftwareStep1Prompt("plan", "", "MEDIA_SENTINEL") },
			"MEDIA_SENTINEL",
		},
		{
			"Step2_structural",
			func() string {
				return SoftwareStep2Prompt("plan content", "agents content", "epic-example.md", "generate-epic.md", "", epic.EffortHigh, false, "", "")
			},
			"@epic",
		},
		{
			"Step2_plan_input",
			func() string {
				return SoftwareStep2Prompt("plan content", "agents content", "epic-example.md", "generate-epic.md", "", epic.EffortHigh, false, "", "")
			},
			"plan content",
		},
		{
			"Step2_agents_input",
			func() string {
				return SoftwareStep2Prompt("plan content", "agents content", "epic-example.md", "generate-epic.md", "", epic.EffortHigh, false, "", "")
			},
			"agents content",
		},
		{
			"Step2_with_user_prompt",
			func() string {
				return SoftwareStep2Prompt("plan", "agents", "epic-ex.md", "gen-epic.md", "build the dashboard", epic.EffortHigh, false, "", "")
			},
			"build the dashboard",
		},
		{
			"Step2_with_review",
			func() string {
				return SoftwareStep2Prompt("plan", "agents", "epic-ex.md", "gen-epic.md", "", epic.EffortHigh, true, "", "")
			},
			"@review_between_sprints",
		},
		{
			"Step2_effort_low",
			func() string {
				return SoftwareStep2Prompt("plan", "agents", "epic-ex.md", "gen-epic.md", "", epic.EffortLow, false, "", "")
			},
			"EFFORT LEVEL: LOW",
		},
		{
			"Step2_effort_medium",
			func() string {
				return SoftwareStep2Prompt("plan", "agents", "epic-ex.md", "gen-epic.md", "", epic.EffortMedium, false, "", "")
			},
			"EFFORT LEVEL: MEDIUM",
		},
		{
			"Step2_effort_max",
			func() string {
				return SoftwareStep2Prompt("plan", "agents", "epic-ex.md", "gen-epic.md", "", epic.EffortMax, false, "", "")
			},
			"EFFORT LEVEL: MAX",
		},
		{
			"Step2_effort_auto",
			func() string {
				return SoftwareStep2Prompt("plan", "agents", "epic-ex.md", "gen-epic.md", "", epic.EffortLevel(""), false, "", "")
			},
			"EFFORT LEVEL: AUTO-DETECT",
		},
		{
			"Step3_structural",
			func() string {
				return SoftwareStep3Prompt("plan content", "epic content", "verification-example.md", "", "")
			},
			"@check_file",
		},
		{
			"Step3_plan_input",
			func() string {
				return SoftwareStep3Prompt("plan content", "epic content", "verification-example.md", "", "")
			},
			"plan content",
		},
		{
			"Step3_epic_input",
			func() string {
				return SoftwareStep3Prompt("plan content", "epic content", "verification-example.md", "", "")
			},
			"epic content",
		},
		{
			"Step3_with_user_prompt",
			func() string {
				return SoftwareStep3Prompt("plan", "epic", "ver-ex.md", "user directive here", "")
			},
			"user directive here",
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

	t.Run("Step1_empty_executive_omits_context", func(t *testing.T) {
		t.Parallel()
		result := SoftwareStep1Prompt("plan", "", "")
		assert.NotContains(t, result, "Also read plans/executive.md")
	})
}
