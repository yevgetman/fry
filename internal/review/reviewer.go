package review

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	frylog "github.com/yevgetman/fry/internal/log"
)


type ReviewPromptOpts struct {
	ProjectDir             string
	SprintNum              int
	TotalSprints           int
	SprintName             string
	RemainingSprintPrompts []string
	EpicProgressContent    string
	SprintProgressContent  string
	DeviationLogContent    string
	EffortLevel            epic.EffortLevel
	Mode                   string
}

type RunReviewOpts struct {
	ProjectDir      string
	SprintNum       int
	TotalSprints    int
	SprintName      string
	Epic            *epic.Epic
	Engine          engine.Engine
	SimulateVerdict string
	Verbose         bool
	Mode            string
	Stdout          io.Writer // optional; defaults to os.Stdout when Verbose is true
}

func AssembleReviewPrompt(opts ReviewPromptOpts) (string, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Sprint Review — After Sprint %d: %s\n\n", opts.SprintNum, opts.SprintName))
	b.WriteString("## Your Role\n")
	if opts.Mode == "writing" {
		b.WriteString(fmt.Sprintf("You are a content plan reviewer. You have just observed the completion of Sprint %d.\n", opts.SprintNum))
	} else {
		b.WriteString(fmt.Sprintf("You are a build plan reviewer. You have just observed the completion of Sprint %d.\n", opts.SprintNum))
	}
	b.WriteString(fmt.Sprintf("Your job is to decide whether the remaining sprints (Sprint %d through %d)\n", opts.SprintNum+1, opts.TotalSprints))
	b.WriteString("need adjustment based on what actually happened.\n\n")
	if opts.EffortLevel == epic.EffortMax {
		b.WriteString("## Bias: THOROUGH REVIEW\n")
		b.WriteString("At MAX effort level, apply heightened scrutiny. Recommend DEVIATE when:\n")
		b.WriteString("- Any deviation from the plan that could affect system correctness\n")
		b.WriteString("- Missing error handling or edge case coverage in completed sprint\n")
		b.WriteString("- Performance or security concerns that downstream sprints should account for\n")
		b.WriteString("- A file path, module name, or API shape in a downstream sprint prompt references\n")
		b.WriteString("  something that was built differently than the prompt assumes\n")
		b.WriteString("- A technical decision made during the sprint makes a downstream sprint's approach\n")
		b.WriteString("  infeasible or significantly suboptimal\n")
		b.WriteString("- A dependency assumed by a downstream sprint was not created or was created differently\n\n")
	} else {
		b.WriteString("## Bias: CONTINUE\n")
		b.WriteString("Your default answer is CONTINUE. Only recommend DEVIATE when:\n")
		b.WriteString("- A file path, module name, or API shape in a downstream sprint prompt references\n")
		b.WriteString("  something that was built differently than the prompt assumes\n")
		b.WriteString("- A technical decision made during the sprint makes a downstream sprint's approach\n")
		b.WriteString("  infeasible or significantly suboptimal\n")
		b.WriteString("- A dependency assumed by a downstream sprint was not created or was created differently\n\n")
	}
	b.WriteString("Do NOT recommend DEVIATE for:\n")
	b.WriteString("- Minor style differences (naming conventions, comment style)\n")
	b.WriteString("- Implementation details that don't affect downstream sprints\n")
	b.WriteString("- \"Better\" approaches that would require redoing completed work (unless critical)\n")
	b.WriteString("- Cosmetic or subjective improvements\n\n")
	b.WriteString("## What Was Built (Cumulative)\n")
	b.WriteString(defaultIfEmpty(opts.EpicProgressContent, "(No epic progress recorded yet.)"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("## Detailed Sprint %d Log\n\n", opts.SprintNum))
	b.WriteString(defaultIfEmpty(opts.SprintProgressContent, "(No sprint progress recorded.)"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("## Remaining Plan (Sprints %d through %d)\n\n", opts.SprintNum+1, opts.TotalSprints))
	if len(opts.RemainingSprintPrompts) == 0 {
		b.WriteString("(No remaining sprints.)\n\n")
	} else {
		for _, prompt := range opts.RemainingSprintPrompts {
			b.WriteString(prompt)
			if !strings.HasSuffix(prompt, "\n") {
				b.WriteByte('\n')
			}
			b.WriteByte('\n')
		}
	}
	b.WriteString("## Original Plan\n")
	b.WriteString("Read plans/plan.md for the original human-authored intent.\n\n")
	b.WriteString("## Prior Deviations\n")
	b.WriteString(defaultIfEmpty(opts.DeviationLogContent, "None — this is the first review."))
	b.WriteString("\n\n")
	b.WriteString("## Your Output\n")
	b.WriteString("Respond with EXACTLY this structure:\n\n")
	b.WriteString("### Analysis\n")
	b.WriteString("2-5 sentences: what happened vs. what was planned.\n\n")
	b.WriteString("### Downstream Impact Assessment\n")
	b.WriteString("For each remaining sprint, one line:\n")
	b.WriteString("- Sprint N: NO IMPACT | MINOR — reason | MODERATE — reason | MAJOR — reason\n\n")
	b.WriteString("### Decision\n")
	b.WriteString("<verdict>CONTINUE</verdict> or <verdict>DEVIATE</verdict>\n\n")
	b.WriteString("### If DEVIATE: Deviation Spec\n")
	b.WriteString("Only include this section if your verdict is DEVIATE.\n")
	b.WriteString("- **Trigger**: what specifically in the completed sprint differs from the plan\n")
	b.WriteString("- **Affected sprints**: list of sprint numbers\n")
	b.WriteString("- **For each affected sprint**:\n")
	b.WriteString("  - Sprint X: what specifically needs to change in the @prompt block\n")
	b.WriteString("- **Risk assessment**: Low | Medium | High — 1 sentence\n")

	prompt := b.String()
	if opts.ProjectDir != "" {
		path := filepath.Join(opts.ProjectDir, config.ReviewPromptFile)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("assemble review prompt: create dir: %w", err)
		}
		if err := os.WriteFile(path, []byte(prompt), 0o644); err != nil {
			return "", fmt.Errorf("assemble review prompt: write file: %w", err)
		}
	}

	return prompt, nil
}

func ParseVerdict(output string) ReviewVerdict {
	switch {
	case strings.Contains(output, "<verdict>DEVIATE</verdict>"):
		return VerdictDeviate
	case strings.Contains(output, "<verdict>CONTINUE</verdict>"):
		return VerdictContinue
	default:
		frylog.Log("WARNING: no <verdict> tag found in review output — defaulting to CONTINUE")
		return VerdictContinue
	}
}

func ExtractDeviationSpec(output string) *DeviationSpec {
	lines := strings.Split(output, "\n")
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "###") && strings.Contains(strings.ToLower(trimmed), "deviation spec") {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return nil
	}

	body := strings.TrimSpace(strings.Join(lines[start:], "\n"))
	if body == "" {
		return nil
	}

	spec := &DeviationSpec{RawText: body}
	var detailLines []string
	numberPattern := regexp.MustCompile(`\d+`)

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "- **trigger**:"):
			spec.Trigger = strings.TrimSpace(afterColon(trimmed))
		case strings.HasPrefix(lower, "- **affected sprints**:"):
			for _, match := range numberPattern.FindAllString(trimmed, -1) {
				n, err := strconv.Atoi(match)
				if err == nil {
					spec.AffectedSprints = append(spec.AffectedSprints, n)
				}
			}
		case strings.HasPrefix(lower, "- **risk assessment**:"):
			spec.RiskAssessment = strings.TrimSpace(afterColon(trimmed))
		default:
			detailLines = append(detailLines, trimmed)
		}
	}

	spec.Details = strings.TrimSpace(strings.Join(detailLines, "\n"))
	if spec.Trigger == "" && spec.RiskAssessment == "" && len(spec.AffectedSprints) == 0 && spec.Details == "" {
		return nil
	}
	return spec
}

