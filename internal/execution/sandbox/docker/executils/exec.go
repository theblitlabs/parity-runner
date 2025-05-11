package executils

import (
	"context"
	"os/exec"
)

// ExecCommand executes a command with context and returns its combined output
func ExecCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
