package sprint

import (
	"context"
	"fmt"
	"strings"

	"github.com/yevgetman/fry/internal/engine"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/textutil"
)

const maxCompactionErrorOutputBytes = 2000

func CompactSprintProgress(ctx context.Context, projectDir string, sprintNum int, sprintName, status string, eng engine.Engine, useAgent bool, model, effortLevel string) (string, error) {
	progress, err := ReadSprintProgress(projectDir)
	if err != nil {
		return "", fmt.Errorf("read sprint progress for compaction: %w", err)
	}

	compacted := mechanicalCompaction(progress)
	if useAgent {
		frylog.Log("  Compacting sprint progress with agent...  model=%s", model)
		if eng == nil {
			return "", fmt.Errorf("compact sprint progress: engine is required for agent compaction")
		}
		prompt := "Summarize the sprint progress below in 3-8 lines. Focus on what was completed, what remains, and any important gotchas.\n\n" + progress
		output, exitCode, runErr := eng.Run(ctx, prompt, engine.RunOpts{
			Model:       model,
			SessionType: engine.SessionCompaction,
			EffortLevel: effortLevel,
			WorkDir:     projectDir,
		})
		if runErr != nil {
			details := summarizeCompactionFailureOutput(output)
			frylog.Log("  Compaction agent failed: engine=%s  model=%s  exit_code=%d  err=%v", eng.Name(), model, exitCode, runErr)
			if details != "" {
				frylog.Log("  Compaction agent output:\n%s", details)
			}
			msg := fmt.Sprintf("compact sprint progress with agent: engine=%s model=%s exit_code=%d: %v", eng.Name(), model, exitCode, runErr)
			if details != "" {
				msg += "\nagent output:\n" + details
			}
			return "", fmt.Errorf("%s", msg)
		}
		compacted = strings.TrimSpace(output)
	}

	return fmt.Sprintf("## Sprint %d: %s — %s\n\n%s\n", sprintNum, sprintName, status, strings.TrimSpace(compacted)), nil
}

func mechanicalCompaction(progress string) string {
	lines := strings.Split(progress, "\n")
	lastIndex := -1

	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "## Iteration") || strings.HasPrefix(lines[i], "--- Alignment attempt") {
			lastIndex = i
			break
		}
	}

	if lastIndex == -1 {
		return strings.TrimSpace(progress)
	}

	return strings.TrimSpace(strings.Join(lines[lastIndex:], "\n"))
}

func summarizeCompactionFailureOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > maxCompactionErrorOutputBytes {
		return textutil.TruncateUTF8(trimmed, maxCompactionErrorOutputBytes) + "\n...(truncated)"
	}
	return trimmed
}