func RunSprintReview(ctx context.Context, opts RunReviewOpts) (*ReviewResult, error) {
	if opts.ProjectDir == "" {
		return nil, fmt.Errorf("run sprint review: project dir is required")
	}

	effortStr := ""
	if opts.Epic != nil {
		effortStr = string(opts.Epic.EffortLevel)
	}
	reviewOverride := ""
	if opts.Epic != nil {
		reviewOverride = opts.Epic.ReviewModel
	}

	frylog.Log("  --- SPRINT REVIEW: after Sprint %d ---", opts.SprintNum)

	promptOpts, err := buildReviewPromptOpts(opts)
	if err != nil {
		return nil, fmt.Errorf("run sprint review: %w", err)
	}
	if _, err := AssembleReviewPrompt(promptOpts); err != nil {
		return nil, fmt.Errorf("run sprint review: %w", err)
	}

	if verdict := strings.ToUpper(strings.TrimSpace(opts.SimulateVerdict)); verdict != "" {
		frylog.Log("  SIMULATION MODE: injecting %s verdict", verdict)
		output, err := simulatedReviewOutput(verdict, opts.SprintNum, opts.TotalSprints)
		if err != nil {
			return nil, fmt.Errorf("run sprint review: %w", err)
		}
		result := &ReviewResult{Verdict: ParseVerdict(output), RawOutput: output}
		if result.Verdict == VerdictDeviate {
			result.Deviation = ExtractDeviationSpec(output)
		}
		frylog.Log("  Review verdict: %s", result.Verdict)
		return result, nil
	}

	if opts.Engine == nil {
		return nil, fmt.Errorf("run sprint review: engine is required")
	}

	resolvedModel := engine.ResolveModel(reviewOverride, opts.Engine.Name(), effortStr, engine.SessionReview)
	frylog.Log("  REVIEW  engine=%s  model=%s", opts.Engine.Name(), resolvedModel)

	runOpts := engine.RunOpts{
		Model:   resolvedModel,
		WorkDir: opts.ProjectDir,
	}
	if opts.Verbose {
		stdout := opts.Stdout
		if stdout == nil {
			stdout = os.Stdout
		}
		runOpts.Stdout = stdout
		runOpts.Stderr = stdout
	}
	reviewRole := "build plan reviewer"
	if opts.Mode == "writing" {
		reviewRole = "content plan reviewer"
	}
	output, _, runErr := opts.Engine.Run(ctx,
		fmt.Sprintf("Read and execute ALL instructions in .fry/review-prompt.md. You are a %s evaluating whether downstream sprints need adjustment.", reviewRole),
		runOpts,
	)
	if runErr != nil && strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("run sprint review: reviewer agent failed: %w", runErr)
	}
	if runErr != nil {
		frylog.Log("WARNING: reviewer agent exited with error (non-fatal): %v", runErr)
	}

	reviewTS := time.Now().Format("20060102_150405")
	reviewLogPath := filepath.Join(opts.ProjectDir, config.BuildLogsDir, fmt.Sprintf(config.SprintReviewLogPattern, opts.SprintNum, reviewTS))
	if err := os.MkdirAll(filepath.Dir(reviewLogPath), 0o755); err != nil {
		frylog.Log("WARNING: could not create review log dir: %v", err)
	} else if err := os.WriteFile(reviewLogPath, []byte(output), 0o644); err != nil {
		frylog.Log("WARNING: could not write review log: %v", err)
	}

	result := &ReviewResult{
		Verdict:   ParseVerdict(output),
		RawOutput: output,
	}
	if result.Verdict == VerdictDeviate {
		result.Deviation = ExtractDeviationSpec(output)
	}
	frylog.Log("  Review verdict: %s", result.Verdict)
	return result, nil
}

