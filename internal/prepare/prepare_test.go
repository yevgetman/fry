package prepare

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/engine"
	frylog "github.com/yevgetman/fry/internal/log"
)

type fakeEngine struct {
	output string
}

func (f *fakeEngine) Run(_ context.Context, _ string, _ engine.RunOpts) (string, int, error) {
	return f.output, 0, nil
}

func (f *fakeEngine) Name() string {
	return "fake"
}

func TestPrepareValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	planPath := dir + "/plans/plan.md"
	require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
	require.NoError(t, os.WriteFile(planPath, []byte(strings.Repeat("word ", 100)), 0o644))
	require.NoError(t, validateStep0(planPath))

	agentsPath := dir + "/.fry/AGENTS.md"
	require.NoError(t, os.MkdirAll(dir+"/.fry", 0o755))
	require.NoError(t, os.WriteFile(agentsPath, []byte("1. Rule one\n"), 0o644))
	require.NoError(t, validateStep1(agentsPath))

	epicPath := dir + "/.fry/epic.md"
	require.NoError(t, os.WriteFile(epicPath, []byte("@sprint 1\n"), 0o644))
	require.NoError(t, validateStep2(epicPath))

	verificationPath := dir + "/.fry/verification.md"
	require.NoError(t, os.WriteFile(verificationPath, []byte("@check_file foo\n"), 0o644))
	require.NoError(t, validateStep3(verificationPath))
}

func TestSoftwareStep2ReferencesGenerateEpic(t *testing.T) {
	t.Parallel()

	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "", "", "")
	assert.Contains(t, prompt, "/tmp/GENERATE_EPIC.md")
}

func TestPlanningStep2NoGenerateEpic(t *testing.T) {
	t.Parallel()

	prompt := PlanningStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", "", "")
	assert.NotContains(t, prompt, "GENERATE_EPIC.md")
}

func TestEffortSizingGuidance_Low(t *testing.T) {
	t.Parallel()

	guidance := effortSizingGuidance("low")
	assert.Contains(t, guidance, "AT MOST 2 sprints")
	assert.Contains(t, guidance, "EFFORT LEVEL: LOW")
}

func TestEffortSizingGuidance_Max(t *testing.T) {
	t.Parallel()

	guidance := effortSizingGuidance("max")
	assert.Contains(t, guidance, "30-50")
	assert.Contains(t, guidance, "EFFORT LEVEL: MAX")
}

func TestEffortSizingGuidance_Auto(t *testing.T) {
	t.Parallel()

	guidance := effortSizingGuidance("")
	assert.Contains(t, guidance, "AUTO-DETECT")
	assert.Contains(t, guidance, "Analyze the plan")
}

func TestSoftwareStep2Prompt_IncludesEffort(t *testing.T) {
	t.Parallel()

	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "low", "", "")
	assert.Contains(t, prompt, "EFFORT LEVEL: LOW")
	assert.Contains(t, prompt, "AT MOST 2 sprints")
}

func TestPreparePrerequisites(t *testing.T) {
	t.Parallel()

	t.Run("fails when no files and no user prompt", func(t *testing.T) {
		t.Parallel()
		err := validatePreparePrerequisites(t.TempDir(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plans/plan.md")
		assert.Contains(t, err.Error(), "plans/executive.md")
		assert.Contains(t, err.Error(), "--user-prompt")
	})

	t.Run("passes with user prompt and no files", func(t *testing.T) {
		t.Parallel()
		err := validatePreparePrerequisites(t.TempDir(), "build me an app")
		require.NoError(t, err)
	})

	t.Run("passes with executive file and no user prompt", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
		require.NoError(t, os.WriteFile(dir+"/plans/executive.md", []byte("exec"), 0o644))
		err := validatePreparePrerequisites(dir, "")
		require.NoError(t, err)
	})
}

func TestBootstrapExecutive_UserApproves(t *testing.T) {
	// Not parallel: mutates package-level newEngine variable
	dir := t.TempDir()
	executivePath := dir + "/plans/executive.md"

	stdin := strings.NewReader("y\n")
	var stdout strings.Builder

	oldNewEngine := newEngine
	newEngine = func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "# My Project\n\nGenerated executive content."}, nil
	}
	defer func() { newEngine = oldNewEngine }()

	eng, _ := newEngine("fake")
	err := bootstrapExecutive(context.Background(), eng, "fake", PrepareOpts{
		ProjectDir: dir,
		UserPrompt: "build a todo app",
		Stdin:      stdin,
		Stdout:     &stdout,
	}, executivePath, "", "")

	require.NoError(t, err)
	data, readErr := os.ReadFile(executivePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "Generated executive content")
	assert.Contains(t, stdout.String(), "Generated executive context")
}

