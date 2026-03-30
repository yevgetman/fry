package prepare

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
)

// errReader returns an error after reading all buffered data.
type errReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

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

	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "", false, "", "")
	assert.Contains(t, prompt, "/tmp/GENERATE_EPIC.md")
}

func TestPlanningStep2NoGenerateEpic(t *testing.T) {
	t.Parallel()

	prompt := PlanningStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", false, "", "")
	assert.NotContains(t, prompt, "GENERATE_EPIC.md")
}

func TestEffortSizingGuidance_Fast(t *testing.T) {
	t.Parallel()

	guidance := effortSizingGuidance("fast")
	assert.Contains(t, guidance, "AT MOST 2 sprints")
	assert.Contains(t, guidance, "EFFORT LEVEL: FAST")
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

	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "fast", false, "", "")
	assert.Contains(t, prompt, "EFFORT LEVEL: FAST")
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
	t.Parallel()

	dir := t.TempDir()
	executivePath := dir + "/plans/executive.md"

	stdin := strings.NewReader("y\n")
	var stdout strings.Builder

	eng := &fakeEngine{output: "# My Project\n\nGenerated executive content."}
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
	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "", false, "", assetsSection)
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
	prompt := PlanningStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", false, "", assetsSection)
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

	prompt := WritingStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", false, "", "")
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
	prompt := WritingStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", false, "", assetsSection)
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

func TestEffortSizingGuidanceWriting_Fast(t *testing.T) {
	t.Parallel()

	guidance := effortSizingGuidanceWriting("fast")
	assert.Contains(t, guidance, "AT MOST 2 sprints")
	assert.Contains(t, guidance, "EFFORT LEVEL: FAST")
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

func TestBootstrapExecutive_AutoAccept(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	executivePath := dir + "/plans/executive.md"

	stdin := strings.NewReader("")
	var stdout strings.Builder

	eng := &fakeEngine{output: "# My Project\n\nGenerated executive content."}
	err := bootstrapExecutive(context.Background(), eng, "fake", PrepareOpts{
		ProjectDir: dir,
		UserPrompt: "build a todo app",
		AutoAccept: true,
		Stdin:      stdin,
		Stdout:     &stdout,
	}, executivePath, "", "")

	require.NoError(t, err)
	data, readErr := os.ReadFile(executivePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "Generated executive content")
	assert.Contains(t, stdout.String(), "auto-accepted")
}

func TestBootstrapExecutive_UserDeclines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	executivePath := dir + "/plans/executive.md"

	stdin := strings.NewReader("n\n")
	var stdout strings.Builder

	eng := &fakeEngine{output: "# My Project\n\nGenerated content."}
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

func testLogFunc(buf *strings.Builder) func(string, ...interface{}) {
	return func(format string, args ...interface{}) {
		fmt.Fprintf(buf, format+"\n", args...)
	}
}

func TestBootstrapExecutive_Logging(t *testing.T) {
	t.Parallel()

	t.Run("logs assets and media", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		executivePath := dir + "/plans/executive.md"

		stdin := strings.NewReader("y\n")
		var stdout strings.Builder
		var logBuf strings.Builder

		eng := &fakeEngine{output: "# My Project\n\n" + strings.Repeat("word ", 100)}
		err := bootstrapExecutive(context.Background(), eng, "fake", PrepareOpts{
			ProjectDir: dir,
			UserPrompt: "build a todo app",
			Stdin:      stdin,
			Stdout:     &stdout,
			LogFunc:    testLogFunc(&logBuf),
		}, executivePath, "media manifest content", "assets section content")

		require.NoError(t, err)
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "user prompt")
		assert.Contains(t, logOutput, "assets/ assets")
		assert.Contains(t, logOutput, "media/ manifest")
	})

	t.Run("logs prompt only when no extras", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		executivePath := dir + "/plans/executive.md"

		stdin := strings.NewReader("y\n")
		var stdout strings.Builder
		var logBuf strings.Builder

		eng := &fakeEngine{output: "# My Project\n\n" + strings.Repeat("word ", 100)}
		err := bootstrapExecutive(context.Background(), eng, "fake", PrepareOpts{
			ProjectDir: dir,
			UserPrompt: "build a todo app",
			Stdin:      stdin,
			Stdout:     &stdout,
			LogFunc:    testLogFunc(&logBuf),
		}, executivePath, "", "")

		require.NoError(t, err)
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "from user prompt (engine: fake, model:")
		assert.NotContains(t, logOutput, "assets/")
		assert.NotContains(t, logOutput, "media/")
	})
}

func TestParseOverviewSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected OverviewSummary
	}{
		{
			name: "well-formed output",
			input: `PROJECT_TYPE: Software (REST API)
GOAL: Build a todo app with PostgreSQL backend
EXPECTED_OUTPUT: Go binary with REST API, database migrations, Docker setup
KEY_TOPICS: REST API, PostgreSQL, authentication, Docker
EFFORT: standard (3-4 sprints)`,
			expected: OverviewSummary{
				ProjectType:    "Software (REST API)",
				Goal:           "Build a todo app with PostgreSQL backend",
				ExpectedOutput: "Go binary with REST API, database migrations, Docker setup",
				KeyTopics:      "REST API, PostgreSQL, authentication, Docker",
				EffortEstimate: "standard (3-4 sprints)",
			},
		},
		{
			name: "wrapped in markdown fences",
			input: "```\nPROJECT_TYPE: Writing (guide)\nGOAL: Write a Go concurrency guide\nEXPECTED_OUTPUT: 6 chapters in output/\nKEY_TOPICS: goroutines, channels\nEFFORT: standard (3 sprints)\n```",
			expected: OverviewSummary{
				ProjectType:    "Writing (guide)",
				Goal:           "Write a Go concurrency guide",
				ExpectedOutput: "6 chapters in output/",
				KeyTopics:      "goroutines, channels",
				EffortEstimate: "standard (3 sprints)",
			},
		},
		{
			name: "missing fields",
			input: `PROJECT_TYPE: Planning (analysis)
GOAL: Analyze market trends`,
			expected: OverviewSummary{
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
EFFORT: fast (1 sprint)`,
			expected: OverviewSummary{
				ProjectType:    "Software (CLI tool)",
				Goal:           "Build a CLI",
				ExpectedOutput: "binary",
				KeyTopics:      "cobra, testing",
				EffortEstimate: "fast (1 sprint)",
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: OverviewSummary{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseOverviewSummary(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDisplayOverviewSummary(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	displayOverviewSummary(&buf, OverviewSummary{
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

func TestDisplayOverviewSummary_UnknownFields(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	displayOverviewSummary(&buf, OverviewSummary{
		ProjectType: "Software (app)",
	})
	output := buf.String()
	assert.Contains(t, output, "Software (app)")
	assert.Contains(t, output, "(unknown)")
}

func TestRunProjectOverview_UserApproves(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software (CLI)\nGOAL: Build a CLI tool\nEXPECTED_OUTPUT: binary\nKEY_TOPICS: cobra\nEFFORT: low (1 sprint)"}
	stdin := strings.NewReader("y\n")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan content", "", "", "")

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Project summary")
	assert.Contains(t, stdout.String(), "Software (CLI)")
	assert.Equal(t, "", result.UserPrompt)
	assert.Equal(t, epic.EffortLevel(""), result.EffortLevel)
}

func TestRunProjectOverview_DefaultYes(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low"}
	stdin := strings.NewReader("\n") // empty line = default yes
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunProjectOverview_UserDeclines(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low"}
	stdin := strings.NewReader("n\n")
	var stdout strings.Builder

	_, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.ErrorIs(t, err, ErrProjectOverviewDeclined)
}

func TestRunProjectOverview_EOF(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low"}
	stdin := strings.NewReader("") // EOF
	var stdout strings.Builder

	_, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.ErrorIs(t, err, ErrProjectOverviewDeclined)
}

func TestSoftwareOverviewPrompt_IncludesInputs(t *testing.T) {
	t.Parallel()

	prompt := SoftwareOverviewPrompt("my plan", "my executive", "focus on backend", "standard", "media manifest", "assets section")
	assert.Contains(t, prompt, "senior software architect")
	assert.Contains(t, prompt, "my plan")
	assert.Contains(t, prompt, "my executive")
	assert.Contains(t, prompt, "focus on backend")
	assert.Contains(t, prompt, "media manifest")
	assert.Contains(t, prompt, "assets section")
	assert.Contains(t, prompt, `"standard"`)
}

func TestSoftwareOverviewPrompt_OmitsMissingInputs(t *testing.T) {
	t.Parallel()

	prompt := SoftwareOverviewPrompt("my plan", "", "", "", "", "")
	assert.Contains(t, prompt, "my plan")
	assert.NotContains(t, prompt, "Executive context")
	assert.NotContains(t, prompt, "User directive")
	assert.NotContains(t, prompt, "Media assets")
}

func TestPlanningOverviewPrompt(t *testing.T) {
	t.Parallel()

	prompt := PlanningOverviewPrompt("plan", "", "", "", "", "")
	assert.Contains(t, prompt, "senior strategic planner")
}

func TestWritingOverviewPrompt(t *testing.T) {
	t.Parallel()

	prompt := WritingOverviewPrompt("plan", "", "", "", "", "")
	assert.Contains(t, prompt, "senior author and content strategist")
}

func TestRunPrepare_Logging(t *testing.T) {
	t.Parallel()

	fakeFactory := func(name string) (engine.Engine, error) {
		return &fakeEngine{output: "1. Rule one\n@sprint 1\n@check_file foo\n"}, nil
	}

	t.Run("project overview skipped", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
		require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

		var logBuf strings.Builder

		err := RunPrepare(context.Background(), PrepareOpts{
			ProjectDir:      dir,
			Engine:          "claude",
			SkipProjectOverview: true,
			EngineFactory:   fakeFactory,
			LogFunc:         testLogFunc(&logBuf),
		})

		require.NoError(t, err)
		logOutput := logBuf.String()
		assert.NotContains(t, logOutput, "Project overview")
	})

	t.Run("plan exists without executive", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		// Create plan but NOT executive.
		require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
		require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

		var logBuf strings.Builder

		err := RunPrepare(context.Background(), PrepareOpts{
			ProjectDir:      dir,
			Engine:          "claude",
			SkipProjectOverview: true,
			EngineFactory:   fakeFactory,
			LogFunc:         testLogFunc(&logBuf),
		})

		require.NoError(t, err)
		logOutput := logBuf.String()
		// Executive doesn't exist — should not be mentioned.
		assert.NotContains(t, logOutput, "Using existing plans/executive.md")
		assert.Contains(t, logOutput, "Using existing plans/plan.md")
		// Step 1 should list only plan.md (no executive).
		assert.Contains(t, logOutput, "Step 1: Generating .fry/AGENTS.md from plans/plan.md (engine: claude, model:")
	})

	t.Run("logs existing files", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		// Create existing plan and executive files.
		require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
		require.NoError(t, os.WriteFile(dir+"/plans/executive.md", []byte("exec content"), 0o644))
		require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

		var logBuf strings.Builder

		err := RunPrepare(context.Background(), PrepareOpts{
			ProjectDir:      dir,
			Engine:          "claude",
			SkipProjectOverview: true,
			EngineFactory:   fakeFactory,
			LogFunc:         testLogFunc(&logBuf),
		})

		require.NoError(t, err)
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "Using existing plans/executive.md")
		assert.Contains(t, logOutput, "Using existing plans/plan.md")
	})

	t.Run("logs user prompt and assets", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		// Create existing plan and executive files.
		require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
		require.NoError(t, os.WriteFile(dir+"/plans/executive.md", []byte("exec content"), 0o644))
		require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

		// Create an assets directory with a file.
		require.NoError(t, os.MkdirAll(dir+"/assets", 0o755))
		require.NoError(t, os.WriteFile(dir+"/assets/spec.yaml", []byte("openapi: 3.0.0"), 0o644))

		var logBuf strings.Builder

		err := RunPrepare(context.Background(), PrepareOpts{
			ProjectDir:      dir,
			Engine:          "claude",
			UserPrompt:      "focus on backend",
			SkipProjectOverview: true,
			EngineFactory:   fakeFactory,
			LogFunc:         testLogFunc(&logBuf),
		})

		require.NoError(t, err)
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "User prompt detected")
		assert.Contains(t, logOutput, "Supplementary assets detected (1 file(s) in assets/)")
		assert.Contains(t, logOutput, "Step 2: Generating .fry/epic.md from plans/plan.md, .fry/AGENTS.md, user prompt, assets/ assets")
	})

	t.Run("logs user prompt source when provided", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		require.NoError(t, os.MkdirAll(dir+"/plans", 0o755))
		require.NoError(t, os.WriteFile(dir+"/plans/executive.md", []byte("exec content"), 0o644))
		require.NoError(t, os.WriteFile(dir+"/plans/plan.md", []byte(strings.Repeat("word ", 100)), 0o644))

		var logBuf strings.Builder

		err := RunPrepare(context.Background(), PrepareOpts{
			ProjectDir:       dir,
			Engine:           "claude",
			UserPrompt:       "focus on backend",
			UserPromptSource: "--user-prompt-file prompts/backend.txt",
			SkipProjectOverview:  true,
			EngineFactory:    fakeFactory,
			LogFunc:          testLogFunc(&logBuf),
		})

		require.NoError(t, err)
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "User prompt loaded from --user-prompt-file prompts/backend.txt")
		assert.NotContains(t, logOutput, "User prompt detected")
	})
}

func TestRunProjectOverview_AdjustAddsUserPrompt(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low (1-2 sprints)"}
	// First: adjust with text, keep effort, keep review default, then: approve
	stdin := strings.NewReader("a\nfocus on the backend\n\n\ny\n")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
	assert.Equal(t, "focus on the backend", result.UserPrompt)
	assert.Contains(t, stdout.String(), "adjust")
}

func TestRunProjectOverview_AdjustAppendsToExistingPrompt(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: medium (2-4 sprints)"}
	stdin := strings.NewReader("a\nalso add tests\n\n\ny\n")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir:  t.TempDir(),
		UserPrompt:  "build a REST API",
		EffortLevel: epic.EffortStandard,
		Stdin:       stdin,
		Stdout:      &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
	assert.Equal(t, "build a REST API\n\nalso add tests", result.UserPrompt)
	assert.Equal(t, epic.EffortStandard, result.EffortLevel)
}

func TestRunProjectOverview_AdjustChangesEffort(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: high (4-10 sprints)"}
	// Adjust: blank text, change effort to high, keep review default, then approve
	stdin := strings.NewReader("a\n\nhigh\n\ny\n")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir:  t.TempDir(),
		EffortLevel: epic.EffortFast,
		Stdin:       stdin,
		Stdout:      &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
	assert.Equal(t, epic.EffortHigh, result.EffortLevel)
}

func TestRunProjectOverview_AdjustInvalidEffortKeepsOld(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: medium (2-4 sprints)"}
	// Adjust: blank text, invalid effort, keep review default, then approve
	stdin := strings.NewReader("a\n\ngarbage\n\ny\n")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir:  t.TempDir(),
		EffortLevel: epic.EffortStandard,
		Stdin:       stdin,
		Stdout:      &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
	assert.Equal(t, epic.EffortStandard, result.EffortLevel)
	assert.Contains(t, stdout.String(), "Invalid effort level")
}

func TestRunProjectOverview_AdjustThenDecline(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low (1-2 sprints)"}
	// Adjust, then decline on the second prompt
	stdin := strings.NewReader("a\nadjust text\n\nn\n")
	var stdout strings.Builder

	_, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.ErrorIs(t, err, ErrProjectOverviewDeclined)
}

func TestRunProjectOverview_AdjustEOFDuringText(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low (1-2 sprints)"}
	// Choose adjust then EOF
	stdin := strings.NewReader("a\n")
	var stdout strings.Builder

	_, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.ErrorIs(t, err, ErrProjectOverviewDeclined)
}

func TestRunProjectOverview_AdjustEOFDuringEffort(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low (1-2 sprints)"}
	// Choose adjust, provide text, but EOF before effort answer
	stdin := strings.NewReader("a\nsome adjustment\n")
	var stdout strings.Builder

	_, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.ErrorIs(t, err, ErrProjectOverviewDeclined)
}

func TestRunProjectOverview_AdjustEnablesReview(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: high (4-10 sprints)"}
	// Adjust: blank text, keep effort (high), enable review, then approve
	stdin := strings.NewReader("a\n\n\ny\ny\n")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir:  t.TempDir(),
		EffortLevel: epic.EffortHigh,
		Stdin:       stdin,
		Stdout:      &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
	assert.True(t, result.EnableReview)
	assert.Contains(t, stdout.String(), "Enable sprint review?")
}

func TestRunProjectOverview_ReviewNotShownForLowEffort(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low (1-2 sprints)"}
	// Adjust: blank text, keep effort (low), then approve — no review prompt expected
	stdin := strings.NewReader("a\n\n\ny\n")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir:  t.TempDir(),
		EffortLevel: epic.EffortFast,
		Stdin:       stdin,
		Stdout:      &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
	assert.False(t, result.EnableReview)
	assert.NotContains(t, stdout.String(), "Enable sprint review?")
}

func TestRunProjectOverview_ReviewAutoEnabledForMaxEffort(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: max (4-10 sprints)"}
	// Adjust: blank text, keep effort (max), then approve — no review prompt, but auto-enabled
	stdin := strings.NewReader("a\n\n\ny\n")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir:  t.TempDir(),
		EffortLevel: epic.EffortMax,
		Stdin:       stdin,
		Stdout:      &stdout,
	}, "plan", "", "", "")

	require.NoError(t, err)
	assert.True(t, result.EnableReview)
	assert.NotContains(t, stdout.String(), "Enable sprint review?")
	assert.Contains(t, stdout.String(), "auto-enabled for max effort")
}

func TestSoftwareStep2Prompt_WithReviewEnabled(t *testing.T) {
	t.Parallel()

	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "", true, "", "")
	assert.Contains(t, prompt, "@review_between_sprints")
}

func TestRunProjectOverview_ReaderErrorNotSilenced(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software\nGOAL: test\nEXPECTED_OUTPUT: test\nKEY_TOPICS: test\nEFFORT: low (1-2 sprints)"}
	readErr := errors.New("disk read failed")
	stdin := &errReader{data: nil, err: readErr}
	var stdout strings.Builder

	_, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir: t.TempDir(),
		Stdin:      stdin,
		Stdout:     &stdout,
	}, "plan", "", "", "")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrProjectOverviewDeclined)
	assert.Contains(t, err.Error(), "disk read failed")
}

func TestRunProjectOverview_AutoAccept(t *testing.T) {
	t.Parallel()

	eng := &fakeEngine{output: "PROJECT_TYPE: Software (CLI)\nGOAL: Build a CLI tool\nEXPECTED_OUTPUT: binary\nKEY_TOPICS: cobra\nEFFORT: low (1 sprint)"}
	stdin := strings.NewReader("")
	var stdout strings.Builder

	result, err := runProjectOverview(context.Background(), eng, PrepareOpts{
		ProjectDir:  t.TempDir(),
		AutoAccept:  true,
		EffortLevel: epic.EffortStandard,
		Stdin:       stdin,
		Stdout:      &stdout,
	}, "plan content", "", "", "")

	require.NoError(t, err)
	assert.Equal(t, epic.EffortStandard, result.EffortLevel)
	assert.Equal(t, "", result.UserPrompt)
	assert.False(t, result.EnableReview)
	assert.Contains(t, stdout.String(), "auto-accepted")
}

func TestOverviewPrompt_SprintsNotHours(t *testing.T) {
	t.Parallel()

	prompt := SoftwareOverviewPrompt("my plan", "", "", "", "", "")
	assert.Contains(t, prompt, "sprint count")
	assert.Contains(t, prompt, "NEVER use hours or days")
}
