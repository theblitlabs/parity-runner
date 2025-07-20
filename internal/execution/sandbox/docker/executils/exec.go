package executils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func ExecCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()

	// If there's an error, include the command and stderr for better debugging
	if err != nil {
		cmdStr := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
		return output, fmt.Errorf("command failed: %s\nOutput: %s\nError: %w", cmdStr, string(output), err)
	}

	return output, nil
}