func TestSoftwareStep0Prompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/spec.yaml (100 B)\n```yaml\nopenapi: 3.0.0\n```\n"
	prompt := SoftwareStep0Prompt("executive content", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "openapi: 3.0.0")
	assert.Contains(t, prompt, "supplementary asset documents")
}

func TestSoftwareStep0Prompt_WithoutAssets(t *testing.T) {
	t.Parallel()

	prompt := SoftwareStep0Prompt("executive content", "", "")
	assert.NotContains(t, prompt, "SUPPLEMENTARY ASSETS")
}

func TestSoftwareStep2Prompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/api.json (50 B)\n```json\n{}\n```\n"
	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "supplementary asset documents")
}

func TestExecutiveFromUserPromptPrompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/brief.md (30 B)\n```markdown\n# Brief\n```\n"
	prompt := ExecutiveFromUserPromptPrompt("build a todo app", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "supplementary asset documents")
}

func TestPlanningStep0Prompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/research.md (200 B)\n```markdown\n# Research\n```\n"
	prompt := PlanningStep0Prompt("executive content", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "supplementary asset documents")
}

func TestPlanningStep2Prompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/framework.yaml (80 B)\n```yaml\nname: test\n```\n"
	prompt := PlanningStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "supplementary asset documents")
}

func TestPlanningExecutiveFromUserPromptPrompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/context.txt (40 B)\n```\nsome context\n```\n"
	prompt := PlanningExecutiveFromUserPromptPrompt("analyze market trends", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "supplementary asset documents")
}

func TestWritingStep2NoGenerateEpic(t *testing.T) {
	t.Parallel()

	prompt := WritingStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", "", "")
	assert.NotContains(t, prompt, "GENERATE_EPIC.md")
}

func TestWritingStep0Prompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/research.md (200 B)\n```markdown\n# Research\n```\n"
	prompt := WritingStep0Prompt("executive content", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "supplementary asset documents")
}

func TestWritingStep2Prompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/source.md (80 B)\n```markdown\ndata\n```\n"
	prompt := WritingStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "supplementary asset documents")
}

func TestWritingExecutiveFromUserPromptPrompt_WithAssets(t *testing.T) {
	t.Parallel()

	assetsSection := "# ===== SUPPLEMENTARY ASSETS =====\n## File: assets/outline.md (40 B)\n```markdown\n# Outline\n```\n"
	prompt := WritingExecutiveFromUserPromptPrompt("write a book about Go", "", assetsSection)
	assert.Contains(t, prompt, "SUPPLEMENTARY ASSETS")
	assert.Contains(t, prompt, "supplementary asset documents")
	assert.Contains(t, prompt, "Voice & Tone")
}

func TestEffortSizingGuidanceWriting_Low(t *testing.T) {
	t.Parallel()

	guidance := effortSizingGuidanceWriting("low")
	assert.Contains(t, guidance, "AT MOST 2 sprints")
	assert.Contains(t, guidance, "EFFORT LEVEL: LOW")
}

func TestEffortSizingGuidanceWriting_Max(t *testing.T) {
	t.Parallel()

	guidance := effortSizingGuidanceWriting("max")
	assert.Contains(t, guidance, "30-50")
	assert.Contains(t, guidance, "EFFORT LEVEL: MAX")
}

func TestEffortSizingGuidanceWriting_Auto(t *testing.T) {
	t.Parallel()

	guidance := effortSizingGuidanceWriting("")
	assert.Contains(t, guidance, "AUTO-DETECT")
	assert.Contains(t, guidance, "content plan")
}

