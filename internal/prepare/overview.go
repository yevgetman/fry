package prepare

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yevgetman/fry/internal/color"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/textutil"
)

// ErrProjectOverviewDeclined is returned when the user declines the project summary.
var ErrProjectOverviewDeclined = fmt.Errorf("user declined project summary")

// OverviewSummary holds the parsed fields from the AI-generated project summary.
type OverviewSummary struct {
	ProjectType    string
	Goal           string
	ExpectedOutput string
	KeyTopics      string
	EffortEstimate string
}

// OverviewResult holds adjusted values from the project overview confirmation loop.
type OverviewResult struct {
	UserPrompt   string
	EffortLevel  epic.EffortLevel
	EnableReview bool
}

func runProjectOverview(ctx context.Context, eng engine.Engine, opts PrepareOpts,
	planContent, executiveContent, mediaManifest, assetsSection string) (*OverviewResult, error) {

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	userPrompt := opts.UserPrompt
	effortLevel := opts.EffortLevel
	enableReview := opts.EnableReview
	scanner := bufio.NewScanner(stdin)

	// scanLine reads a line from the scanner and returns the trimmed text.
	// Returns ErrProjectOverviewDeclined on EOF or wraps the scanner error.
	scanLine := func() (string, error) {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("run prepare: read input: %w", err)
			}
			return "", ErrProjectOverviewDeclined
		}
		return strings.TrimSpace(scanner.Text()), nil
	}

	for {
		overviewModel := engine.ResolveModelForSession(eng.Name(), string(effortLevel), engine.SessionProjectOverview)
		frylog.Log("Project overview: summarizing project (engine: %s, model: %s)...", eng.Name(), overviewModel)

		prompt := overviewPrompt(opts.Mode, planContent, executiveContent, userPrompt, effortLevel, mediaManifest, assetsSection)
		output, _, err := eng.Run(ctx, prompt, engine.RunOpts{WorkDir: opts.ProjectDir, Model: overviewModel})
		if err != nil && strings.TrimSpace(output) == "" {
			return nil, fmt.Errorf("run prepare: project overview: %w", err)
		}

		summary := parseOverviewSummary(output)
		displayOverviewSummary(stdout, summary)

		if opts.AutoAccept {
			fmt.Fprintln(stdout, "Does this look right? [Y/n/a] (a = adjust) Y (auto-accepted)")
			return &OverviewResult{UserPrompt: userPrompt, EffortLevel: effortLevel, EnableReview: enableReview}, nil
		}

		fmt.Fprint(stdout, "Does this look right? [Y/n/a] (a = adjust) ")

		answer, err := scanLine()
		if err != nil {
			return nil, err
		}
		answer = strings.ToLower(answer)

		if answer == "" || answer == "y" || answer == "yes" {
			return &OverviewResult{UserPrompt: userPrompt, EffortLevel: effortLevel, EnableReview: enableReview}, nil
		}

		if answer == "a" || answer == "adjust" {
			fmt.Fprint(stdout, "\nPrompt adjustment (describe any change, or leave blank to skip): ")
			adjustment, err := scanLine()
			if err != nil {
				return nil, err
			}
			if adjustment != "" {
				if strings.TrimSpace(userPrompt) == "" {
					userPrompt = adjustment
				} else {
					userPrompt = userPrompt + "\n\n" + adjustment
				}
			}

			fmt.Fprintf(stdout, "Effort level [%s] (low/medium/high/max, or Enter to keep): ", effortLevel.String())
			effortInput, err := scanLine()
			if err != nil {
				return nil, err
			}
			effortInput = strings.ToLower(effortInput)
			if effortInput != "" {
				parsed, parseErr := epic.ParseEffortLevel(effortInput)
				if parseErr != nil {
					fmt.Fprintf(stdout, "Invalid effort level %q — keeping %s.\n", effortInput, effortLevel.String())
				} else {
					effortLevel = parsed
				}
			}

			// Offer sprint review toggle for non-low effort levels.
			// Max effort auto-enables review and shows a confirmation message.
			// Medium/high/auto get an interactive toggle.
			effectiveEffort := effortLevel
			if effectiveEffort == "" {
				effectiveEffort = epic.EffortMedium // auto-detect defaults to at least medium
			}
			if effectiveEffort == epic.EffortMax {
				enableReview = true
				fmt.Fprintf(stdout, "Sprint review: %s (auto-enabled for max effort)\n", color.GreenText("enabled"))
			} else if effectiveEffort != epic.EffortLow {
				reviewDefault := "n"
				if enableReview {
					reviewDefault = "y"
				}
				fmt.Fprintf(stdout, "Enable sprint review? [%s] (y/n, or Enter to keep): ", reviewDefault)
				reviewInput, err := scanLine()
				if err != nil {
					return nil, err
				}
				switch strings.ToLower(reviewInput) {
				case "y", "yes":
					enableReview = true
				case "n", "no":
					enableReview = false
				}
			}

			frylog.Log("Regenerating project summary with adjustments...")
			continue
		}

		return nil, ErrProjectOverviewDeclined
	}
}

