package prepare

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yevgetman/fry/internal/assets"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/media"
	"github.com/yevgetman/fry/internal/textutil"
	"github.com/yevgetman/fry/templates"
)

type PrepareOpts struct {
	ProjectDir       string
	EpicFilename     string
	Engine           string
	UserPrompt       string
	UserPromptSource string // human-readable origin, e.g. "--user-prompt-file path" or "--user-prompt flag"
	ValidateOnly     bool
	SkipSanityCheck  bool
	EngineFactory    func(string) (engine.Engine, error)       // optional; defaults to engine.NewEngine
	LogFunc          func(format string, args ...interface{})   // optional; defaults to frylog.Log
	Mode             Mode
	EffortLevel      epic.EffortLevel
	EnableReview     bool      // include @review_between_sprints in the generated epic
	Stdin            io.Reader // for interactive confirmation (defaults to os.Stdin)
	Stdout           io.Writer // for displaying generated content (defaults to os.Stdout)
}

var numberedRulePattern = regexp.MustCompile(`(?m)^[0-9]+\.`)

// ErrUserDeclined is returned when the user declines the generated executive context.
var ErrUserDeclined = fmt.Errorf("user declined generated executive context")

func resolveLogFunc(opts PrepareOpts) func(string, ...interface{}) {
	if opts.LogFunc != nil {
		return opts.LogFunc
	}
	return frylog.Log
}

