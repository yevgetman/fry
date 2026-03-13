package prepare

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/media"
	"github.com/yevgetman/fry/internal/textutil"
	"github.com/yevgetman/fry/templates"
)

type PrepareOpts struct {
	ProjectDir   string
	EpicFilename string
	Engine       string
	UserPrompt   string
	ValidateOnly bool
	Planning     bool
	EffortLevel  epic.EffortLevel
}

var newEngine = engine.NewEngine
var numberedRulePattern = regexp.MustCompile(`(?m)^[0-9]+\.`)

func RunPrepare(ctx context.Context, opts PrepareOpts) error {
	projectDir := opts.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}

	if err := validatePreparePrerequisites(projectDir); err != nil {
		return err
	}
	if opts.ValidateOnly {
		return nil
	}

	engName, err := engine.ResolveEngine(opts.Engine, "", "")
	if err != nil {
		return fmt.Errorf("run prepare: %w", err)
	}
	eng, err := newEngine(engName)
	if err != nil {
		return fmt.Errorf("run prepare: %w", err)
	}

	fryDir := filepath.Join(projectDir, config.FryDir)
	if err := os.MkdirAll(fryDir, 0o755); err != nil {
		return fmt.Errorf("run prepare: create fry dir: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "fry-prepare-*")
	if err != nil {
		return fmt.Errorf("run prepare: create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	epicExamplePath, verificationExamplePath, generateEpicPath, err := writeEmbeddedTemplates(tempDir)
	if err != nil {
		return fmt.Errorf("run prepare: write embedded templates: %w", err)
	}

	if opts.Planning {
		outputDir := filepath.Join(projectDir, config.PlanningOutputDir)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("run prepare: create planning output dir: %w", err)
		}
	}

	// Scan media directory for available assets (optional).
	mediaAssets, mediaErr := media.Scan(projectDir)
	if mediaErr != nil {
		frylog.Log("WARNING: could not scan media directory: %v", mediaErr)
	}
	mediaManifest := media.BuildManifest(mediaAssets)

	planPath := filepath.Join(projectDir, config.PlanFile)
	executivePath := filepath.Join(projectDir, config.ExecutiveFile)

	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		frylog.Log("Step 0: Generating %s from %s (engine: %s)...", config.PlanFile, config.ExecutiveFile, engName)
		executiveContent, err := os.ReadFile(executivePath)
		if err != nil {
			return fmt.Errorf("run prepare: read executive: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
			return err
		}
		prompt := step0Prompt(opts.Planning, string(executiveContent), mediaManifest)
		beforeMod := textutil.FileModTime(planPath)
		output, err := runPrepareStep(ctx, eng, projectDir, prompt)
		if err != nil {
			return err
		}
		if err := textutil.ResolveArtifact(planPath, beforeMod, output); err != nil {
			return err
		}
		if err := validateStep0(planPath); err != nil {
			return err
		}
		frylog.Log("Generated %s.", config.PlanFile)
	}

	planContentBytes, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("run prepare: read plan: %w", err)
	}
	planContent := string(planContentBytes)
	executiveContent, _ := readPrepareOptional(executivePath)

	agentsPath := filepath.Join(projectDir, config.AgentsFile)
	frylog.Log("Step 1: Generating %s (engine: %s)...", config.AgentsFile, engName)
	prompt := step1Prompt(opts.Planning, planContent, executiveContent, mediaManifest)
	beforeMod := textutil.FileModTime(agentsPath)
	output, err := runPrepareStep(ctx, eng, projectDir, prompt)
	if err != nil {
		return err
	}
	if err := textutil.ResolveArtifact(agentsPath, beforeMod, output); err != nil {
		return fmt.Errorf("run prepare: write agents: %w", err)
	}
	if err := validateStep1(agentsPath); err != nil {
		return err
	}
	frylog.Log("Generated %s.", config.AgentsFile)

	agentsContentBytes, err := os.ReadFile(agentsPath)
	if err != nil {
		return fmt.Errorf("run prepare: read agents: %w", err)
	}
	agentsContent := string(agentsContentBytes)

	epicFilename := opts.EpicFilename
	if strings.TrimSpace(epicFilename) == "" {
		epicFilename = "epic.md"
	}
	if !strings.Contains(epicFilename, string(filepath.Separator)) {
		epicFilename = filepath.Join(config.FryDir, epicFilename)
	}
	epicPath := filepath.Join(projectDir, epicFilename)

	frylog.Log("Step 2: Generating %s (engine: %s)...", epicFilename, engName)
	prompt = step2Prompt(opts.Planning, planContent, agentsContent, epicExamplePath, generateEpicPath, opts.UserPrompt, opts.EffortLevel, mediaManifest)
	beforeMod = textutil.FileModTime(epicPath)
	output, err = runPrepareStep(ctx, eng, projectDir, prompt)
	if err != nil {
		return err
	}
	if err := textutil.ResolveArtifact(epicPath, beforeMod, output); err != nil {
		return fmt.Errorf("run prepare: write epic: %w", err)
	}
	if err := validateStep2(epicPath); err != nil {
		return err
	}
	frylog.Log("Generated %s.", epicFilename)

	epicContentBytes, err := os.ReadFile(epicPath)
	if err != nil {
		return fmt.Errorf("run prepare: read epic: %w", err)
	}

	frylog.Log("Step 3: Generating %s (engine: %s)...", config.DefaultVerificationFile, engName)
	prompt = step3Prompt(opts.Planning, planContent, string(epicContentBytes), verificationExamplePath, opts.UserPrompt, mediaManifest)
	verificationPath := filepath.Join(projectDir, config.DefaultVerificationFile)
	beforeMod = textutil.FileModTime(verificationPath)
	output, err = runPrepareStep(ctx, eng, projectDir, prompt)
	if err != nil {
		return err
	}
	if err := textutil.ResolveArtifact(verificationPath, beforeMod, output); err != nil {
		return fmt.Errorf("run prepare: write verification: %w", err)
	}
	if err := validateStep3(verificationPath); err != nil {
		frylog.Log("WARNING: %s has no @check_* directives. Continuing without verification.", config.DefaultVerificationFile)
		return nil
	}
	frylog.Log("Generated %s.", config.DefaultVerificationFile)

	return nil
}

func validatePreparePrerequisites(projectDir string) error {
	planPath := filepath.Join(projectDir, config.PlanFile)
	executivePath := filepath.Join(projectDir, config.ExecutiveFile)
	if _, err := os.Stat(planPath); err == nil {
		return nil
	}
	if _, err := os.Stat(executivePath); err == nil {
		return nil
	}
	return fmt.Errorf("prepare requires %s or %s", config.PlanFile, config.ExecutiveFile)
}

func validateStep0(planPath string) error {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("validate step 0: %w", err)
	}
	if len(strings.Fields(string(data))) < 100 {
		return fmt.Errorf("validate step 0: generated %s is too shallow", config.PlanFile)
	}
	return nil
}