func overviewPrompt(mode Mode, planContent, executiveContent, userPrompt string, effort epic.EffortLevel, mediaManifest, assetsSection string) string {
	switch mode {
	case ModePlanning:
		return PlanningOverviewPrompt(planContent, executiveContent, userPrompt, effort, mediaManifest, assetsSection)
	case ModeWriting:
		return WritingOverviewPrompt(planContent, executiveContent, userPrompt, effort, mediaManifest, assetsSection)
	default:
		return SoftwareOverviewPrompt(planContent, executiveContent, userPrompt, effort, mediaManifest, assetsSection)
	}
}

func parseOverviewSummary(output string) OverviewSummary {
	cleaned := textutil.StripMarkdownFences(output)
	lines := strings.Split(cleaned, "\n")
	var s OverviewSummary
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "PROJECT_TYPE:"):
			s.ProjectType = strings.TrimSpace(strings.TrimPrefix(trimmed, "PROJECT_TYPE:"))
		case strings.HasPrefix(trimmed, "GOAL:"):
			s.Goal = strings.TrimSpace(strings.TrimPrefix(trimmed, "GOAL:"))
		case strings.HasPrefix(trimmed, "EXPECTED_OUTPUT:"):
			s.ExpectedOutput = strings.TrimSpace(strings.TrimPrefix(trimmed, "EXPECTED_OUTPUT:"))
		case strings.HasPrefix(trimmed, "KEY_TOPICS:"):
			s.KeyTopics = strings.TrimSpace(strings.TrimPrefix(trimmed, "KEY_TOPICS:"))
		case strings.HasPrefix(trimmed, "EFFORT:"):
			s.EffortEstimate = strings.TrimSpace(strings.TrimPrefix(trimmed, "EFFORT:"))
		}
	}
	return s
}

func displayOverviewSummary(w io.Writer, s OverviewSummary) {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, color.CyanText("── Project summary ─────────────────────────────────────────────"))
	fmt.Fprintf(w, "%s    %s\n", color.CyanText("Project type:"), fieldOrUnknown(s.ProjectType))
	fmt.Fprintf(w, "%s            %s\n", color.CyanText("Goal:"), fieldOrUnknown(s.Goal))
	fmt.Fprintf(w, "%s %s\n", color.CyanText("Expected output:"), fieldOrUnknown(s.ExpectedOutput))
	fmt.Fprintf(w, "%s      %s\n", color.CyanText("Key topics:"), fieldOrUnknown(s.KeyTopics))
	fmt.Fprintf(w, "%s          %s\n", color.CyanText("Effort:"), fieldOrUnknown(s.EffortEstimate))
	fmt.Fprintln(w, color.CyanText("─────────────────────────────────────────────────────────────────"))
}

func fieldOrUnknown(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(unknown)"
	}
	return v
}

func buildOverviewPrompt(persona, planContent, executiveContent, userPrompt string, effort epic.EffortLevel, mediaManifest, assetsSection string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "You are a %s. Review the following project inputs and produce a concise structured summary.\n\n", persona)
	b.WriteString("Available inputs:\n\n")

	fmt.Fprintf(&b, "Plan (plans/plan.md):\n%s\n\n", planContent)

	if strings.TrimSpace(executiveContent) != "" {
		fmt.Fprintf(&b, "Executive context (plans/executive.md):\n%s\n\n", executiveContent)
	}

	if strings.TrimSpace(userPrompt) != "" {
		fmt.Fprintf(&b, "User directive:\n%s\n\n", userPrompt)
	}

	if strings.TrimSpace(mediaManifest) != "" {
		fmt.Fprintf(&b, "Media assets:\n%s\n\n", mediaManifest)
	}

	if strings.TrimSpace(assetsSection) != "" {
		fmt.Fprintf(&b, "%s\n\n", assetsSection)
	}

	b.WriteString(`Produce EXACTLY this output format — no other text, no markdown fences, no explanations:

PROJECT_TYPE: <type> (<short descriptor>)
GOAL: <1-2 sentence goal>
EXPECTED_OUTPUT: <what the build will produce>
KEY_TOPICS: <comma-separated key components or topics>
EFFORT: <low|medium|high|max> (<N-M> sprints)

Rules:
- Derive everything from the provided content. Do not invent information.
- PROJECT_TYPE must start with one of: Software, Planning, Writing
- GOAL should be specific and actionable
- EXPECTED_OUTPUT should describe concrete deliverables
- KEY_TOPICS should list 3-7 items
- EFFORT must express scope as a sprint count — NEVER use hours or days. Use the format "level (N-M sprints)". Sprint ranges by level: low = 1-2, medium = 2-4, high = 4-10, max = 4-10.
`)

	if effort != "" {
		fmt.Fprintf(&b, "- The user has specified effort level %q. Use this for the EFFORT line.\n", effort)
	}

	b.WriteString("- Do NOT write any files. Output ONLY the structured summary.\n")

	return b.String()
}
