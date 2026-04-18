package chat

import (
	_ "embed"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/yevgetman/fry/internal/state"
)

//go:embed systemprompt.md
var systemPromptTmpl string

type templateData struct {
	MissionID    string
	MissionDir   string
	CurrentWake  int
	ElapsedHours string
	Status       string
	SoftDeadline string
	HardDeadline string
}

// Launch spawns an interactive claude session with mission context pre-loaded.
func Launch(missionDir string, m *state.Mission) error {
	now := time.Now().UTC()
	data := templateData{
		MissionID:    m.MissionID,
		MissionDir:   missionDir,
		CurrentWake:  m.CurrentWake,
		ElapsedHours: fmt.Sprintf("%.2f", m.ElapsedHours(now)),
		Status:       string(m.Status),
		SoftDeadline: m.SoftDeadline().Format(time.RFC3339),
		HardDeadline: m.HardDeadlineUTC.Format(time.RFC3339),
	}

	tmpl, err := template.New("sysprompt").Parse(systemPromptTmpl)
	if err != nil {
		return fmt.Errorf("chat: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("chat: render template: %w", err)
	}
	sysPrompt := buf.String()

	fmt.Printf("[fry] Loaded %s | wake %d | elapsed %sh | status %s\n",
		m.MissionID, m.CurrentWake, data.ElapsedHours, m.Status)

	// Append a query entry to supervisor_log so the chat session is auditable.
	_ = AppendSupervisorLog(missionDir, "query", "chat session opened", nil)

	cmd := exec.Command("claude",
		"--add-dir", missionDir,
		"--append-system-prompt", sysPrompt,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