func RunPrepare(ctx context.Context, opts PrepareOpts) error {
	logf := resolveLogFunc(opts)

	projectDir := opts.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}

	if err := validatePreparePrerequisites(projectDir, opts.UserPrompt); err != nil {
		return err
	}
	if opts.ValidateOnly {
		return nil
	}

	engName, err := engine.ResolveEngine(opts.Engine, "", "", config.DefaultPrepareEngine)
	if err != nil {
		return fmt.Errorf("run prepare: %w", err)
	}
	engineFactory := opts.EngineFactory
	if engineFactory == nil {
		engineFactory = engine.NewEngine
	}
	eng, err := engineFactory(engName)
	if err != nil {
		return fmt.Errorf("run prepare: %w", err)
	}
	prepModel := engine.ResolveModelForSession(engName, string(opts.EffortLevel), engine.SessionPrepare)

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

	switch opts.Mode {
	case ModePlanning:
		outputDir := filepath.Join(projectDir, config.PlanningOutputDir)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("run prepare: create planning output dir: %w", err)
		}
	case ModeWriting:
		outputDir := filepath.Join(projectDir, config.WritingOutputDir)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("run prepare: create writing output dir: %w", err)
		}
	}

	// Scan media directory for available assets (optional).
	mediaAssets, mediaTruncated, mediaErr := media.Scan(projectDir)
	if mediaErr != nil {
		logf("WARNING: could not scan media directory: %v", mediaErr)
	}
	if mediaTruncated {
		logf("WARNING: media directory has more than %d files — scan truncated", media.MaxAssets)
	}
	mediaManifest := media.BuildManifest(mediaAssets)

	// Scan assets directory for supplementary documents (optional).
	assetsResult, assetsErr := assets.Scan(projectDir)
	if assetsErr != nil {
		logf("WARNING: could not scan assets directory: %v", assetsErr)
	}
	for _, w := range assetsResult.Warnings {
		logf("WARNING: assets: %s", w)
	}
	if assetsResult.Truncated {
		logf("WARNING: assets directory scan truncated — some files skipped")
	}
	assetsSection := assets.BuildSection(assetsResult)

	hasAssets := len(assetsResult.Assets) > 0
	hasMedia := len(mediaAssets) > 0
	hasUserPrompt := strings.TrimSpace(opts.UserPrompt) != ""

	planPath := filepath.Join(projectDir, config.PlanFile)
	executivePath := filepath.Join(projectDir, config.ExecutiveFile)
	_, planStatErr := os.Stat(planPath)
	_, execStatErr := os.Stat(executivePath)
	planMissing := os.IsNotExist(planStatErr)
	execMissing := os.IsNotExist(execStatErr)

	// Log detected inputs for visibility.
	if hasUserPrompt {
		if opts.UserPromptSource != "" {
			logf("User prompt loaded from %s — will be included in generation.", opts.UserPromptSource)
		} else {
			logf("User prompt detected — will be included in generation.")
		}
	}
	if hasAssets {
		logf("Supplementary assets detected (%d file(s) in %s/) — will be included in generation.", len(assetsResult.Assets), config.AssetsDir)
	}
	if hasMedia {
		logf("Media assets detected (%d file(s) in %s/) — manifest will be included in generation.", len(mediaAssets), config.MediaDir)
	}

	// Bootstrap: generate executive.md from user prompt if neither plan nor executive exists.
	if planMissing && execMissing {
		if err := bootstrapExecutive(ctx, eng, engName, opts, executivePath, mediaManifest, assetsSection); err != nil {
			return err
		}
	} else if !execMissing {
		logf("Using existing %s.", config.ExecutiveFile)
	}

	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		var step0Inputs []string
		step0Inputs = append(step0Inputs, config.ExecutiveFile)
		if hasAssets {
			step0Inputs = append(step0Inputs, config.AssetsDir+"/ assets")
		}
		if hasMedia {
			step0Inputs = append(step0Inputs, fmt.Sprintf("%s/ manifest", config.MediaDir))
		}
		logf("Step 0: Generating %s from %s (engine: %s, model: %s)...", config.PlanFile, strings.Join(step0Inputs, ", "), engName, prepModel)
		executiveContent, err := os.ReadFile(executivePath)
		if err != nil {
			return fmt.Errorf("run prepare: read executive: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
			return err
		}
		prompt := step0Prompt(opts.Mode, string(executiveContent), mediaManifest, assetsSection)
		beforeSize := textutil.FileSize(planPath)
		output, err := runPrepareStep(ctx, eng, projectDir, prompt, prepModel)
		if err != nil {
			return err
		}
		if err := textutil.ResolveArtifact(planPath, beforeSize, output); err != nil {
			return err
		}
		if err := validateStep0(planPath); err != nil {
			return err
		}
		logf("Generated %s.", config.PlanFile)
	} else {
		logf("Using existing %s.", config.PlanFile)
	}

	planContentBytes, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("run prepare: read plan: %w", err)
	}
	planContent := string(planContentBytes)
	executiveContent, _ := readPrepareOptional(executivePath)

	// Sanity check: summarize project and ask user to confirm before generating artifacts.
	if !opts.SkipSanityCheck {
		result, err := runSanityCheck(ctx, eng, opts,
			planContent, executiveContent, mediaManifest, assetsSection)
		if err != nil {
			return err
		}
		opts.UserPrompt = result.UserPrompt
		opts.EffortLevel = result.EffortLevel
		opts.EnableReview = result.EnableReview
		// Refresh derived flags — user may have added a prompt or changed effort via adjust.
		hasUserPrompt = strings.TrimSpace(opts.UserPrompt) != ""
		prepModel = engine.ResolveModelForSession(engName, string(opts.EffortLevel), engine.SessionPrepare)
	}

	agentsPath := filepath.Join(projectDir, config.AgentsFile)
	var step1Inputs []string
	step1Inputs = append(step1Inputs, config.PlanFile)
	if strings.TrimSpace(executiveContent) != "" {
		step1Inputs = append(step1Inputs, config.ExecutiveFile)
	}
	if hasMedia {
		step1Inputs = append(step1Inputs, config.MediaDir+"/ manifest")
	}
	logf("Step 1: Generating %s from %s (engine: %s, model: %s)...", config.AgentsFile, strings.Join(step1Inputs, ", "), engName, prepModel)
	prompt := step1Prompt(opts.Mode, planContent, executiveContent, mediaManifest)
	beforeSize := textutil.FileSize(agentsPath)
	output, err := runPrepareStep(ctx, eng, projectDir, prompt, prepModel)
	if err != nil {
		return err
	}
	if err := textutil.ResolveArtifact(agentsPath, beforeSize, output); err != nil {
		return fmt.Errorf("run prepare: write agents: %w", err)
	}
	if err := validateStep1(agentsPath); err != nil {
		return err
	}
	logf("Generated %s.", config.AgentsFile)

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

	var step2Inputs []string
	step2Inputs = append(step2Inputs, config.PlanFile, config.AgentsFile)
	if hasUserPrompt {
		step2Inputs = append(step2Inputs, "user prompt")
	}
	if hasAssets {
		step2Inputs = append(step2Inputs, config.AssetsDir+"/ assets")
	}
	if hasMedia {
		step2Inputs = append(step2Inputs, config.MediaDir+"/ manifest")
	}
	logf("Step 2: Generating %s from %s (engine: %s, model: %s)...", epicFilename, strings.Join(step2Inputs, ", "), engName, prepModel)
	prompt = step2Prompt(opts.Mode, planContent, agentsContent, epicExamplePath, generateEpicPath, opts.UserPrompt, opts.EffortLevel, opts.EnableReview, mediaManifest, assetsSection)
	beforeSize = textutil.FileSize(epicPath)
	output, err = runPrepareStep(ctx, eng, projectDir, prompt, prepModel)
	if err != nil {
		return err
	}
	if err := textutil.ResolveArtifact(epicPath, beforeSize, output); err != nil {
		return fmt.Errorf("run prepare: write epic: %w", err)
	}
	if err := validateStep2(epicPath); err != nil {
		return err
	}
	logf("Generated %s.", epicFilename)

	epicContentBytes, err := os.ReadFile(epicPath)
	if err != nil {
		return fmt.Errorf("run prepare: read epic: %w", err)
	}

	var step3Inputs []string
	step3Inputs = append(step3Inputs, config.PlanFile, epicFilename)
	if hasUserPrompt {
		step3Inputs = append(step3Inputs, "user prompt")
	}
	if hasMedia {
		step3Inputs = append(step3Inputs, config.MediaDir+"/ manifest")
	}
	logf("Step 3: Generating %s from %s (engine: %s, model: %s)...", config.DefaultVerificationFile, strings.Join(step3Inputs, ", "), engName, prepModel)
	prompt = step3Prompt(opts.Mode, planContent, string(epicContentBytes), verificationExamplePath, opts.UserPrompt, mediaManifest)
	verificationPath := filepath.Join(projectDir, config.DefaultVerificationFile)
	beforeSize = textutil.FileSize(verificationPath)
	output, err = runPrepareStep(ctx, eng, projectDir, prompt, prepModel)
	if err != nil {
		return err
	}
	if err := textutil.ResolveArtifact(verificationPath, beforeSize, output); err != nil {
		return fmt.Errorf("run prepare: write verification: %w", err)
	}
	if err := validateStep3(verificationPath); err != nil {
		logf("WARNING: %s has no @check_* directives. Continuing without verification.", config.DefaultVerificationFile)
		return nil
	}
	logf("Generated %s.", config.DefaultVerificationFile)

	return nil
}

