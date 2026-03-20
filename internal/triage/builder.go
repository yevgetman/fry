package triage

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/prepare"
	"github.com/yevgetman/fry/internal/textutil"
	"github.com/yevgetman/fry/templates"
)

// SimpleEpicOpts configures the programmatic epic builder for simple tasks.
type SimpleEpicOpts struct {
	ProjectDir  string
	UserPrompt  string
	PlanContent string
	ExecContent string
	EngineName  string
	Mode        prepare.Mode
}

// BuildSimpleEpic constructs a minimal Epic struct for simple tasks.
// No LLM call — purely programmatic.
func BuildSimpleEpic(opts SimpleEpicOpts) (*epic.Epic, error) {
	prompt := opts.PlanContent
	if prompt == "" {
		prompt = opts.ExecContent
	}
	if prompt == "" {
		prompt = opts.UserPrompt
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("build simple epic: no content available for sprint prompt")
	}

	ep := &epic.Epic{
		Name:                 "Simple Task",
		Engine:               opts.EngineName,
		EffortLevel:          epic.EffortLow,
		MaxHealAttempts:      0,
		MaxFailPercent:       config.DefaultMaxFailPercent,
		VerificationFile:     config.DefaultVerificationFile,
		AuditAfterSprint:     false,
		ReviewBetweenSprints: false,
		TotalSprints:         1,
		Sprints: []epic.Sprint{{
			Number:        1,
			Name:          "Execute task",
			MaxIterations: epic.EffortLow.DefaultMaxIterations(),
			Promise:       "SIMPLE_TASK_COMPLETE",
			Prompt:        prompt,
		}},
	}
	return ep, nil
}

