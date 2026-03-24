package triage

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/yevgetman/fry/internal/color"
	"github.com/yevgetman/fry/internal/epic"
)

// ErrTriageDeclined is returned when the user declines the triage classification.
var ErrTriageDeclined = fmt.Errorf("user declined triage classification")

// ConfirmOpts configures the interactive triage confirmation prompt.
type ConfirmOpts struct {
	Decision *TriageDecision
	Stdin    io.Reader
	Stdout   io.Writer
}

// ConfirmResult holds the (possibly adjusted) values from the confirmation prompt.
type ConfirmResult struct {
	Complexity  Complexity
	EffortLevel epic.EffortLevel
	GitStrategy string // "current", "branch", "worktree", or "" (no override)
}

// ConfirmDecision displays the triage classification and asks the user to
// accept, decline, or adjust it. Returns ErrTriageDeclined if the user
// declines or input ends unexpectedly.
func ConfirmDecision(opts ConfirmOpts) (*ConfirmResult, error) {
	stdout := opts.Stdout
	stdin := opts.Stdin
	d := opts.Decision

	DisplayTriageSummary(stdout, d)
	fmt.Fprint(stdout, "Accept this classification? [Y/n/a] (a = adjust) ")

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return nil, ErrTriageDeclined
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))

	switch answer {
	case "", "y", "yes":
		return &ConfirmResult{
			Complexity:  d.Complexity,
			EffortLevel: d.EffortLevel,
		}, nil

	case "a", "adjust":
		return adjustDecision(scanner, stdout, d)

	default:
		return nil, ErrTriageDeclined
	}
}

func adjustDecision(scanner *bufio.Scanner, stdout io.Writer, d *TriageDecision) (*ConfirmResult, error) {
	complexity := d.Complexity
	effortLevel := d.EffortLevel

	// Prompt for difficulty adjustment.
	fmt.Fprintf(stdout, "Difficulty [%s] (simple/moderate/complex, or Enter to keep): ", complexity)
	if !scanner.Scan() {
		return nil, ErrTriageDeclined
	}
	diffInput := strings.TrimSpace(scanner.Text())
	if diffInput != "" {
		parsed, err := parseComplexityInput(diffInput)
		if err != nil {
			fmt.Fprintf(stdout, "Invalid difficulty %q — keeping %s.\n", diffInput, complexity)
		} else {
			complexity = parsed
		}
	}

	// Prompt for effort adjustment.
	effortChoices := "low/medium/high"
	if complexity == ComplexityComplex {
		effortChoices = "low/medium/high/max"
	}
	fmt.Fprintf(stdout, "Effort [%s] (%s, or Enter to keep): ", effortLevel.String(), effortChoices)
	if !scanner.Scan() {
		return nil, ErrTriageDeclined
	}
	effortInput := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if effortInput != "" {
		parsed, parseErr := epic.ParseEffortLevel(effortInput)
		if parseErr != nil {
			fmt.Fprintf(stdout, "Invalid effort %q — keeping %s.\n", effortInput, effortLevel.String())
		} else if parsed == epic.EffortMax && complexity != ComplexityComplex {
			fmt.Fprintf(stdout, "Effort \"max\" is reserved for complex tasks — keeping %s.\n", effortLevel.String())
		} else {
			effortLevel = parsed
		}
	}

	// Final validation: max effort is only valid for complex tasks.
	// This catches the case where the user downgrades complexity from COMPLEX
	// but keeps max effort by pressing Enter (bypassing the typed-input guard).
	if effortLevel == epic.EffortMax && complexity != ComplexityComplex {
		fmt.Fprintf(stdout, "Effort \"max\" is reserved for complex tasks — downgrading to high.\n")
		effortLevel = epic.EffortHigh
	}

	// Prompt for git strategy adjustment.
	recommended := "branch"
	if complexity == ComplexityComplex {
		recommended = "worktree"
	}
	fmt.Fprintf(stdout, "Git strategy [%s] (current/branch/worktree, or Enter to keep): ", recommended)
	var gitStrategy string
	if scanner.Scan() {
		gsInput := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if gsInput != "" {
			switch gsInput {
			case "current", "branch", "worktree":
				gitStrategy = gsInput
			default:
				fmt.Fprintf(stdout, "Invalid git strategy %q — keeping %s.\n", gsInput, recommended)
			}
		}
	}

	return &ConfirmResult{
		Complexity:  complexity,
		EffortLevel: effortLevel,
		GitStrategy: gitStrategy,
	}, nil
}

// DisplayTriageSummary prints the triage classification result to the given writer.
func DisplayTriageSummary(w io.Writer, d *TriageDecision) {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, color.DimText("── Triage classification ───────────────────────────────────────"))
	fmt.Fprintf(w, "%s  %s\n", color.BoldText("Difficulty:"), d.Complexity)
	fmt.Fprintf(w, "%s      %s\n", color.BoldText("Effort:"), d.EffortLevel.String())
	fmt.Fprintf(w, "%s         %s\n", color.BoldText("Git:"), gitStrategyLabel(d.Complexity))
	fmt.Fprintf(w, "%s      %s\n", color.BoldText("Reason:"), d.Reason)
	fmt.Fprintf(w, "%s      %s\n", color.BoldText("Action:"), actionDescription(d.Complexity, d.SprintCount))
	fmt.Fprintln(w, color.DimText("─────────────────────────────────────────────────────────────────"))
}

func gitStrategyLabel(c Complexity) string {
	if c == ComplexityComplex {
		return "worktree (isolated working copy)"
	}
	return "branch (new branch for this build)"
}

func actionDescription(c Complexity, sprintCount int) string {
	switch c {
	case ComplexitySimple:
		return "Build 1-sprint epic programmatically (no LLM prepare)"
	case ComplexityModerate:
		sprints := sprintCount
		if sprints <= 0 {
			sprints = 1
		}
		if sprints > 2 {
			sprints = 2
		}
		return fmt.Sprintf("Build %d-sprint epic programmatically (no LLM prepare)", sprints)
	case ComplexityComplex:
		return "Run full prepare pipeline (3-4 LLM calls)"
	default:
		return "Unknown"
	}
}

func parseComplexityInput(s string) (Complexity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "simple":
		return ComplexitySimple, nil
	case "moderate":
		return ComplexityModerate, nil
	case "complex":
		return ComplexityComplex, nil
	default:
		return "", fmt.Errorf("invalid complexity %q: must be simple, moderate, or complex", s)
	}
}
