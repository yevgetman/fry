package consciousness

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
)

// SprintOutcome is a lightweight summary of a sprint result used for
// experience synthesis. Defined here to avoid importing the sprint package.
type SprintOutcome struct {
	Number       int
	Name         string
	Status       string
	HealAttempts int
}

// SummarizeOpts contains parameters for the end-of-build experience summary.
type SummarizeOpts struct {
	ProjectDir    string
	Engine        engine.Engine
	Model         string
	EffortLevel   string
	Record        BuildRecord
	SprintResults []SprintOutcome
	BuildOutcome  string
	Verbose       bool
	Stdout        io.Writer
}

// SummarizeExperience invokes an LLM to synthesize all observer observations
// from a build into a coherent experience summary. Returns the summary text.
// Non-fatal by design — callers should treat errors as warnings.
func SummarizeExperience(ctx context.Context, opts SummarizeOpts) (string, error) {
	if opts.Engine == nil {
		return "", fmt.Errorf("summarize experience: engine is required")
	}

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("summarize experience: %w", ctx.Err())
	default:
	}

	prompt := buildExperiencePrompt(opts)

	// Write prompt file
	promptPath := filepath.Join(opts.ProjectDir, config.ConsciousnessPromptFile)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		return "", fmt.Errorf("summarize experience: create dir: %w", err)
	}
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return "", fmt.Errorf("summarize experience: write prompt: %w", err)
	}

	// Create log file
	buildLogsDir := filepath.Join(opts.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return "", fmt.Errorf("summarize experience: create logs dir: %w", err)
	}
	logPath := filepath.Join(buildLogsDir,
		fmt.Sprintf("consciousness_%s.log", time.Now().Format("20060102_150405")),
	)
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", fmt.Errorf("summarize experience: create log: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	runOpts := engine.RunOpts{
		Model:       opts.Model,
		SessionType: engine.SessionExperienceSummary,
		EffortLevel: opts.EffortLevel,
		WorkDir:     opts.ProjectDir,
	}

	if opts.Verbose {
		stdout := opts.Stdout
		if stdout == nil {
			stdout = os.Stdout
		}
		writer := io.MultiWriter(stdout, logFile)
		runOpts.Stdout = writer
		runOpts.Stderr = writer
	} else {
		runOpts.Stdout = logFile
		runOpts.Stderr = logFile
	}

	invocationPrompt := "Read and execute ALL instructions in " + config.ConsciousnessPromptFile + ". Do not create or modify any files."
	output, _, runErr := opts.Engine.Run(ctx, invocationPrompt, runOpts)
	if runErr != nil {
		if strings.TrimSpace(output) == "" {
			return "", fmt.Errorf("summarize experience: %w", runErr)
		}
		return "", fmt.Errorf("summarize experience: engine error with partial output: %w", runErr)
	}

	// Cleanup prompt file
	_ = os.Remove(promptPath)

	summary := strings.TrimSpace(output)
	if summary == "" {
		return "", fmt.Errorf("summarize experience: agent produced empty output")
	}

	return summary, nil
}

func buildExperiencePrompt(opts SummarizeOpts) string {
	var b strings.Builder

	b.WriteString("# Experience Synthesis\n\n")
	b.WriteString("You are synthesizing a build's observer observations into a coherent experience summary.\n")
	b.WriteString("Your output will be stored as part of Fry's memory pipeline and may eventually contribute to Fry's evolving identity.\n\n")

	// Build metadata
	b.WriteString("## Build Metadata\n\n")
	b.WriteString(fmt.Sprintf("- **Engine:** %s\n", opts.Record.Engine))
	b.WriteString(fmt.Sprintf("- **Effort:** %s\n", opts.Record.EffortLevel))
	b.WriteString(fmt.Sprintf("- **Total Sprints:** %d\n", opts.Record.TotalSprints))
	b.WriteString(fmt.Sprintf("- **Outcome:** %s\n\n", opts.BuildOutcome))

	// Sprint results
	if len(opts.SprintResults) > 0 {
		b.WriteString("## Sprint Results\n\n")
		b.WriteString("| Sprint | Name | Status | Alignment Attempts |\n")
		b.WriteString("|--------|------|--------|--------------------|\n")
		for _, r := range opts.SprintResults {
			b.WriteString(fmt.Sprintf("| %d | %s | %s | %d |\n", r.Number, r.Name, r.Status, r.HealAttempts))
		}
		b.WriteString("\n")
	}

	// Observer observations
	b.WriteString("## Observer Observations\n\n")
	if len(opts.Record.Observations) == 0 {
		b.WriteString("(No observations were recorded during this build.)\n\n")
	} else {
		for i, obs := range opts.Record.Observations {
			b.WriteString(fmt.Sprintf("### Observation %d — %s (Sprint %d)\n\n", i+1, obs.WakePoint, obs.SprintNum))
			b.WriteString(obs.Thoughts)
			b.WriteString("\n\n")
		}
	}

	// Synthesis instructions
	b.WriteString("## Instructions\n\n")
	b.WriteString("Synthesize the observations above into a cohesive experience summary (200-500 words) that captures:\n\n")
	b.WriteString("1. **What happened** — the narrative arc of this build\n")
	b.WriteString("2. **What was surprising** — unexpected behaviors, failures, or successes\n")
	b.WriteString("3. **What struggled** — alignment loops, audit cycling, sanity check failures, and their causes\n")
	b.WriteString("4. **Process-level insights** — observations about how the build system performed, not what was built\n")
	b.WriteString("5. **Generalizable lessons** — wisdom that would apply to future builds of any kind\n\n")
	b.WriteString("**Important constraints:**\n")
	b.WriteString("- Write in general terms. Do NOT include project-specific file paths, variable names, class names, or proprietary code.\n")
	b.WriteString("- Focus on the build process and patterns, not the specific software being built.\n")
	b.WriteString("- Write as a narrative, not a list of bullet points.\n")
	b.WriteString("- Output ONLY the summary text. No headers, no preamble, no closing remarks.\n")

	return b.String()
}