// WriteEpicFile serializes an Epic struct into the @epic/@sprint file format
// and writes it to the given path. The output can be parsed back by epic.ParseEpic.
func WriteEpicFile(path string, ep *epic.Epic) error {
	var b strings.Builder

	fmt.Fprintf(&b, "@epic %s\n", ep.Name)
	if ep.Engine != "" {
		fmt.Fprintf(&b, "@engine %s\n", ep.Engine)
	}
	if ep.EffortLevel != "" {
		fmt.Fprintf(&b, "@effort %s\n", ep.EffortLevel)
	}
	if ep.MaxHealAttempts > 0 {
		fmt.Fprintf(&b, "@max_heal_attempts %d\n", ep.MaxHealAttempts)
	} else if ep.MaxHealAttempts == 0 {
		b.WriteString("@max_heal_attempts 0\n")
	}
	if ep.MaxFailPercent > 0 && ep.MaxFailPercent != config.DefaultMaxFailPercent {
		fmt.Fprintf(&b, "@max_fail_percent %d\n", ep.MaxFailPercent)
	}
	if !ep.AuditAfterSprint {
		b.WriteString("@no_audit\n")
	}
	b.WriteString("\n")

	for _, s := range ep.Sprints {
		fmt.Fprintf(&b, "@sprint %d\n", s.Number)
		fmt.Fprintf(&b, "@name %s\n", s.Name)
		fmt.Fprintf(&b, "@max_iterations %d\n", s.MaxIterations)
		fmt.Fprintf(&b, "@promise %s\n", s.Promise)
		b.WriteString("@prompt\n")
		b.WriteString(s.Prompt)
		if !strings.HasSuffix(s.Prompt, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("@end\n\n")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("write epic file: create dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write epic file: %w", err)
	}
	return nil
}

// AbbreviatedPrepareOpts configures the single-LLM-call prepare for moderate tasks.
type AbbreviatedPrepareOpts struct {
	ProjectDir   string
	EpicFilename string
	Engine       engine.Engine
	Model        string
	UserPrompt   string
	PlanContent  string
	ExecContent  string
	EffortLevel  epic.EffortLevel
	Mode         prepare.Mode
}

// RunAbbreviatedPrepare runs a single LLM call to generate a 1-2 sprint epic.
// Skips AGENTS.md and verification.md generation.
func RunAbbreviatedPrepare(ctx context.Context, opts AbbreviatedPrepareOpts) error {
	epicFilename := opts.EpicFilename
	if strings.TrimSpace(epicFilename) == "" {
		epicFilename = "epic.md"
	}
	if !strings.Contains(epicFilename, string(filepath.Separator)) {
		epicFilename = filepath.Join(config.FryDir, epicFilename)
	}
	epicPath := filepath.Join(opts.ProjectDir, epicFilename)

	// Load the GENERATE_EPIC.md template for format reference.
	generateEpicContent, err := fs.ReadFile(templates.TemplateFS, "GENERATE_EPIC.md")
	if err != nil {
		return fmt.Errorf("abbreviated prepare: read GENERATE_EPIC template: %w", err)
	}
	epicExampleContent, err := fs.ReadFile(templates.TemplateFS, "epic-example.md")
	if err != nil {
		return fmt.Errorf("abbreviated prepare: read epic-example template: %w", err)
	}

	prompt := buildAbbreviatedPrompt(opts, string(generateEpicContent), string(epicExampleContent))

	frylog.Log("▶ TRIAGE  running abbreviated prepare (1 LLM call)...  engine=%s  model=%s", opts.Engine.Name(), opts.Model)

	beforeMod := textutil.FileModTime(epicPath)
	output, _, runErr := opts.Engine.Run(ctx, prompt, engine.RunOpts{
		WorkDir: opts.ProjectDir,
		Model:   opts.Model,
	})
	if runErr != nil && strings.TrimSpace(output) == "" {
		return fmt.Errorf("abbreviated prepare: engine error: %w", runErr)
	}

	if err := os.MkdirAll(filepath.Dir(epicPath), 0o755); err != nil {
		return fmt.Errorf("abbreviated prepare: create dir: %w", err)
	}
	if err := textutil.ResolveArtifact(epicPath, beforeMod, output); err != nil {
		return fmt.Errorf("abbreviated prepare: write epic: %w", err)
	}

	// Validate the generated epic.
	ep, parseErr := epic.ParseEpic(epicPath)
	if parseErr != nil {
		return fmt.Errorf("abbreviated prepare: generated epic is invalid: %w", parseErr)
	}
	if valErr := epic.ValidateEpic(ep); valErr != nil {
		return fmt.Errorf("abbreviated prepare: generated epic failed validation: %w", valErr)
	}

	frylog.Log("  Generated %s (%d sprint(s)).", epicFilename, len(ep.Sprints))
	return nil
}

func buildAbbreviatedPrompt(opts AbbreviatedPrepareOpts, generateEpicTemplate, epicExample string) string {
	var b strings.Builder

	switch opts.Mode {
	case prepare.ModePlanning:
		b.WriteString("You are a strategic planner generating a concise epic.md for a MODERATE-complexity planning task.\n\n")
	case prepare.ModeWriting:
		b.WriteString("You are a senior content strategist generating a concise epic.md for a MODERATE-complexity writing task.\n\n")
	default:
		b.WriteString("You are a senior software architect generating a concise epic.md for a MODERATE-complexity task.\n\n")
	}
	b.WriteString("## Constraints\n\n")
	b.WriteString("- Generate AT MOST 2 sprints. Prefer 1 sprint if the task can reasonably fit.\n")
	b.WriteString("- Each sprint prompt must be self-contained and actionable.\n")
	b.WriteString("- Use @effort medium unless the user specifies otherwise.\n")
	b.WriteString("- Follow the epic format EXACTLY as shown in the reference below.\n\n")

	if opts.EffortLevel != "" {
		fmt.Fprintf(&b, "- Use @effort %s as specified by the user.\n\n", opts.EffortLevel)
	}

	b.WriteString("## Epic Format Reference\n\n")
	b.WriteString("```\n")
	b.WriteString(epicExample)
	b.WriteString("\n```\n\n")

	b.WriteString("## Generation Instructions (from GENERATE_EPIC.md)\n\n")
	b.WriteString(generateEpicTemplate)
	b.WriteString("\n\n")

	b.WriteString("## Project Inputs\n\n")

	if strings.TrimSpace(opts.PlanContent) != "" {
		b.WriteString("### Build Plan (plans/plan.md)\n\n")
		b.WriteString(opts.PlanContent)
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(opts.ExecContent) != "" {
		b.WriteString("### Executive Context (plans/executive.md)\n\n")
		b.WriteString(opts.ExecContent)
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(opts.UserPrompt) != "" {
		b.WriteString("### User Directive\n\n")
		b.WriteString(opts.UserPrompt)
		b.WriteString("\n\n")
	}

	b.WriteString("## Output\n\n")
	b.WriteString("Write the epic.md file to ")
	b.WriteString(config.FryDir)
	b.WriteString("/epic.md. The file must be a valid epic that can be parsed by fry's epic parser.\n")
	b.WriteString("Remember: AT MOST 2 sprints. Keep it focused and actionable.\n")

	return b.String()
}
