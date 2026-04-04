package preflight

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInferEnvVars_BashStyle(t *testing.T) {
	t.Parallel()

	vars := inferEnvVars("Set $SUPABASE_URL and ${HMAC_TOKEN_SECRET} in your env")
	assert.Contains(t, vars, "SUPABASE_URL")
	assert.Contains(t, vars, "HMAC_TOKEN_SECRET")
}

func TestInferEnvVars_NodeStyle(t *testing.T) {
	t.Parallel()

	vars := inferEnvVars("const url = process.env.DATABASE_URL")
	assert.Contains(t, vars, "DATABASE_URL")
}

func TestInferEnvVars_GoStyle(t *testing.T) {
	t.Parallel()

	vars := inferEnvVars(`key := os.Getenv("API_SECRET_KEY")`)
	assert.Contains(t, vars, "API_SECRET_KEY")
}

func TestInferEnvVars_IgnoresSystemVars(t *testing.T) {
	t.Parallel()

	vars := inferEnvVars("Use $HOME and $PATH and $CUSTOM_VAR")
	assert.NotContains(t, vars, "HOME")
	assert.NotContains(t, vars, "PATH")
	assert.Contains(t, vars, "CUSTOM_VAR")
}

func TestInferEnvVars_ProseList(t *testing.T) {
	t.Parallel()

	vars := inferEnvVars("Set env vars like SUPABASE_URL, SUPABASE_KEY, HMAC_SECRET")
	assert.Contains(t, vars, "SUPABASE_URL")
	assert.Contains(t, vars, "SUPABASE_KEY")
	assert.Contains(t, vars, "HMAC_SECRET")
}

func TestInferEnvVars_NoDuplicates(t *testing.T) {
	t.Parallel()

	vars := inferEnvVars("$MY_VAR and $MY_VAR and ${MY_VAR}")
	count := 0
	for _, v := range vars {
		if v == "MY_VAR" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestInferTools_Playwright(t *testing.T) {
	t.Parallel()

	tools := inferTools("Run Playwright tests for E2E coverage")
	assert.Contains(t, tools, "npx")
}

func TestInferTools_Redis(t *testing.T) {
	t.Parallel()

	tools := inferTools("The service requires Redis for caching")
	assert.Contains(t, tools, "redis-cli")
}

func TestInferTools_NoMatch(t *testing.T) {
	t.Parallel()

	tools := inferTools("Build the TypeScript frontend")
	assert.Empty(t, tools)
}

func TestRunSprintPreflight_NoPrerequisites(t *testing.T) {
	t.Parallel()

	result := RunSprintPreflight("Create a simple Go file")
	assert.Nil(t, result)
}

func TestRunSprintPreflight_DetectsDocker(t *testing.T) {
	t.Parallel()

	result := RunSprintPreflight("Use Docker to run the integration tests with testcontainers")
	require.NotNil(t, result)
	assert.True(t, result.DockerRequired)
}

func TestRunSprintPreflight_DetectsEnvVars(t *testing.T) {
	t.Parallel()

	// Use an env var name that is almost certainly not set
	result := RunSprintPreflight("Configure $FRY_TEST_NONEXISTENT_VAR_12345")
	require.NotNil(t, result)
	assert.Contains(t, result.MissingEnvVars, "FRY_TEST_NONEXISTENT_VAR_12345")
	assert.True(t, result.HasBlockers())
}

func TestSprintPreflightResult_Summary(t *testing.T) {
	t.Parallel()

	r := &SprintPreflightResult{
		MissingEnvVars: []string{"FOO", "BAR"},
		DockerRequired: true,
		DockerMissing:  true,
	}
	s := r.Summary()
	assert.Contains(t, s, "missing env vars: FOO, BAR")
	assert.Contains(t, s, "Docker not available")
}

func TestSprintPreflightResult_NoBlockers(t *testing.T) {
	t.Parallel()

	r := &SprintPreflightResult{DockerRequired: true, DockerMissing: false}
	assert.False(t, r.HasBlockers())
}
