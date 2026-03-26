package prepare

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yevgetman/fry/internal/epic"
)

func TestWritingPromptBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fn     func() string
		marker string
	}{
		{
			"Step0_mode_marker",
			func() string { return WritingStep0Prompt("executive content", "", "") },
			"author and content architect",
		},
		{
			"Step0_input_propagation",
			func() string { return WritingStep0Prompt("executive content", "", "") },
			"executive content",
		},
		{
			"Step0_with_media",
			func() string { return WritingStep0Prompt("exec", "MEDIA_SENTINEL", "") },
			"MEDIA_SENTINEL",
		},
		{
			"Step0_with_assets",
			func() string { return WritingStep0Prompt("exec", "", "Asset supplement text") },
			"Asset supplement text",
		},
		{
			"Step1_structural",
			func() string { return WritingStep1Prompt("plan content", "executive content", "") },
			"AGENTS.md",
		},
		{
			"Step1_plan_input",
			func() string { return WritingStep1Prompt("plan content", "executive content", "") },
			"plan content",
		},
		{
			"Step1_executive_input",
			func() string { return WritingStep1Prompt("plan content", "executive content", "") },
			"executive content",
		},
		{
			"Step1_writing_mode_marker",
			func() string { return WritingStep1Prompt("plan", "", "") },
			"WRITING project",
		},
		{
			"Step1_media_injection",
			func() string { return WritingStep1Prompt("plan", "", "MEDIA_SENTINEL") },
			"MEDIA_SENTINEL",
		},
		{
			"Step2_mode_marker",
			func() string {
				return WritingStep2Prompt("plan content", "agents content", "epic-example.md", "", epic.EffortHigh, false, "", "")
			},
			"WRITING project",
		},
		{
			"Step2_plan_input",
			func() string {
				return WritingStep2Prompt("plan content", "agents content", "epic-example.md", "", epic.EffortHigh, false, "", "")
			},
			"plan content",
		},
		{
			"Step2_agents_input",
			func() string {
				return WritingStep2Prompt("plan content", "agents content", "epic-example.md", "", epic.EffortHigh, false, "", "")
			},
			"agents content",
		},
		{
			"Step2_with_user_prompt",
			func() string {
				return WritingStep2Prompt("plan", "agents", "epic-ex.md", "write the chapter", epic.EffortHigh, false, "", "")
			},
			"write the chapter",
		},
		{
			"Step2_with_review",
			func() string {
				return WritingStep2Prompt("plan", "agents", "epic-ex.md", "", epic.EffortHigh, true, "", "")
			},
			"@review_between_sprints",
		},
		{
			"Step2_effort_low",
			func() string {
				return WritingStep2Prompt("plan", "agents", "epic-ex.md", "", epic.EffortLow, false, "", "")
			},
			"EFFORT LEVEL: LOW",
		},
		{
			"Step2_effort_medium",
			func() string {
				return WritingStep2Prompt("plan", "agents", "epic-ex.md", "", epic.EffortMedium, false, "", "")
			},
			"EFFORT LEVEL: MEDIUM",
		},
		{
			"Step2_effort_max",
			func() string {
				return WritingStep2Prompt("plan", "agents", "epic-ex.md", "", epic.EffortMax, false, "", "")
			},
			"EFFORT LEVEL: MAX",
		},
		{
			"Step2_effort_auto",
			func() string {
				return WritingStep2Prompt("plan", "agents", "epic-ex.md", "", epic.EffortLevel(""), false, "", "")
			},
			"EFFORT LEVEL: AUTO-DETECT",
		},
		{
			"Step3_structural",
			func() string {
				return WritingStep3Prompt("plan content", "epic content", "verification-example.md", "", "")
			},
			"@check_file",
		},
		{
			"Step3_plan_input",
			func() string {
				return WritingStep3Prompt("plan content", "epic content", "verification-example.md", "", "")
			},
			"plan content",
		},
		{
			"Step3_epic_input",
			func() string {
				return WritingStep3Prompt("plan content", "epic content", "verification-example.md", "", "")
			},
			"epic content",
		},
		{
			"Step3_with_user_prompt",
			func() string {
				return WritingStep3Prompt("plan", "epic", "ver-ex.md", "user directive here", "")
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
		result := WritingStep1Prompt("plan", "", "")
		assert.NotContains(t, result, "Also read plans/executive.md")
	})
}
