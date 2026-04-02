package review

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/yevgetman/fry/internal/textutil"
)

var (
	deviationSectionHeaderRe = regexp.MustCompile(`(?m)^## Review after Sprint (\d+):`)
	deviationDecisionRe      = regexp.MustCompile(`(?m)^\- \*\*Decision\*\*:\s*(\w+)\s*$`)
	deviationAffectedRe      = regexp.MustCompile(`(?m)^\- \*\*Affected sprints\*\*:\s*(.+)\s*$`)
	deviationTriggerRe       = regexp.MustCompile(`(?m)^\- \*\*Trigger\*\*:\s*(.+)\s*$`)
	deviationRiskRe          = regexp.MustCompile(`(?m)^\- \*\*Risk assessment\*\*:\s*(.+)\s*$`)
)

type promptDeviationEntry struct {
	SprintNum       int
	Content         string
	Verdict         string
	AffectedSprints []int
	Summary         string
}

// LoadRelevantDeviations returns raw markdown deviation entries relevant to the given sprint.
func LoadRelevantDeviations(projectDir string, sprintNum int, maxBytes int) string {
	entries := loadDeviationPromptEntries(projectDir, sprintNum)
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	for i, entry := range entries {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(strings.TrimSpace(entry.Content))
	}

	rendered := b.String()
	if maxBytes > 0 && len(rendered) > maxBytes {
		rendered = textutil.TruncateUTF8(rendered, maxBytes) + "\n...(relevant deviations truncated)\n"
	}
	return rendered
}

// LoadActiveDeviationGuidance returns compact bullet guidance for the upcoming sprint prompt.
func LoadActiveDeviationGuidance(projectDir string, sprintNum int, maxBytes int) string {
	entries := loadDeviationPromptEntries(projectDir, sprintNum)
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	for _, entry := range entries {
		summary := strings.TrimSpace(entry.Summary)
		if summary == "" {
			summary = "Preserve this intentional divergence unless the sprint explicitly revises it."
		}
		b.WriteString("- ")
		b.WriteString(summary)
		b.WriteString(" Preserve the divergence unless this sprint explicitly revises it, and add a brief reconciliation note where it surfaces.\n")
	}

	rendered := strings.TrimSpace(b.String())
	if maxBytes > 0 && len(rendered) > maxBytes {
		rendered = textutil.TruncateUTF8(rendered, maxBytes) + "\n...(deviation guidance truncated)"
	}
	return rendered
}

func loadDeviationPromptEntries(projectDir string, sprintNum int) []promptDeviationEntry {
	content, err := ReadDeviationLog(projectDir)
	if err != nil || strings.TrimSpace(content) == "" {
		return nil
	}

	allEntries := parseDeviationPromptEntries(content)
	var relevant []promptDeviationEntry
	for _, entry := range allEntries {
		if !strings.EqualFold(entry.Verdict, string(VerdictDeviate)) {
			continue
		}
		if entry.SprintNum == sprintNum || containsInt(entry.AffectedSprints, sprintNum) {
			relevant = append(relevant, entry)
		}
	}
	return relevant
}

func parseDeviationPromptEntries(content string) []promptDeviationEntry {
	matches := deviationSectionHeaderRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var entries []promptDeviationEntry
	for i, match := range matches {
		start := match[0]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := strings.TrimSpace(content[start:end])
		sprintNum, _ := strconv.Atoi(content[match[2]:match[3]])
		entry := promptDeviationEntry{
			SprintNum: sprintNum,
			Content:   section,
		}
		if decision := firstSubmatch(deviationDecisionRe, section); decision != "" {
			entry.Verdict = decision
		}
		if affected := firstSubmatch(deviationAffectedRe, section); affected != "" {
			entry.AffectedSprints = parseCommaSeparatedInts(affected)
		}
		entry.Summary = deviationSummary(section)
		entries = append(entries, entry)
	}
	return entries
}

func deviationSummary(section string) string {
	if trigger := firstSubmatch(deviationTriggerRe, section); trigger != "" {
		return trigger
	}
	lines := strings.Split(section, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "## Review after Sprint") {
			continue
		}
		if strings.HasPrefix(line, "- **Reviewed**:") ||
			strings.HasPrefix(line, "- **Decision**:") ||
			strings.HasPrefix(line, "- **Affected sprints**:") {
			continue
		}
		if strings.HasPrefix(line, "- **Risk assessment**:") {
			if risk := firstSubmatch(deviationRiskRe, section); risk != "" {
				return risk
			}
			continue
		}
		return strings.TrimPrefix(line, "- ")
	}
	return ""
}

func firstSubmatch(re *regexp.Regexp, value string) string {
	match := re.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func parseCommaSeparatedInts(value string) []int {
	parts := strings.Split(value, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
