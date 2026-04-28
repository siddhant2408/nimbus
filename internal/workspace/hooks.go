package workspace

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/siddhant2408/nimbus/internal/config"
)

// maxHookOutputBytes is the maximum number of bytes of combined stdout+stderr
// captured from a hook for logging purposes.
const maxHookOutputBytes = 2048

// runHook executes a shell script in the given workspace directory with a
// deadline derived from hooks.timeout_ms. It returns nil on success (exit 0)
// and an error on non-zero exit, timeout, or execution failure.
func runHook(ctx context.Context, cfg *config.Config, script, workspacePath, hookName string) error {
	if script == "" {
		return nil
	}

	timeoutMs := cfg.Hooks.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 60000
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	slog.Info("Running workspace hook",
		"hook", hookName,
		"workspace", workspacePath,
	)

	cmd := exec.CommandContext(ctx, "sh", "-lc", script)
	cmd.Dir = workspacePath

	// Capture combined stdout+stderr.
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()

	output := truncateOutput(buf.Bytes())

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			slog.Warn("Workspace hook timed out",
				"hook", hookName,
				"workspace", workspacePath,
				"timeout_ms", timeoutMs,
				"output", output,
			)
			return fmt.Errorf("workspace hook %s timed out after %dms", hookName, timeoutMs)
		}

		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		slog.Warn("Workspace hook failed",
			"hook", hookName,
			"workspace", workspacePath,
			"exit_code", exitCode,
			"output", output,
		)
		return fmt.Errorf("workspace hook %s failed with exit code %d", hookName, exitCode)
	}

	return nil
}

// truncateOutput returns output truncated to maxHookOutputBytes. If truncation
// occurs a "... (truncated)" suffix is appended.
func truncateOutput(b []byte) string {
	if len(b) <= maxHookOutputBytes {
		return string(b)
	}
	return string(b[:maxHookOutputBytes]) + "... (truncated)"
}
