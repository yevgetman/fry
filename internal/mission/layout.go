package mission

import (
	"embed"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/yevgetman/fry/internal/state"
)

//go:embed templates/*
var templateFS embed.FS

// NewOptions carries all flags from `fry new`.
type NewOptions struct {
	Name     string
	BaseDir  string
	Effort   string
	Interval time.Duration
	Duration time.Duration
	Overtime time.Duration

	// Exactly one of these must be set:
	PromptFile string
	PlanFile   string
	SpecDir    string
}

func (o NewOptions) InputMode() string {
	switch {
	case o.PromptFile != "" && o.PlanFile != "":
		return "prompt+plan"
	case o.PromptFile != "":
		return "prompt"
	case o.PlanFile != "":
		return "plan"
	case o.SpecDir != "":
		return "spec-dir"
	default:
		return ""
	}
}

func (o NewOptions) validate() error {
	if o.Name == "" {
		return fmt.Errorf("mission name is required")
	}
	mode := o.InputMode()
	if mode == "" {
		return fmt.Errorf("exactly one of --prompt, --plan, or --spec-dir is required")
	}
	switch o.Effort {
	case "fast", "standard", "max":
	default:
		return fmt.Errorf("--effort must be fast, standard, or max; got %q", o.Effort)
	}
	if o.Interval <= 0 {
		return fmt.Errorf("--interval must be positive")
	}
	if o.Duration <= 0 {
		return fmt.Errorf("--duration must be positive")
	}
	return nil
}

// templateData is passed into all templates.
type templateData struct {
	MissionID       string
	MissionDir      string
	IntervalSeconds int
	Home            string
	User            string
	Path            string
	FryBinaryPath   string
}

// Scaffold creates a new mission directory tree.
// Returns the mission directory path on success.
func Scaffold(o NewOptions) (string, error) {
	if err := o.validate(); err != nil {
		return "", err
	}

	baseDir := o.BaseDir
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home dir: %w", err)
		}
		baseDir = filepath.Join(home, "missions")
	}

	missionDir := filepath.Join(baseDir, o.Name)
	if _, err := os.Stat(missionDir); err == nil {
		return "", fmt.Errorf("mission %q already exists at %s", o.Name, missionDir)
	}

	// Create directory tree
	for _, sub := range []string{"artifacts", "lock", "logs"} {
		if err := os.MkdirAll(filepath.Join(missionDir, sub), 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}

	// Copy input files
	promptPath := ""
	planPath := ""
	switch o.InputMode() {
	case "prompt":
		dst := filepath.Join(missionDir, "prompt.md")
		if err := copyFile(o.PromptFile, dst); err != nil {
			return "", err
		}
		promptPath = dst
	case "plan":
		dst := filepath.Join(missionDir, "plan.md")
		if err := copyFile(o.PlanFile, dst); err != nil {
			return "", err
		}
		planPath = dst
	case "prompt+plan":
		pdst := filepath.Join(missionDir, "prompt.md")
		if err := copyFile(o.PromptFile, pdst); err != nil {
			return "", err
		}
		ldst := filepath.Join(missionDir, "plan.md")
		if err := copyFile(o.PlanFile, ldst); err != nil {
			return "", err
		}
		promptPath = pdst
		planPath = ldst
	case "spec-dir":
		if err := copyDir(o.SpecDir, missionDir); err != nil {
			return "", err
		}
	}

	// Render notes.md from template
	td := buildTemplateData(o, missionDir)
	if err := renderTemplate("templates/notes.md.tmpl", filepath.Join(missionDir, "notes.md"), td); err != nil {
		return "", err
	}

	// Render runner.sh
	runnerPath := filepath.Join(missionDir, "runner.sh")
	if err := renderTemplate("templates/runner.sh.tmpl", runnerPath, td); err != nil {
		return "", err
	}
	if err := os.Chmod(runnerPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod runner.sh: %w", err)
	}

	// Render launchagent.plist
	plistPath := filepath.Join(missionDir, "scheduler.plist")
	if err := renderTemplate("templates/launchagent.plist.tmpl", plistPath, td); err != nil {
		return "", err
	}

	// Touch wake_log.jsonl and supervisor_log.jsonl
	for _, f := range []string{"wake_log.jsonl", "supervisor_log.jsonl"} {
		p := filepath.Join(missionDir, f)
		fh, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", fmt.Errorf("touch %s: %w", f, err)
		}
		fh.Close()
	}

	// Build and write state.json
	now := time.Now().UTC()
	intervalSec := int(o.Interval.Seconds())
	softDeadline := now.Add(o.Duration)
	hardDeadline := softDeadline.Add(o.Overtime)

	m := &state.Mission{
		MissionID:       o.Name,
		CreatedAt:       now,
		PromptPath:      promptPath,
		PlanPath:        planPath,
		SpecDir:         o.SpecDir,
		InputMode:       o.InputMode(),
		Effort:          o.Effort,
		IntervalSeconds: intervalSec,
		DurationHours:   math.Round(o.Duration.Hours()*1000) / 1000,
		OvertimeHours:   math.Round(o.Overtime.Hours()*1000) / 1000,
		CurrentWake:     0,
		Status:          state.StatusActive,
		HardDeadlineUTC: hardDeadline,
	}
	if err := m.Save(missionDir); err != nil {
		return "", err
	}

	return missionDir, nil
}

func buildTemplateData(o NewOptions, missionDir string) templateData {
	home, _ := os.UserHomeDir()
	userName := os.Getenv("USER")
	if userName == "" {
		userName = os.Getenv("LOGNAME")
	}
	path := os.Getenv("PATH")

	// Try to find fry binary: prefer go/bin, fall back to /usr/local/bin
	fryBin := filepath.Join(home, "go", "bin", "fry")
	if _, err := os.Stat(fryBin); err != nil {
		fryBin = "/usr/local/bin/fry"
	}

	return templateData{
		MissionID:       o.Name,
		MissionDir:      missionDir,
		IntervalSeconds: int(o.Interval.Seconds()),
		Home:            home,
		User:            userName,
		Path:            path,
		FryBinaryPath:   fryBin,
	}
}

func renderTemplate(tmplName, dst string, data templateData) error {
	tmplBytes, err := templateFS.ReadFile(tmplName)
	if err != nil {
		return fmt.Errorf("read template %s: %w", tmplName, err)
	}
	tmpl, err := template.New(tmplName).Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", tmplName, err)
	}
	fh, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer fh.Close()
	if err := tmpl.Execute(fh, data); err != nil {
		return fmt.Errorf("execute template %s: %w", tmplName, err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s→%s: %w", src, dst, err)
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, src)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}
