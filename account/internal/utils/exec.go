package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandResult captures stdout and stderr from an executed command.
type CommandResult struct {
	Stdout string
	Stderr string
}

// RunCommand executes the provided command with context awareness and
// returns the collected stdout/stderr output. Errors include contextual
// information as well as stderr to make troubleshooting easier.
func RunCommand(ctx context.Context, name string, args ...string) (*CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	result := &CommandResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String()}
	if err != nil {
		return result, fmt.Errorf("command %s %s failed: %w\n%s", name, strings.Join(args, " "), err, result.Stderr)
	}

	return result, nil
}

// RunPgDump executes pg_dump with the flags required for schema comparison
// and returns the textual dump. The pglogical schema is excluded to avoid
// touching logical replication internals.
func RunPgDump(ctx context.Context, dsn string) (string, error) {
	args := []string{
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		"--exclude-schema=pglogical",
		"--dbname", dsn,
	}

	result, err := RunCommand(ctx, "pg_dump", args...)
	if err != nil {
		return "", err
	}

	return result.Stdout, nil
}