func validatePreparePrerequisites(projectDir, userPrompt string) error {
	if strings.TrimSpace(userPrompt) != "" {
		return nil
	}
	planPath := filepath.Join(projectDir, config.PlanFile)
	executivePath := filepath.Join(projectDir, config.ExecutiveFile)
	if _, err := os.Stat(planPath); err == nil {
		return nil
	}
	if _, err := os.Stat(executivePath); err == nil {
		return nil
	}
	return fmt.Errorf("prepare requires %s, %s, or --user-prompt", config.PlanFile, config.ExecutiveFile)
}

func bootstrapExecutive(ctx context.Context, eng engine.Engine, engName string, opts PrepareOpts, executivePath, mediaManifest, assetsSection string) error {
	logf := resolveLogFunc(opts)

	// UserPrompt is guaranteed non-empty by the caller (validatePreparePrerequisites passed).
	var bootstrapInputs []string
	bootstrapInputs = append(bootstrapInputs, "user prompt")
	if strings.TrimSpace(assetsSection) != "" {
		bootstrapInputs = append(bootstrapInputs, config.AssetsDir+"/ assets")
	}
	if strings.TrimSpace(mediaManifest) != "" {
		bootstrapInputs = append(bootstrapInputs, config.MediaDir+"/ manifest")
	}
	prepModel := engine.ResolveModelForSession(engName, string(opts.EffortLevel), engine.SessionPrepare)
	logf("Generating %s from %s (engine: %s, model: %s)...", config.ExecutiveFile, strings.Join(bootstrapInputs, ", "), engName, prepModel)

	prompt := executiveFromUserPromptPrompt(opts.Mode, opts.UserPrompt, mediaManifest, assetsSection)
	output, _, err := eng.Run(ctx, prompt, engine.RunOpts{WorkDir: opts.ProjectDir, Model: prepModel})
	if err != nil && strings.TrimSpace(output) == "" {
		return fmt.Errorf("run prepare: generate executive: %w", err)
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── Generated executive context ──────────────────────────────────")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, output)
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "─────────────────────────────────────────────────────────────────")
	fmt.Fprint(stdout, "Proceed with this executive context? [y/N] ")

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("run prepare: read input: %w", err)
		}
		return ErrUserDeclined
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		return ErrUserDeclined
	}

	if err := os.MkdirAll(filepath.Dir(executivePath), 0o755); err != nil {
		return fmt.Errorf("run prepare: create plans dir: %w", err)
	}
	if err := os.WriteFile(executivePath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("run prepare: write executive: %w", err)
	}
	logf("Saved %s.", config.ExecutiveFile)
	return nil
}

