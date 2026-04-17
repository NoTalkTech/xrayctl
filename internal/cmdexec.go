package internal

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// CommandRunner runs external commands. Production code uses DefaultRunner,
// which shells out via os/exec; tests can inject a fake that records calls.
//
// All methods MUST respect ctx cancellation so callers can bound long-running
// commands (warp-cli connect, acme.sh issuance, apt install, …).
type CommandRunner interface {
	// Run executes name with args and returns stdout on success or stderr on
	// failure. Progress is logged via the [CMD]/[STDOUT]/[STDERR] prefixes.
	Run(ctx context.Context, name string, args ...string) (string, error)

	// RunSilent is like Run but without logging — use for status probes
	// whose output is handled by the caller.
	RunSilent(ctx context.Context, name string, args ...string) (string, error)

	// RunWithSudo executes name with args, prefixing `sudo` when the current
	// process is not already root.
	RunWithSudo(ctx context.Context, name string, args ...string) (string, error)
}

// DefaultRunner is the runner used by the package-level Exec* helpers.
// Tests that want to stub shell-outs can swap this out (and restore it via
// defer) — see internal/cmdexec_test.go for the pattern.
var DefaultRunner CommandRunner = realRunner{}

type realRunner struct{}

func (realRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	fmt.Printf("[CMD] %s", name)
	for _, arg := range args {
		fmt.Printf(" %s", arg)
	}
	fmt.Println()

	err := cmd.Run()

	if stdout.Len() > 0 {
		fmt.Printf("[STDOUT] %s\n", stdout.String())
	}
	if err != nil {
		if stderr.Len() > 0 {
			fmt.Printf("[STDERR] %s\n", stderr.String())
		}
		return stderr.String(), err
	}
	return stdout.String(), nil
}

func (realRunner) RunSilent(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}

func (r realRunner) RunWithSudo(ctx context.Context, name string, args ...string) (string, error) {
	if IsRoot() {
		return r.Run(ctx, name, args...)
	}
	full := append([]string{name}, args...)
	return r.Run(ctx, "sudo", full...)
}

// ----------------------------------------------------------------------------
// Package-level helpers — preserved for existing call sites.
//
// These use context.Background() and delegate to DefaultRunner, so tests can
// still inject a fake. Prefer the CommandRunner interface directly in new code
// so callers can pass their own context for cancellation/timeouts.
// ----------------------------------------------------------------------------

// ExecCommand executes a shell command with stdout/stderr logging.
func ExecCommand(name string, args ...string) (string, error) {
	return DefaultRunner.Run(context.Background(), name, args...)
}

// ExecCommandSilent executes a shell command without logging.
func ExecCommandSilent(name string, args ...string) (string, error) {
	return DefaultRunner.RunSilent(context.Background(), name, args...)
}

// ExecCommandWithSudo executes a shell command with sudo if not already root.
func ExecCommandWithSudo(name string, args ...string) (string, error) {
	return DefaultRunner.RunWithSudo(context.Background(), name, args...)
}

// CommandExists reports whether name can be resolved in $PATH.
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
