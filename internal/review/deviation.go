package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/epic"
)

type DeviationLogEntry struct {
	SprintNum       int
	SprintName      string
	Verdict         ReviewVerdict
	Trigger         string
	Impact          string
	RiskAssessment  string
	AffectedSprints []int
	Timestamp       time.Time
}

type DeviationSummary struct {
	TotalSprints      int
	ReviewsConducted  int
	DeviationsApplied int
	Retries           int
	AllLowRisk        bool
}

func AppendDeviationLog(projectDir string, entry DeviationLogEntry) error {
	path := filepath.Join(projectDir, config.DeviationLogFile)
	if err := ensureDeviationLog(projectDir, path); err != nil {
		return err
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("append deviation log: %w", err)
	}
	defer f.Close()

	decision := string(entry.Verdict)
	if decision == "" {
		decision = string(VerdictContinue)
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("## Review after Sprint %d: %s (%s)\n", entry.SprintNum, entry.SprintName, decision))
	b.WriteString(fmt.Sprintf("- **Reviewed**: %s\n", entry.Timestamp.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("- **Decision**: %s\n", decision))

	if entry.Verdict == VerdictContinue {
		rationale := strings.TrimSpace(entry.Impact)
		if rationale == "" {
			rationale = "Sprint completed as planned."
		}
		b.WriteString(fmt.Sprintf("- **Rationale**: %s\n", rationale))
	} else {
		if entry.Trigger != "" {
			b.WriteString(fmt.Sprintf("- **Trigger**: %s\n", entry.Trigger))
		}
		if len(entry.AffectedSprints) > 0 {
			b.WriteString(fmt.Sprintf("- **Affected sprints**: %s\n", joinInts(entry.AffectedSprints)))
		}
		if entry.Impact != "" {
			for _, line := range strings.Split(strings.TrimSpace(entry.Impact), "\n") {
				b.WriteString(line)
				b.WriteByte('\n')
			}
		}
		if entry.RiskAssessment != "" {
			b.WriteString(fmt.Sprintf("- **Risk assessment**: %s\n", entry.RiskAssessment))
		}
	}

	_, err = f.WriteString(b.String())
	return err
}

func AppendDeviationSummary(projectDir string, summary DeviationSummary) error {
	path := filepath.Join(projectDir, config.DeviationLogFile)
	if err := ensureDeviationLog(projectDir, path); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("append deviation summary: %w", err)
	}
	defer f.Close()

	allLowRisk := "No"
	if summary.AllLowRisk {
		allLowRisk = "Yes"
	}

	_, err = fmt.Fprintf(f, "\n## Build Summary\n- **Total sprints**: %d\n- **Reviews conducted**: %d\n- **Deviations applied**: %d\n- **Retries**: %d\n- **All deviations low risk**: %s\n",
		summary.TotalSprints,
		summary.ReviewsConducted,
		summary.DeviationsApplied,
		summary.Retries,
		allLowRisk,
	)
	return err
}

func ReadDeviationLog(projectDir string) (string, error) {
	return readOptional(filepath.Join(projectDir, config.DeviationLogFile))
}

func ensureDeviationLog(projectDir, path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	title := "Fry Build"
	epicPath := filepath.Join(projectDir, config.FryDir, "epic.md")
	if ep, err := epic.ParseEpic(epicPath); err == nil && strings.TrimSpace(ep.Name) != "" {
		title = ep.Name
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("# Deviation Log — %s\n", title)), 0o644)
}

func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%d", value))
	}
	return strings.Join(parts, ", ")
}