func executiveFromUserPromptPrompt(mode Mode, userPrompt, mediaManifest, assetsSection string) string {
	switch mode {
	case ModePlanning:
		return PlanningExecutiveFromUserPromptPrompt(userPrompt, mediaManifest, assetsSection)
	case ModeWriting:
		return WritingExecutiveFromUserPromptPrompt(userPrompt, mediaManifest, assetsSection)
	default:
		return ExecutiveFromUserPromptPrompt(userPrompt, mediaManifest, assetsSection)
	}
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

func runPrepareStep(ctx context.Context, eng engine.Engine, projectDir, prompt, model string) (string, error) {
	output, _, err := eng.Run(ctx, prompt, engine.RunOpts{WorkDir: projectDir, Model: model})
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

func step0Prompt(mode Mode, executiveContent, mediaManifest, assetsSection string) string {
	switch mode {
	case ModePlanning:
		return PlanningStep0Prompt(executiveContent, mediaManifest, assetsSection)
	case ModeWriting:
		return WritingStep0Prompt(executiveContent, mediaManifest, assetsSection)
	default:
		return SoftwareStep0Prompt(executiveContent, mediaManifest, assetsSection)
	}
}

func step1Prompt(mode Mode, planContent, executiveContent, mediaManifest string) string {
	switch mode {
	case ModePlanning:
		return PlanningStep1Prompt(planContent, executiveContent, mediaManifest)
	case ModeWriting:
		return WritingStep1Prompt(planContent, executiveContent, mediaManifest)
	default:
		return SoftwareStep1Prompt(planContent, executiveContent, mediaManifest)
	}
}

func step2Prompt(mode Mode, planContent, agentsContent, epicExamplePath, generateEpicPath, userPrompt string, effort epic.EffortLevel, enableReview bool, mediaManifest, assetsSection string) string {
	switch mode {
	case ModePlanning:
		return PlanningStep2Prompt(planContent, agentsContent, epicExamplePath, userPrompt, effort, enableReview, mediaManifest, assetsSection)
	case ModeWriting:
		return WritingStep2Prompt(planContent, agentsContent, epicExamplePath, userPrompt, effort, enableReview, mediaManifest, assetsSection)
	default:
		return SoftwareStep2Prompt(planContent, agentsContent, epicExamplePath, generateEpicPath, userPrompt, effort, enableReview, mediaManifest, assetsSection)
	}
}

func step3Prompt(mode Mode, planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest string) string {
	switch mode {
	case ModePlanning:
		return PlanningStep3Prompt(planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest)
	case ModeWriting:
		return WritingStep3Prompt(planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest)
	default:
		return SoftwareStep3Prompt(planContent, epicContent, verificationExamplePath, userPrompt, mediaManifest)
	}
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

