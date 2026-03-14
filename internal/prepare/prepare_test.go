package prepare

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/engine"
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

	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "", "")
	assert.Contains(t, prompt, "/tmp/GENERATE_EPIC.md")
}

func TestPlanningStep2NoGenerateEpic(t *testing.T) {
	t.Parallel()

	prompt := PlanningStep2Prompt("plan", "agents", "/tmp/epic-example.md", "", "", "")
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

	prompt := SoftwareStep2Prompt("plan", "agents", "/tmp/epic-example.md", "/tmp/GENERATE_EPIC.md", "", "low", "")
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
	t.Parallel()

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
	}, executivePath, "")

	require.NoError(t, err)
	data, readErr := os.ReadFile(executivePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "Generated executive content")
	assert.Contains(t, stdout.String(), "Generated executive context")
}

func TestBootstrapExecutive_UserDeclines(t *testing.T) {
	t.Parallel()

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
	}, executivePath, "")

	require.ErrorIs(t, err, ErrUserDeclined)
	_, statErr := os.Stat(executivePath)
	assert.True(t, os.IsNotExist(statErr), "executive.md should not be created when user declines")
}
