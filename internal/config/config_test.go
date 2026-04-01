package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yevgetman/fry/internal/config"
)

func TestFryDirPathConsistency(t *testing.T) {
	t.Parallel()

	// All .fry/ paths should be rooted under FryDir
	fryPaths := map[string]string{
		"ProjectConfigFile":       config.ProjectConfigFile,
		"BuildLogsDir":           config.BuildLogsDir,
		"DefaultVerificationFile": config.DefaultVerificationFile,
		"PromptFile":             config.PromptFile,
		"SprintProgressFile":     config.SprintProgressFile,
		"EpicProgressFile":       config.EpicProgressFile,
		"ReviewPromptFile":       config.ReviewPromptFile,
		"DeviationLogFile":       config.DeviationLogFile,
		"LockFile":               config.LockFile,
		"UserPromptFile":         config.UserPromptFile,
		"AgentsFile":             config.AgentsFile,
	}

	for name, path := range fryPaths {
		assert.True(t, strings.HasPrefix(path, config.FryDir+"/"),
			"%s (%q) should start with %q", name, path, config.FryDir+"/")
	}
}

func TestPlansDirPathConsistency(t *testing.T) {
	t.Parallel()

	plansPaths := map[string]string{
		"PlanFile":      config.PlanFile,
		"ExecutiveFile": config.ExecutiveFile,
	}

	for name, path := range plansPaths {
		assert.True(t, strings.HasPrefix(path, config.PlansDir+"/"),
			"%s (%q) should start with %q", name, path, config.PlansDir+"/")
	}
}

func TestInvocationPromptsNonEmpty(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, config.AgentInvocationPrompt)
	assert.NotEmpty(t, config.HealInvocationPrompt)
}