func validateStep1(agentsPath string) error {
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		return fmt.Errorf("validate step 1: %w", err)
	}
	if !numberedRulePattern.Match(data) {
		return fmt.Errorf("validate step 1: AGENTS.md has no numbered rules")
	}
	return nil
}

func validateStep2(epicPath string) error {
	data, err := os.ReadFile(epicPath)
	if err != nil {
		return fmt.Errorf("validate step 2: %w", err)
	}
	if countDirective(string(data), "@sprint ") == 0 {
		return fmt.Errorf("validate step 2: epic.md contains no @sprint blocks")
	}
	return nil
}

func validateStep3(verificationPath string) error {
	data, err := os.ReadFile(verificationPath)
	if err != nil {
		return fmt.Errorf("validate step 3: %w", err)
	}
	if countDirective(string(data), "@check_") == 0 {
		return fmt.Errorf("validate step 3: verification.md contains no @check_* directives")
	}
	return nil
}

func runPrepareStep(ctx context.Context, eng engine.Engine, projectDir, prompt string) (string, error) {
	output, _, err := eng.Run(ctx, prompt, engine.RunOpts{WorkDir: projectDir})
	if err != nil && strings.TrimSpace(output) == "" {
		return "", fmt.Errorf("run prepare step: %w", err)
	}
	return output, nil
}

func writeEmbeddedTemplates(dir string) (epicExamplePath, verificationExamplePath, generateEpicPath string, err error) {
	write := func(name string) (string, error) {
		data, readErr := fs.ReadFile(templates.TemplateFS, name)
		if readErr != nil {
			return "", readErr
		}
		path := filepath.Join(dir, name)
		if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
			return "", writeErr
		}
		return path, nil
	}

	if epicExamplePath, err = write("epic-example.md"); err != nil {
		return "", "", "", err
	}
	if verificationExamplePath, err = write("verification-example.md"); err != nil {
		return "", "", "", err
	}
	if generateEpicPath, err = write("GENERATE_EPIC.md"); err != nil {
		return "", "", "", err
	}
	return epicExamplePath, verificationExamplePath, generateEpicPath, nil
}

func step0Prompt(planning bool, executiveContent, mediaManifest string) string {
	if planning {
		return PlanningStep0Prompt(executiveContent, mediaManifest)
	}
	return SoftwareStep0Prompt(executiveContent, mediaManifest)
}

func step1Prompt(planning bool, planContent, executiveContent, mediaManifest string) string {
	if planning {
		return PlanningStep1Prompt(planContent, executiveContent, mediaManifest)
	}
	return SoftwareStep1Prompt(planContent, executiveContent, mediaManifest)
}

func step2Prompt(planning bool, planContent, agentsContent, epicExamplePath, generateEpicPath, userPrompt string, effort epic.EffortLevel, mediaManifest string) string {
	if planning {
		return PlanningStep2Prompt(planContent, agentsContent, epicExamplePath, userPrompt, effort, mediaManifest)
	}
	return SoftwareStep2Prompt(planContent, agentsContent, epicExamplePath, generateEpicPath, userPrompt, effort, mediaManifest)
}

func step3Prompt(planning bool, planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest string) string {
	if planning {
		return PlanningStep3Prompt(planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest)
	}
	return SoftwareStep3Prompt(planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest)
}

func readPrepareOptional(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func countDirective(content, prefix string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			count++
		}
	}
	return count
}

