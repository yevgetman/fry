package triage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/prepare"
)

func TestParseClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		output         string
		wantComplexity Complexity
		wantSprints    int
		wantReason     string
	}{
		{
			name: "simple task",
			output: `<complexity>SIMPLE</complexity>
<sprints>1</sprints>
<reason>Single config file change.</reason>`,
			wantComplexity: ComplexitySimple,
			wantSprints:    1,
			wantReason:     "Single config file change.",
		},
		{
			name: "moderate task",
			output: `<complexity>MODERATE</complexity>
<sprints>2</sprints>
<reason>REST endpoint with tests across 6 files.</reason>`,
			wantComplexity: ComplexityModerate,
			wantSprints:    2,
			wantReason:     "REST endpoint with tests across 6 files.",
		},
		{
			name: "complex task",
			output: `<complexity>COMPLEX</complexity>
<sprints>0</sprints>
<reason>Full-stack app with database and Docker.</reason>`,
			wantComplexity: ComplexityComplex,
			wantSprints:    0,
			wantReason:     "Full-stack app with database and Docker.",
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
<sprints>2</sprints>
<reason>Multi-file change needed.</reason>
Thank you!`,
			wantComplexity: ComplexityModerate,
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
			assert.Equal(t, tt.wantSprints, got.SprintCount)
			if tt.wantReason != "" {
				assert.Equal(t, tt.wantReason, got.Reason)
			}
		})
	}
}

func TestBuildSimpleEpic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		opts        SimpleEpicOpts
		wantErr     bool
		wantPrompt  string
		wantSprints int
	}{
		{
			name: "with plan content",
			opts: SimpleEpicOpts{
				ProjectDir:  "/tmp/test",
				PlanContent: "Build a CLI tool that converts CSV to JSON",
				EngineName:  "claude",
				Mode:        prepare.ModeSoftware,
			},
			wantPrompt:  "Build a CLI tool that converts CSV to JSON",
			wantSprints: 1,
		},
		{
			name: "with only user prompt",
			opts: SimpleEpicOpts{
				ProjectDir: "/tmp/test",
				UserPrompt: "Fix the typo in README.md",
				EngineName: "claude",
				Mode:       prepare.ModeSoftware,
			},
			wantPrompt:  "Fix the typo in README.md",
			wantSprints: 1,
		},
		{
			name: "with exec content as fallback",
			opts: SimpleEpicOpts{
				ProjectDir:  "/tmp/test",
				ExecContent: "Executive context for the project",
				EngineName:  "codex",
				Mode:        prepare.ModeSoftware,
			},
			wantPrompt:  "Executive context for the project",
			wantSprints: 1,
		},
		{
			name: "plan takes precedence over user prompt",
			opts: SimpleEpicOpts{
				ProjectDir:  "/tmp/test",
				PlanContent: "The plan",
				UserPrompt:  "The user prompt",
				EngineName:  "claude",
				Mode:        prepare.ModeSoftware,
			},
			wantPrompt:  "The plan",
			wantSprints: 1,
		},
		{
			name: "empty content returns error",
			opts: SimpleEpicOpts{
				ProjectDir: "/tmp/test",
				EngineName: "claude",
				Mode:       prepare.ModeSoftware,
			},
			wantErr: true,
		},
		{
			name: "whitespace-only content returns error",
			opts: SimpleEpicOpts{
				ProjectDir:  "/tmp/test",
				PlanContent: "   \n  \t  ",
				EngineName:  "claude",
				Mode:        prepare.ModeSoftware,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ep, err := BuildSimpleEpic(tt.opts)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, ep)

			assert.Equal(t, "Simple Task", ep.Name)
			assert.Equal(t, tt.opts.EngineName, ep.Engine)
			assert.Equal(t, epic.EffortLow, ep.EffortLevel)
			assert.Equal(t, 0, ep.MaxHealAttempts)
			assert.False(t, ep.AuditAfterSprint)
			assert.False(t, ep.ReviewBetweenSprints)
			assert.Equal(t, tt.wantSprints, len(ep.Sprints))
			assert.Equal(t, tt.wantSprints, ep.TotalSprints)

			if tt.wantSprints > 0 {
				s := ep.Sprints[0]
				assert.Equal(t, 1, s.Number)
				assert.Equal(t, "Execute task", s.Name)
				assert.Equal(t, 12, s.MaxIterations)
				assert.Equal(t, "SIMPLE_TASK_COMPLETE", s.Promise)
				assert.Equal(t, tt.wantPrompt, s.Prompt)
			}

			// Verify the Epic passes validation.
			require.NoError(t, epic.ValidateEpic(ep))
		})
	}
}

func TestWriteEpicFile(t *testing.T) {
	t.Parallel()

	t.Run("roundtrip simple epic", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		epicPath := filepath.Join(dir, ".fry", "epic.md")

		original := &epic.Epic{
			Name:             "Test Epic",
			Engine:           "claude",
			EffortLevel:      epic.EffortLow,
			MaxHealAttempts:  0,
			AuditAfterSprint: false,
			TotalSprints:     1,
			Sprints: []epic.Sprint{{
				Number:        1,
				Name:          "Do the thing",
				MaxIterations: 12,
				Promise:       "DONE",
				Prompt:        "Build a widget that does stuff.\nIt should have tests.",
			}},
		}

		err := WriteEpicFile(epicPath, original)
		require.NoError(t, err)

		// Verify the file was written.
		_, statErr := os.Stat(epicPath)
		require.NoError(t, statErr)

		// Parse it back and verify roundtrip.
		parsed, parseErr := epic.ParseEpic(epicPath)
		require.NoError(t, parseErr)
		require.NoError(t, epic.ValidateEpic(parsed))

		assert.Equal(t, original.Name, parsed.Name)
		assert.Equal(t, original.Engine, parsed.Engine)
		assert.Equal(t, original.EffortLevel, parsed.EffortLevel)
		assert.False(t, parsed.AuditAfterSprint)
		require.Len(t, parsed.Sprints, 1)

		s := parsed.Sprints[0]
		assert.Equal(t, 1, s.Number)
		assert.Equal(t, "Do the thing", s.Name)
		assert.Equal(t, 12, s.MaxIterations)
		assert.Equal(t, "DONE", s.Promise)
		assert.Contains(t, s.Prompt, "Build a widget that does stuff.")
		assert.Contains(t, s.Prompt, "It should have tests.")
	})

	t.Run("roundtrip multi-sprint epic", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		epicPath := filepath.Join(dir, ".fry", "epic.md")

		original := &epic.Epic{
			Name:             "Multi Sprint",
			Engine:           "codex",
			EffortLevel:      epic.EffortMedium,
			MaxHealAttempts:  3,
			AuditAfterSprint: true,
			TotalSprints:     2,
			Sprints: []epic.Sprint{
				{
					Number:        1,
					Name:          "Sprint one",
					MaxIterations: 20,
					Promise:       "SPRINT1_DONE",
					Prompt:        "First sprint instructions.",
				},
				{
					Number:        2,
					Name:          "Sprint two",
					MaxIterations: 20,
					Promise:       "SPRINT2_DONE",
					Prompt:        "Second sprint instructions.",
				},
			},
		}

		err := WriteEpicFile(epicPath, original)
		require.NoError(t, err)

		parsed, parseErr := epic.ParseEpic(epicPath)
		require.NoError(t, parseErr)
		require.NoError(t, epic.ValidateEpic(parsed))

		assert.Equal(t, "Multi Sprint", parsed.Name)
		assert.Equal(t, "codex", parsed.Engine)
		assert.Equal(t, epic.EffortMedium, parsed.EffortLevel)
		require.Len(t, parsed.Sprints, 2)
		assert.Equal(t, "Sprint one", parsed.Sprints[0].Name)
		assert.Equal(t, "Sprint two", parsed.Sprints[1].Name)
	})
}

func TestBuildTriagePrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		opts          TriageOpts
		wantContains  []string
		wantAbsent    []string
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
}
