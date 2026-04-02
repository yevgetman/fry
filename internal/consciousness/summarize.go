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
	"github.com/yevgetman/fry/internal/textutil"
)

// SprintOutcome is a lightweight summary of a sprint result used for experience synthesis.
type SprintOutcome struct {
	Number       int
	Name         string
	Status       string
	HealAttempts int
}

type DistillOpts struct {
	ProjectDir  string
	Engine      engine.Engine
	Model       string
	EffortLevel string
	Record      BuildRecord
	Checkpoint  ObservationCheckpoint
	Verbose     bool
	Stdout      io.Writer
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

// DistillCheckpoint synthesizes a durable checkpoint summary from one persisted checkpoint.
func DistillCheckpoint(ctx context.Context, opts DistillOpts) (CheckpointSummary, error) {
	if opts.Engine == nil {
		return CheckpointSummary{}, fmt.Errorf("distill checkpoint: engine is required")
	}
	if opts.Checkpoint.Sequence == 0 {
		return CheckpointSummary{}, fmt.Errorf("distill checkpoint: checkpoint sequence is required")
	}
	if opts.Checkpoint.Observation == nil && opts.Checkpoint.CheckpointType != CheckpointTypeInterruption {
		return CheckpointSummary{}, fmt.Errorf("distill checkpoint: canonical observation is required")
	}

	prompt := buildCheckpointPrompt(opts)
	output, err := runConsciousnessSession(ctx, consciousnessRunOpts{
		ProjectDir:  opts.ProjectDir,
		PromptPath:  filepath.Join(opts.ProjectDir, config.ConsciousnessCheckpointPromptFile),
		LogPrefix:   fmt.Sprintf("consciousness_checkpoint_%04d", opts.Checkpoint.Sequence),
		Prompt:      prompt,
		Engine:      opts.Engine,
		Model:       opts.Model,
		EffortLevel: opts.EffortLevel,
		Verbose:     opts.Verbose,
		Stdout:      opts.Stdout,
	})
	if err != nil {
		return CheckpointSummary{}, err
	}

	var parsed distillResult
	if _, err := textutil.ExtractJSONWithDiagnostics(output, &parsed); err != nil {
		return CheckpointSummary{}, fmt.Errorf("distill checkpoint: %w", err)
	}
	if strings.TrimSpace(parsed.Summary) == "" {
		return CheckpointSummary{}, fmt.Errorf("distill checkpoint: empty summary")
	}

	return CheckpointSummary{
		SessionID:      opts.Record.SessionID,
		Sequence:       opts.Checkpoint.Sequence,
		CheckpointType: opts.Checkpoint.CheckpointType,
		Timestamp:      time.Now().UTC(),
		Summary:        strings.TrimSpace(parsed.Summary),
		Lessons:        cleanList(parsed.Lessons),
		RiskSignals:    cleanList(parsed.RiskSignals),
	}, nil
}

// SummarizeExperience synthesizes a final build summary from checkpoint summaries only.
func SummarizeExperience(ctx context.Context, opts SummarizeOpts) (string, error) {
	if opts.Engine == nil {
		return "", fmt.Errorf("summarize experience: engine is required")
	}
	if len(opts.Record.CheckpointSummaries) == 0 {
		return "", fmt.Errorf("summarize experience: checkpoint summaries are required")
	}

	prompt := buildExperiencePrompt(opts)
	output, err := runConsciousnessSession(ctx, consciousnessRunOpts{
		ProjectDir:  opts.ProjectDir,
		PromptPath:  filepath.Join(opts.ProjectDir, config.ConsciousnessPromptFile),
		LogPrefix:   "consciousness_final",
		Prompt:      prompt,
		Engine:      opts.Engine,
		Model:       opts.Model,
		EffortLevel: opts.EffortLevel,
		Verbose:     opts.Verbose,
		Stdout:      opts.Stdout,
	})
	if err != nil {
		return "", err
	}

	var parsed summaryResult
	if _, err := textutil.ExtractJSONWithDiagnostics(output, &parsed); err != nil {
		return "", fmt.Errorf("summarize experience: %w", err)
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		return "", fmt.Errorf("summarize experience: empty summary")
	}
	return summary, nil
}

type consciousnessRunOpts struct {
	ProjectDir  string
	PromptPath  string
	LogPrefix   string
	Prompt      string
	Engine      engine.Engine
	Model       string
	EffortLevel string
	Verbose     bool
	Stdout      io.Writer
}

func runConsciousnessSession(ctx context.Context, opts consciousnessRunOpts) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if err := os.MkdirAll(filepath.Dir(opts.PromptPath), 0o755); err != nil {
		return "", fmt.Errorf("consciousness session: create dir: %w", err)
	}
	if err := os.WriteFile(opts.PromptPath, []byte(opts.Prompt), 0o644); err != nil {
		return "", fmt.Errorf("consciousness session: write prompt: %w", err)
	}
	defer func() {
		_ = os.Remove(opts.PromptPath)
	}()

	buildLogsDir := filepath.Join(opts.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return "", fmt.Errorf("consciousness session: create logs dir: %w", err)
	}
	logPath := filepath.Join(buildLogsDir, fmt.Sprintf("%s_%s.log", opts.LogPrefix, time.Now().Format("20060102_150405")))
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", fmt.Errorf("consciousness session: create log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

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

	invocationPrompt := "Read and execute ALL instructions in " + filepath.Base(opts.PromptPath) + ". Do not create or modify any other files."
	output, _, runErr := opts.Engine.Run(ctx, invocationPrompt, runOpts)
	if runErr != nil {
		return "", fmt.Errorf("consciousness session: %w", runErr)
	}
	return strings.TrimSpace(output), nil
}

