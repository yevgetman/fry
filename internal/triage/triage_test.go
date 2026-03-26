package triage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/prepare"
)

type stubEngine struct {
	output string
	err    error
}

func (s *stubEngine) Run(_ context.Context, _ string, _ engine.RunOpts) (string, int, error) {
	return s.output, 0, s.err
}

func (s *stubEngine) Name() string { return "stub" }

func TestParseClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		output         string
		wantComplexity Complexity
		wantEffort     epic.EffortLevel
		wantSprints    int
		wantReason     string
	}{
		{
			name: "simple task with effort",
			output: `<complexity>SIMPLE</complexity>
<effort>low</effort>
<sprints>1</sprints>
<reason>Single config file change.</reason>`,
			wantComplexity: ComplexitySimple,
			wantEffort:     epic.EffortLow,
			wantSprints:    1,
			wantReason:     "Single config file change.",
		},
		{
			name: "moderate task with effort",
			output: `<complexity>MODERATE</complexity>
<effort>medium</effort>
<sprints>2</sprints>
<reason>REST endpoint with tests across 6 files.</reason>`,
			wantComplexity: ComplexityModerate,
			wantEffort:     epic.EffortMedium,
			wantSprints:    2,
			wantReason:     "REST endpoint with tests across 6 files.",
		},
		{
			name: "complex task",
			output: `<complexity>COMPLEX</complexity>
<effort>high</effort>
<sprints>0</sprints>
<reason>Full-stack app with database and Docker.</reason>`,
			wantComplexity: ComplexityComplex,
			wantEffort:     epic.EffortHigh,
			wantSprints:    0,
			wantReason:     "Full-stack app with database and Docker.",
		},
		{
			name: "effort tag missing defaults to empty",
			output: `<complexity>SIMPLE</complexity>
<sprints>1</sprints>
<reason>Tiny change.</reason>`,
			wantComplexity: ComplexitySimple,
			wantEffort:     "",
			wantSprints:    1,
			wantReason:     "Tiny change.",
		},
		{
			name: "max effort excluded by regex",
			output: `<complexity>MODERATE</complexity>
<effort>max</effort>
<sprints>2</sprints>
<reason>Big feature.</reason>`,
			wantComplexity: ComplexityModerate,
			wantEffort:     "", // max not matched by regex
			wantSprints:    2,
			wantReason:     "Big feature.",
		},
		{
			name: "invalid effort value ignored",
			output: `<complexity>SIMPLE</complexity>
<effort>extreme</effort>
<sprints>1</sprints>
<reason>A task.</reason>`,
			wantComplexity: ComplexitySimple,
			wantEffort:     "", // not matched by regex
			wantSprints:    1,
			wantReason:     "A task.",
		},
		{
			name:           "unparseable defaults to complex",
			output:         "I think this is a medium-sized task.",
			wantComplexity: ComplexityComplex,
			wantSprints:    0,
			wantReason:     "could not parse classifier output",
		},
		{
			name:           "empty output defaults to complex",
			output:         "",
			wantComplexity: ComplexityComplex,
			wantSprints:    0,
			wantReason:     "could not parse classifier output",
		},
		{
			name: "invalid complexity value defaults to complex",
			output: `<complexity>EASY</complexity>
<sprints>1</sprints>
<reason>Tiny change.</reason>`,
			wantComplexity: ComplexityComplex,
			wantSprints:    0,
			wantReason:     "could not parse classifier output",
		},
		{
			name: "simple with no sprints tag",
			output: `<complexity>SIMPLE</complexity>
<reason>Just a typo fix.</reason>`,
			wantComplexity: ComplexitySimple,
			wantSprints:    0,
			wantReason:     "Just a typo fix.",
		},
		{
			name: "complexity tag with surrounding text",
			output: `Here is my analysis:
<complexity>MODERATE</complexity>
<effort>high</effort>
<sprints>2</sprints>
<reason>Multi-file change needed.</reason>
Thank you!`,
			wantComplexity: ComplexityModerate,
			wantEffort:     epic.EffortHigh,
			wantSprints:    2,
			wantReason:     "Multi-file change needed.",
		},
		{
			name: "multiline reason",
			output: `<complexity>COMPLEX</complexity>
<sprints>0</sprints>
<reason>This task involves multiple subsystems
and requires database migrations.</reason>`,
			wantComplexity: ComplexityComplex,
			wantSprints:    0,
			wantReason:     "This task involves multiple subsystems\nand requires database migrations.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ParseClassification(tt.output)
			assert.Equal(t, tt.wantComplexity, got.Complexity)
			assert.Equal(t, tt.wantEffort, got.EffortLevel)
			assert.Equal(t, tt.wantSprints, got.SprintCount)
			if tt.wantReason != "" {
				assert.Equal(t, tt.wantReason, got.Reason)
			}
		})
	}
}


func TestBuildTriagePrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		opts         TriageOpts
		wantContains []string
		wantAbsent   []string
	}{
		{
			name: "software mode with plan",
			opts: TriageOpts{
				PlanContent: "Build a REST API",
				Mode:        prepare.ModeSoftware,
			},
			wantContains: []string{
				"CRITICAL BIAS RULE",
				"ALWAYS classify as COMPLEX",
				"Build a REST API",
				"Build Plan",
				"1-3 files",
				"Effort Level Guidelines",
				"<effort>low|medium|high</effort>",
			},
		},
		{
			name: "writing mode",
			opts: TriageOpts{
				UserPrompt: "Write a blog post about Go",
				Mode:       prepare.ModeWriting,
			},
			wantContains: []string{
				"CRITICAL BIAS RULE",
				"Short-form writing",
				"blog post",
				"Write a blog post about Go",
				"User Directive",
				"Effort Level Guidelines",
			},
			wantAbsent: []string{
				"1-3 files",
			},
		},
		{
			name: "planning mode",
			opts: TriageOpts{
				ExecContent: "Strategic plan for Q4",
				Mode:        prepare.ModePlanning,
			},
			wantContains: []string{
				"CRITICAL BIAS RULE",
				"Single deliverable",
				"Strategic plan for Q4",
				"Executive Context",
				"Effort Level Guidelines",
			},
			wantAbsent: []string{
				"1-3 files",
			},
		},
		{
			name: "no inputs",
			opts: TriageOpts{
				Mode: prepare.ModeSoftware,
			},
			wantContains: []string{
				"No project inputs available",
				"classify as COMPLEX",
				"Effort Level Guidelines",
			},
		},
		{
			name: "all inputs present",
			opts: TriageOpts{
				PlanContent: "the plan",
				ExecContent: "the executive",
				UserPrompt:  "the directive",
				Mode:        prepare.ModeSoftware,
			},
			wantContains: []string{
				"the plan",
				"the executive",
				"the directive",
				"Build Plan",
				"Executive Context",
				"User Directive",
				"Effort Level Guidelines",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prompt := buildTriagePrompt(tt.opts)
			for _, s := range tt.wantContains {
				assert.Contains(t, prompt, s, "prompt should contain %q", s)
			}
			for _, s := range tt.wantAbsent {
				assert.NotContains(t, prompt, s, "prompt should not contain %q", s)
			}
		})
	}
}

func TestTruncateReason(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "short", truncateReason("short", 80))
	assert.Equal(t, "hello wo...", truncateReason("hello world!", 11))
	assert.Equal(t, "", truncateReason("", 80))
	// Verify rune-safe truncation: multi-byte characters should not be split.
	assert.Equal(t, "ab...", truncateReason("abcdef", 5))
	assert.Equal(t, "hello", truncateReason("hello", 6))
	// Multi-byte runes: 4 runes "abcd" in 4-byte chars, maxLen=5 runes truncates to 2 runes + "..."
	assert.Equal(t, "ab...", truncateReason("abcdefgh", 5))
}

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		output         string
		engineErr      error
		wantComplexity Complexity
		wantEffort     epic.EffortLevel
		wantSprints    int
	}{
		{
			name: "simple classification yields LOW effort and 1 sprint",
			output: `<complexity>SIMPLE</complexity>
<effort>low</effort>
<sprints>1</sprints>
<reason>Single config file change.</reason>`,
			wantComplexity: ComplexitySimple,
			wantEffort:     epic.EffortLow,
			wantSprints:    1,
		},
		{
			name: "moderate classification yields MEDIUM effort and 2 sprints",
			output: `<complexity>MODERATE</complexity>
<effort>medium</effort>
<sprints>2</sprints>
<reason>Multi-file REST endpoint with tests.</reason>`,
			wantComplexity: ComplexityModerate,
			wantEffort:     epic.EffortMedium,
			wantSprints:    2,
		},
		{
			name: "complex classification",
			output: `<complexity>COMPLEX</complexity>
<effort>high</effort>
<sprints>0</sprints>
<reason>Full-stack app with Docker and database.</reason>`,
			wantComplexity: ComplexityComplex,
			wantEffort:     epic.EffortHigh,
			wantSprints:    0,
		},
		{
			name:           "engine error with no output defaults to complex",
			output:         "",
			engineErr:      errors.New("engine failed"),
			wantComplexity: ComplexityComplex,
		},
		{
			name:           "malformed XML output defaults to complex",
			output:         "I think this is a medium-sized task.",
			wantComplexity: ComplexityComplex,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			stub := &stubEngine{output: tc.output, err: tc.engineErr}

			decision := Classify(context.Background(), TriageOpts{
				ProjectDir: dir,
				UserPrompt: "test task",
				Engine:     stub,
			})

			require.NotNil(t, decision)
			assert.Equal(t, tc.wantComplexity, decision.Complexity)
			if tc.wantEffort != "" {
				assert.Equal(t, tc.wantEffort, decision.EffortLevel)
			}
			assert.Equal(t, tc.wantSprints, decision.SprintCount)
		})
	}
}
