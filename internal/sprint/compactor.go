package sprint

import (
	"context"
	"fmt"
	"strings"

	"github.com/yevgetman/fry/internal/engine"
	frylog "github.com/yevgetman/fry/internal/log"
)

func CompactSprintProgress(ctx context.Context, projectDir string, sprintNum int, sprintName, status string, eng engine.Engine, useAgent bool, model string) (string, error) {
	progress, err := ReadSprintProgress(projectDir)
	if err != nil {
		return "", fmt.Errorf("read sprint progress for compaction: %w", err)
	}

	compacted := mechanicalCompaction(progress)
	if useAgent {
		frylog.Log("  Compacting sprint progress with agent...")
		if eng == nil {
			return "", fmt.Errorf("compact sprint progress: engine is required for agent compaction")
		}
		prompt := "Summarize the sprint progress below in 3-8 lines. Focus on what was completed, what remains, and any important gotchas.\n\n" + progress
		output, _, runErr := eng.Run(ctx, prompt, engine.RunOpts{
			Model:   model,
			WorkDir: projectDir,
		})
		if runErr != nil {
			return "", fmt.Errorf("compact sprint progress with agent: %w", runErr)
		}
		compacted = strings.TrimSpace(output)
	}

	return fmt.Sprintf("## Sprint %d: %s — %s\n\n%s\n", sprintNum, sprintName, status, strings.TrimSpace(compacted)), nil
}

func mechanicalCompaction(progress string) string {
	lines := strings.Split(progress, "\n")
	lastIndex := -1

	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "## Iteration") || strings.HasPrefix(lines[i], "--- Heal attempt") {
			lastIndex = i
			break
		}
	}

	if lastIndex == -1 {
		return strings.TrimSpace(progress)
	}

	return strings.TrimSpace(strings.Join(lines[lastIndex:], "\n"))
}