func buildCheckpointPrompt(opts DistillOpts) string {
	var b strings.Builder
	b.WriteString("# Consciousness Checkpoint Distillation\n\n")
	b.WriteString("Convert the durable checkpoint below into a compact structured summary.\n")
	b.WriteString("Return exactly one JSON object and no prose outside it.\n\n")
	b.WriteString("## Build Metadata\n\n")
	b.WriteString(fmt.Sprintf("- Session: %s\n", opts.Record.SessionID))
	b.WriteString(fmt.Sprintf("- Engine: %s\n", opts.Record.Engine))
	b.WriteString(fmt.Sprintf("- Effort: %s\n", opts.Record.EffortLevel))
	b.WriteString(fmt.Sprintf("- Outcome so far: %s\n\n", strings.TrimSpace(opts.Record.Outcome)))

	b.WriteString("## Checkpoint\n\n")
	b.WriteString(fmt.Sprintf("- Sequence: %d\n", opts.Checkpoint.Sequence))
	b.WriteString(fmt.Sprintf("- Type: %s\n", opts.Checkpoint.CheckpointType))
	b.WriteString(fmt.Sprintf("- Wake point: %s\n", opts.Checkpoint.WakePoint))
	b.WriteString(fmt.Sprintf("- Sprint: %d\n", opts.Checkpoint.SprintNum))
	b.WriteString(fmt.Sprintf("- Parse status: %s\n\n", opts.Checkpoint.ParseStatus))

	if opts.Checkpoint.Observation != nil {
		b.WriteString("### Canonical Observation\n\n")
		b.WriteString(opts.Checkpoint.Observation.Thoughts)
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(opts.Checkpoint.ScratchpadDelta) != "" {
		b.WriteString("### Scratchpad Delta\n\n")
		b.WriteString(opts.Checkpoint.ScratchpadDelta)
		b.WriteString("\n\n")
	}
	if len(opts.Checkpoint.Directives) > 0 {
		b.WriteString("### Directives\n\n")
		for _, directive := range opts.Checkpoint.Directives {
			b.WriteString(fmt.Sprintf("- %s: %s\n", directive.Type, directive.Value))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Output Schema\n\n")
	b.WriteString("```json\n")
	b.WriteString("{\n")
	b.WriteString("  \"summary\": \"One concise narrative paragraph.\",\n")
	b.WriteString("  \"lessons\": [\"Short generalizable lesson\"],\n")
	b.WriteString("  \"risk_signals\": [\"Short risk signal\"]\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")
	b.WriteString("Constraints:\n")
	b.WriteString("- Keep `summary` under 120 words.\n")
	b.WriteString("- `lessons` and `risk_signals` must contain short strings only.\n")
	b.WriteString("- Do not include raw transcripts, file paths, or markdown fences in field values.\n")
	return b.String()
}

func buildExperiencePrompt(opts SummarizeOpts) string {
	var b strings.Builder
	b.WriteString("# Experience Synthesis\n\n")
	b.WriteString("Synthesize the checkpoint summaries below into one final structured build experience.\n")
	b.WriteString("Return exactly one JSON object and no prose outside it.\n\n")
	b.WriteString("## Build Metadata\n\n")
	b.WriteString(fmt.Sprintf("- Engine: %s\n", opts.Record.Engine))
	b.WriteString(fmt.Sprintf("- Effort: %s\n", opts.Record.EffortLevel))
	b.WriteString(fmt.Sprintf("- Total sprints: %d\n", opts.Record.TotalSprints))
	b.WriteString(fmt.Sprintf("- Build outcome: %s\n", opts.BuildOutcome))
	b.WriteString(fmt.Sprintf("- Checkpoints: %d\n", opts.Record.CheckpointCount))
	b.WriteString(fmt.Sprintf("- Parse failures: %d\n\n", opts.Record.ParseFailures))

	if len(opts.SprintResults) > 0 {
		b.WriteString("## Sprint Results\n\n")
		for _, result := range opts.SprintResults {
			b.WriteString(fmt.Sprintf("- Sprint %d (%s): %s, heal attempts=%d\n", result.Number, result.Name, result.Status, result.HealAttempts))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Checkpoint Summaries\n\n")
	for _, summary := range opts.Record.CheckpointSummaries {
		b.WriteString(fmt.Sprintf("### Sequence %d (%s)\n\n", summary.Sequence, summary.CheckpointType))
		b.WriteString(summary.Summary)
		b.WriteString("\n")
		if len(summary.Lessons) > 0 {
			b.WriteString("Lessons: ")
			b.WriteString(strings.Join(summary.Lessons, "; "))
			b.WriteString("\n")
		}
		if len(summary.RiskSignals) > 0 {
			b.WriteString("Risk signals: ")
			b.WriteString(strings.Join(summary.RiskSignals, "; "))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Output Schema\n\n")
	b.WriteString("```json\n")
	b.WriteString("{\n")
	b.WriteString("  \"summary\": \"A 200-500 word narrative summary of the build experience.\"\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")
	b.WriteString("Constraints:\n")
	b.WriteString("- Synthesize only from checkpoint summaries and build metadata above.\n")
	b.WriteString("- Do not quote raw transcripts or raw observer output.\n")
	b.WriteString("- Focus on process-level learning and generalizable lessons.\n")
	b.WriteString("- Do not include markdown headers or bullet lists in `summary`.\n")
	return b.String()
}

func cleanList(values []string) []string {
	var cleaned []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}
