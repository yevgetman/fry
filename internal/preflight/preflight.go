package preflight

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yevgetman/fry/internal/config"
	frylog "github.com/yevgetman/fry/internal/log"
)

type PreflightConfig struct {
	ProjectDir       string
	Engine           string
	DockerFromSprint int
	CurrentSprint    int
	RequiredTools    []string
	PreflightCmds    []string
}

var (
	lookPath    = exec.LookPath
	execCommand = exec.Command
)

func RunPreflight(cfg PreflightConfig) error {
	if cfg.ProjectDir == "" {
		return fmt.Errorf("preflight: project dir is required")
	}

	engineName := strings.TrimSpace(cfg.Engine)
	if engineName == "" {
		engineName = config.DefaultEngine
	}

	if _, err := lookPath(engineName); err != nil {
		return fmt.Errorf("preflight: engine CLI %q not found on PATH", engineName)
	}
	if _, err := lookPath("git"); err != nil {
		return fmt.Errorf("preflight: git not found on PATH")
	}
	if _, err := lookPath("bash"); err != nil {
		return fmt.Errorf("preflight: bash not found on PATH")
	}

	plansDir := filepath.Join(cfg.ProjectDir, config.PlansDir)
	if info, err := os.Stat(plansDir); err != nil || !info.IsDir() {
		return fmt.Errorf("preflight: plans directory missing: %s", plansDir)
	}

	planPath := filepath.Join(cfg.ProjectDir, config.PlanFile)
	execPath := filepath.Join(cfg.ProjectDir, config.ExecutiveFile)
	if !exists(planPath) && !exists(execPath) {
		return fmt.Errorf("preflight: missing both %s and %s", config.PlanFile, config.ExecutiveFile)
	}

	agentsPath := filepath.Join(cfg.ProjectDir, config.AgentsFile)
	if exists(agentsPath) {
		if err := validateAgentsFile(agentsPath); err != nil {
			return err
		}
	}

	if cfg.DockerFromSprint > 0 && cfg.CurrentSprint >= cfg.DockerFromSprint {
		if _, err := lookPath("docker"); err != nil {
			return fmt.Errorf("preflight: docker not found on PATH")
		}
	}

	for _, tool := range cfg.RequiredTools {
		if _, err := lookPath(tool); err != nil {
			return fmt.Errorf("preflight: required tool %q not found on PATH", tool)
		}
	}

	for _, command := range cfg.PreflightCmds {
		cmd := execCommand("bash", "-c", command)
		cmd.Dir = cfg.ProjectDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("preflight: command failed %q: %s", command, strings.TrimSpace(string(output)))
		}
	}

	if freeBytes, err := freeDiskBytes(cfg.ProjectDir); err == nil && freeBytes < 2*1024*1024*1024 {
		frylog.Log("WARNING: Less than 2GB free disk space available.")
	}

	frylog.Log("Preflight checks passed.")
	return nil
}

func validateAgentsFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("preflight: open AGENTS.md: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	firstLine := ""
	for scanner.Scan() {
		lines++
		if lines == 1 {
			firstLine = scanner.Text()
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("preflight: read AGENTS.md: %w", err)
	}
	if firstLine == "# AGENTS.md — PLACEHOLDER" {
		return fmt.Errorf("preflight: %s is still the placeholder", config.AgentsFile)
	}
	if lines < 5 {
		return fmt.Errorf("preflight: %s must have at least 5 lines", config.AgentsFile)
	}
	return nil
}

func freeDiskBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
