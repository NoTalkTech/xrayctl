package internal

import (
	"context"
	"runtime"
	"testing"
)

func TestCommandExists(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cmd"
	} else {
		cmd = "ls"
	}
	if !CommandExists(cmd) {
		t.Errorf("Command %s should exist", cmd)
	}
	if CommandExists("non_existent_command_1234") {
		t.Error("Non existent command should not exist")
	}
}

func TestExecCommand(t *testing.T) {
	output, err := ExecCommand("echo", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(output) == 0 {
		t.Error("Output should not be empty")
	}
}

// TestRunSilentReturnsStdoutOnNonZeroExit pins the contract that status
// probes rely on: systemctl-style tools emit their answer on stdout while
// exiting non-zero (exit 3 = inactive/failed). If RunSilent dropped stdout
// on error — as it used to — ServiceStatus would flatten "failed" into "".
func TestRunSilentReturnsStdoutOnNonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh-specific")
	}
	out, err := realRunner{}.RunSilent(context.Background(), "sh", "-c", "echo failed; exit 3")
	if err == nil {
		t.Errorf("expected non-nil error for exit 3, got nil")
	}
	if out != "failed\n" {
		t.Errorf("stdout = %q, want %q (must survive non-zero exit)", out, "failed\n")
	}
}

// fakeRunner records every call and returns canned responses.
type fakeRunner struct {
	calls [][]string
	reply string
	err   error
}

func (f *fakeRunner) record(name string, args []string) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.record(name, args)
	return f.reply, f.err
}

func (f *fakeRunner) RunSilent(ctx context.Context, name string, args ...string) (string, error) {
	return f.Run(ctx, name, args...)
}

func (f *fakeRunner) RunWithSudo(ctx context.Context, name string, args ...string) (string, error) {
	return f.Run(ctx, name, args...)
}

func TestDefaultRunnerIsInjectable(t *testing.T) {
	fake := &fakeRunner{reply: "active\n"}
	orig := DefaultRunner
	DefaultRunner = fake
	t.Cleanup(func() { DefaultRunner = orig })

	got, err := ExecCommandSilent("systemctl", "is-active", "nginx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "active\n" {
		t.Errorf("reply = %q, want %q", got, "active\n")
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(fake.calls))
	}
	want := []string{"systemctl", "is-active", "nginx"}
	for i, a := range want {
		if fake.calls[0][i] != a {
			t.Errorf("call arg %d = %q, want %q", i, fake.calls[0][i], a)
		}
	}
}