func TestBootstrapExecutive_UserDeclines(t *testing.T) {
	// Not parallel: mutates package-level newEngine variable
	dir := t.TempDir()
	executivePath := dir + "/plans/executive.md"

	stdin := strings.NewReader("n\n")
	var stdout strings.Builder

	oldNewEngine := newEngine
	newEngine = func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "# My Project\n\nGenerated content."}, nil
	}
	defer func() { newEngine = oldNewEngine }()

	eng, _ := newEngine("fake")
	err := bootstrapExecutive(context.Background(), eng, "fake", PrepareOpts{
		ProjectDir: dir,
		UserPrompt: "build a todo app",
		Stdin:      stdin,
		Stdout:     &stdout,
	}, executivePath, "", "")

	require.ErrorIs(t, err, ErrUserDeclined)
	_, statErr := os.Stat(executivePath)
	assert.True(t, os.IsNotExist(statErr), "executive.md should not be created when user declines")
}

func TestBootstrapExecutive_LogsAssetsAndMedia(t *testing.T) {
	// Not parallel: mutates package-level newEngine variable
	dir := t.TempDir()
	executivePath := dir + "/plans/executive.md"

	stdin := strings.NewReader("y\n")
	var stdout strings.Builder
	var logBuf strings.Builder
	frylog.SetLogFile(&logBuf)
	defer frylog.SetLogFile(nil)

	oldNewEngine := newEngine
	newEngine = func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "# My Project\n\n" + strings.Repeat("word ", 100)}, nil
	}
	defer func() { newEngine = oldNewEngine }()

	eng, _ := newEngine("fake")
	err := bootstrapExecutive(context.Background(), eng, "fake", PrepareOpts{
		ProjectDir: dir,
		UserPrompt: "build a todo app",
		Stdin:      stdin,
		Stdout:     &stdout,
	}, executivePath, "media manifest content", "assets section content")

	require.NoError(t, err)
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "user prompt")
	assert.Contains(t, logOutput, "assets/ assets")
	assert.Contains(t, logOutput, "media/ manifest")
}

func TestBootstrapExecutive_LogsPromptOnlyWhenNoExtras(t *testing.T) {
	// Not parallel: mutates package-level newEngine variable
	dir := t.TempDir()
	executivePath := dir + "/plans/executive.md"

	stdin := strings.NewReader("y\n")
	var stdout strings.Builder
	var logBuf strings.Builder
	frylog.SetLogFile(&logBuf)
	defer frylog.SetLogFile(nil)

	oldNewEngine := newEngine
	newEngine = func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "# My Project\n\n" + strings.Repeat("word ", 100)}, nil
	}
	defer func() { newEngine = oldNewEngine }()

	eng, _ := newEngine("fake")
	err := bootstrapExecutive(context.Background(), eng, "fake", PrepareOpts{
		ProjectDir: dir,
		UserPrompt: "build a todo app",
		Stdin:      stdin,
		Stdout:     &stdout,
	}, executivePath, "", "")

	require.NoError(t, err)
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "from user prompt (engine: fake)")
	assert.NotContains(t, logOutput, "assets/")
	assert.NotContains(t, logOutput, "media/")
}

func TestParseSanitySummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected SanitySummary
	}{
		{
			name: "well-formed output",
			input: `PROJECT_TYPE: Software (REST API)
GOAL: Build a todo app with PostgreSQL backend
EXPECTED_OUTPUT: Go binary with REST API, database migrations, Docker setup
KEY_TOPICS: REST API, PostgreSQL, authentication, Docker
EFFORT: medium (3-4 sprints)`,
			expected: SanitySummary{
				ProjectType:    "Software (REST API)",
				Goal:           "Build a todo app with PostgreSQL backend",
				ExpectedOutput: "Go binary with REST API, database migrations, Docker setup",
				KeyTopics:      "REST API, PostgreSQL, authentication, Docker",
				EffortEstimate: "medium (3-4 sprints)",
			},
		},
		{
			name: "wrapped in markdown fences",
			input: "```\nPROJECT_TYPE: Writing (guide)\nGOAL: Write a Go concurrency guide\nEXPECTED_OUTPUT: 6 chapters in output/\nKEY_TOPICS: goroutines, channels\nEFFORT: medium (3 sprints)\n```",
			expected: SanitySummary{
				ProjectType:    "Writing (guide)",
				Goal:           "Write a Go concurrency guide",
				ExpectedOutput: "6 chapters in output/",
				KeyTopics:      "goroutines, channels",
				EffortEstimate: "medium (3 sprints)",
			},
		},
		{
			name: "missing fields",
			input: `PROJECT_TYPE: Planning (analysis)
GOAL: Analyze market trends`,
			expected: SanitySummary{
				ProjectType: "Planning (analysis)",
				Goal:        "Analyze market trends",
			},
		},
		{
			name: "extra noise lines",
			input: `Here is the summary:

PROJECT_TYPE: Software (CLI tool)
GOAL: Build a CLI
Some random line
EXPECTED_OUTPUT: binary
KEY_TOPICS: cobra, testing
EFFORT: low (1 sprint)`,
			expected: SanitySummary{
				ProjectType:    "Software (CLI tool)",
				Goal:           "Build a CLI",
				ExpectedOutput: "binary",
				KeyTopics:      "cobra, testing",
				EffortEstimate: "low (1 sprint)",
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: SanitySummary{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseSanitySummary(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDisplaySanitySummary(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	displaySanitySummary(&buf, SanitySummary{
		ProjectType:    "Software (REST API)",
		Goal:           "Build a todo app",
		ExpectedOutput: "Go binary",
		KeyTopics:      "REST, PostgreSQL",
		EffortEstimate: "medium (3 sprints)",
	})
	output := buf.String()
	assert.Contains(t, output, "Project summary")
	assert.Contains(t, output, "Software (REST API)")
	assert.Contains(t, output, "Build a todo app")
	assert.Contains(t, output, "Go binary")
	assert.Contains(t, output, "REST, PostgreSQL")
	assert.Contains(t, output, "medium (3 sprints)")
}

func TestDisplaySanitySummary_UnknownFields(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	displaySanitySummary(&buf, SanitySummary{
		ProjectType: "Software (app)",
	})
	output := buf.String()
	assert.Contains(t, output, "Software (app)")
	assert.Contains(t, output, "(unknown)")
}

func TestRunSanityCheck_UserApproves(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software (CLI)\nGOAL: Build a CLI tool\nEXPECTED_OUTPUT: binary\nKEY_TOPICS: cobra\nEFFORT: low (1 sprint)"}
	stdin := strings.NewReader("y\n")
	var stdout strings.Builder

	err := runSanityCheck(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan content", "", "", "")

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Project summary")
	assert.Contains(t, stdout.String(), "Software (CLI)")
}

func TestRunSanityCheck_DefaultYes(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low"}
	stdin := strings.NewReader("\n") // empty line = default yes
	var stdout strings.Builder

	err := runSanityCheck(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
}

func TestRunSanityCheck_UserDeclines(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low"}
	stdin := strings.NewReader("n\n")
	var stdout strings.Builder

	err := runSanityCheck(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.ErrorIs(t, err, ErrSanityCheckDeclined)
}

func TestRunSanityCheck_EOF(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low"}
	stdin := strings.NewReader("") // EOF
	var stdout strings.Builder

	err := runSanityCheck(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.ErrorIs(t, err, ErrSanityCheckDeclined)
}

func TestSoftwareSanityCheckPrompt_IncludesInputs(t *testing.T) {
	t.Parallel()

	prompt := SoftwareSanityCheckPrompt("my plan", "my executive", "focus on backend", "medium", "media manifest", "assets section")
	assert.Contains(t, prompt, "senior software architect")
	assert.Contains(t, prompt, "my plan")
	assert.Contains(t, prompt, "my executive")
	assert.Contains(t, prompt, "focus on backend")
	assert.Contains(t, prompt, "media manifest")
	assert.Contains(t, prompt, "assets section")
	assert.Contains(t, prompt, `"medium"`)
}

func TestSoftwareSanityCheckPrompt_OmitsMissingInputs(t *testing.T) {
	t.Parallel()

	prompt := SoftwareSanityCheckPrompt("my plan", "", "", "", "", "")
	assert.Contains(t, prompt, "my plan")
	assert.NotContains(t, prompt, "Executive context")
	assert.NotContains(t, prompt, "User directive")
	assert.NotContains(t, prompt, "Media assets")
}

func TestPlanningSanityCheckPrompt(t *testing.T) {
	t.Parallel()

	prompt := PlanningSanityCheckPrompt("plan", "", "", "", "", "")
	assert.Contains(t, prompt, "senior strategic planner")
}

func TestWritingSanityCheckPrompt(t *testing.T) {
	t.Parallel()

	prompt := WritingSanityCheckPrompt("plan", "", "", "", "", "")
	assert.Contains(t, prompt, "senior author and content strategist")
}

func TestRunPrepare_SanityCheckSkipped(t *testing.T) {
	// Not parallel: mutates package-level newEngine and frylog.SetLogFile
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
	require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

	var logBuf strings.Builder
	frylog.SetLogFile(&logBuf)
	defer frylog.SetLogFile(nil)

	oldNewEngine := newEngine
	newEngine = func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "1. Rule one\n@sprint 1\n@check_file foo\n"}, nil
	}
	defer func() { newEngine = oldNewEngine }()

	err := RunPrepare(context.Background(), PrepareOpts{
		ProjectDir:      dir,
		Engine:          "claude",
		SkipSanityCheck: true,
	})

	require.NoError(t, err)
	logOutput := logBuf.String()
	assert.NotContains(t, logOutput, "Sanity check")
}

func TestRunPrepare_PlanExistsWithoutExecutive(t *testing.T) {
	// Not parallel: mutates package-level newEngine and frylog.SetLogFile
	dir := t.TempDir()

	// Create plan but NOT executive.
	require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
	require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

	var logBuf strings.Builder
	frylog.SetLogFile(&logBuf)
	defer frylog.SetLogFile(nil)

	oldNewEngine := newEngine
	newEngine = func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "1. Rule one\n@sprint 1\n@check_file foo\n"}, nil
	}
	defer func() { newEngine = oldNewEngine }()

	err := RunPrepare(context.Background(), PrepareOpts{
		ProjectDir:      dir,
		Engine:          "claude",
		SkipSanityCheck: true,
	})

	require.NoError(t, err)
	logOutput := logBuf.String()
	// Executive doesn't exist — should not be mentioned.
	assert.NotContains(t, logOutput, "Using existing plans/executive.md")
	assert.Contains(t, logOutput, "Using existing plans/plan.md")
	// Step 1 should list only plan.md (no executive).
	assert.Contains(t, logOutput, "Step 1: Generating .fry/AGENTS.md from plans/plan.md (engine: claude)")
}

func TestRunPrepare_LogsExistingFiles(t *testing.T) {
	// Not parallel: mutates package-level newEngine and frylog.SetLogFile
	dir := t.TempDir()

	// Create existing plan and executive files.
	require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
	require.NoError(t, os.WriteFile(dir+"/plans/executive.md", []byte("exec content"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

	var logBuf strings.Builder
	frylog.SetLogFile(&logBuf)
	defer frylog.SetLogFile(nil)

	oldNewEngine := newEngine
	newEngine = func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "1. Rule one\n@sprint 1\n@check_file foo\n"}, nil
	}
	defer func() { newEngine = oldNewEngine }()

	err := RunPrepare(context.Background(), PrepareOpts{
		ProjectDir:      dir,
		Engine:          "claude",
		SkipSanityCheck: true,
	})

	require.NoError(t, err)
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "Using existing plans/executive.md")
	assert.Contains(t, logOutput, "Using existing plans/plan.md")
}

func TestRunPrepare_LogsUserPromptAndAssets(t *testing.T) {
	// Not parallel: mutates package-level newEngine and frylog.SetLogFile
	dir := t.TempDir()

	// Create existing plan and executive files.
	require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
	require.NoError(t, os.WriteFile(dir+"/plans/executive.md", []byte("exec content"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

	// Create an assets directory with a file.
	require.NoError(t, os.MkdirAll(dir+"/assets", 0o755))
	require.NoError(t, os.WriteFile(dir+"/assets/spec.yaml", []byte("openapi: 3.0.0"), 0o644))

	var logBuf strings.Builder
	frylog.SetLogFile(&logBuf)
	defer frylog.SetLogFile(nil)

	oldNewEngine := newEngine
	newEngine = func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "1. Rule one\n@sprint 1\n@check_file foo\n"}, nil
	}
	defer func() { newEngine = oldNewEngine }()

	err := RunPrepare(context.Background(), PrepareOpts{
		ProjectDir:      dir,
		Engine:          "claude",
		UserPrompt:      "focus on backend",
		SkipSanityCheck: true,
	})

	require.NoError(t, err)
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "User prompt detected")
	assert.Contains(t, logOutput, "Supplementary assets detected (1 file(s) in assets/)")
	assert.Contains(t, logOutput, "Step 2: Generating .fry/epic.md from plans/plan.md, .fry/AGENTS.md, user prompt, assets/ assets")
}