func buildReviewPromptOpts(opts RunReviewOpts) (ReviewPromptOpts, error) {
	epicProgress, err := readOptional(filepath.Join(opts.ProjectDir, config.EpicProgressFile))
	if err != nil {
		return ReviewPromptOpts{}, err
	}
	sprintProgress, err := readOptional(filepath.Join(opts.ProjectDir, config.SprintProgressFile))
	if err != nil {
		return ReviewPromptOpts{}, err
	}
	deviationLog, err := readOptional(filepath.Join(opts.ProjectDir, config.DeviationLogFile))
	if err != nil {
		return ReviewPromptOpts{}, err
	}

	remaining := make([]string, 0)
	if opts.Epic != nil {
		for _, sprint := range opts.Epic.Sprints {
			if sprint.Number <= opts.SprintNum {
				continue
			}
			remaining = append(remaining, fmt.Sprintf("### Sprint %d: %s\n\n%s", sprint.Number, sprint.Name, sprint.Prompt))
		}
	}

	var effortLevel epic.EffortLevel
	if opts.Epic != nil {
		effortLevel = opts.Epic.EffortLevel
	}

	return ReviewPromptOpts{
		ProjectDir:             opts.ProjectDir,
		SprintNum:              opts.SprintNum,
		TotalSprints:           opts.TotalSprints,
		SprintName:             opts.SprintName,
		RemainingSprintPrompts: remaining,
		EpicProgressContent:    epicProgress,
		SprintProgressContent:  sprintProgress,
		DeviationLogContent:    deviationLog,
		EffortLevel:            effortLevel,
		Mode:                   opts.Mode,
	}, nil
}

func simulatedReviewOutput(verdict string, sprintNum, totalSprints int) (string, error) {
	switch verdict {
	case string(VerdictContinue):
		return "### Analysis\nSprint completed as planned.\n\n### Decision\n<verdict>CONTINUE</verdict>\n", nil
	case string(VerdictDeviate):
		affected := sprintNum + 1
		if affected > totalSprints {
			affected = sprintNum
		}
		return fmt.Sprintf("### Analysis\nSprint %d deviated from plan (simulated). Downstream prompts need adjustment.\n\n### Decision\n<verdict>DEVIATE</verdict>\n\n### Deviation Spec\n- **Trigger**: Simulated deviation for testing the replan pipeline\n- **Affected sprints**: %d\n- **Sprint %d**: Add a comment noting this sprint was adjusted by a simulated review\n- **Risk assessment**: Low — simulation only\n", sprintNum, affected, affected), nil
	default:
		return "", fmt.Errorf("unknown simulation verdict %q", verdict)
	}
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimRight(value, "\n")
}

func readOptional(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func afterColon(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}
