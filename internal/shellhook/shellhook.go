package shellhook

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes a shell command via bash. It returns an error that includes
// the command's output for debuggability.
func Run(ctx context.Context, projectDir, command string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run %q: %w\n%s", command, err, strings.TrimSpace(string(output)))
	}
	return nil
}
