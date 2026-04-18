package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const notesFile = "notes.md"

// Notes holds the parsed sections of notes.md.
type Notes struct {
	MissionID         string
	CurrentFocus      string
	NextWakeShould    string
	Decisions         []string
	OpenQuestions     []string
	SupervisorInjects []string
}

// Load reads and parses notes.md from missionDir. Returns empty Notes if file absent.
func Load(missionDir string) (*Notes, error) {
	path := filepath.Join(missionDir, notesFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Notes{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notes.Load: %w", err)
	}
	return parse(string(data)), nil
}

// Save writes notes.md atomically via temp-file + rename.
func (n *Notes) Save(missionDir string) error {
	content := n.render()
	path := filepath.Join(missionDir, notesFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("notes.Save: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("notes.Save: rename: %w", err)
	}
	return nil
}

// AppendDecision records a new decision with timestamp and wake number.
func (n *Notes) AppendDecision(wake int, text string) {
	entry := fmt.Sprintf("%sZ (wake %d): %s",
		time.Now().UTC().Format("2006-01-02T15:04"), wake, text)
	n.Decisions = append(n.Decisions, entry)
}

// AppendInjection records a supervisor injection with timestamp.
func (n *Notes) AppendInjection(text string) {
	entry := fmt.Sprintf("%sZ: %s", time.Now().UTC().Format("2006-01-02T15:04"), text)
	n.SupervisorInjects = append(n.SupervisorInjects, entry)
}

// parse splits notes.md into sections by ## headings.
func parse(content string) *Notes {
	n := &Notes{}
	lines := strings.Split(content, "\n")

	currentSection := ""
	var sectionLines []string

	flush := func() {
		text := cleanSection(strings.Join(sectionLines, "\n"))
		switch currentSection {
		case "Current Focus":
			n.CurrentFocus = text
		case "Next Wake Should":
			n.NextWakeShould = text
		case "Decisions":
			n.Decisions = nonEmpty(strings.Split(text, "\n"))
		case "Open Questions":
			n.OpenQuestions = nonEmpty(strings.Split(text, "\n"))
		case "Supervisor Injections":
			n.SupervisorInjects = nonEmpty(strings.Split(text, "\n"))
		}
		sectionLines = nil
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			currentSection = strings.TrimPrefix(line, "## ")
		case strings.HasPrefix(line, "# "):
			n.MissionID = strings.TrimPrefix(line, "# Mission Notes — ")
		default:
			sectionLines = append(sectionLines, line)
		}
	}
	flush()
	return n
}

// render serializes Notes back to the notes.md format.
func (n *Notes) render() string {
	var sb strings.Builder
	id := n.MissionID
	if id == "" {
		id = "unknown"
	}
	fmt.Fprintf(&sb, "# Mission Notes — %s\n\n", id)

	sb.WriteString("## Current Focus\n")
	if n.CurrentFocus != "" {
		sb.WriteString(n.CurrentFocus)
	} else {
		sb.WriteString("<one sentence — what this wake is for>")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Next Wake Should\n")
	if n.NextWakeShould != "" {
		sb.WriteString(n.NextWakeShould)
	} else {
		sb.WriteString("<handoff directive from current wake to next>")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Decisions\n")
	sb.WriteString("<!-- Format: YYYY-MM-DDTHH:MMZ (wake N): <decision> -->\n")
	for _, d := range n.Decisions {
		sb.WriteString(d)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Open Questions\n")
	sb.WriteString("<!-- Add open questions here as they arise -->\n")
	for _, q := range n.OpenQuestions {
		sb.WriteString(q)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Supervisor Injections\n")
	sb.WriteString("<!-- Chat sessions append here; wakes read and honor. -->\n")
	for _, inj := range n.SupervisorInjects {
		sb.WriteString(inj)
		sb.WriteString("\n")
	}
	return sb.String()
}

// cleanSection strips comment lines and trims whitespace from a section body.
func cleanSection(text string) string {
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		lines = append(lines, l)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// nonEmpty returns slice of non-empty trimmed strings.
func nonEmpty(lines []string) []string {
	var out []string
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}
