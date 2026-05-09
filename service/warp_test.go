package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"xrayctl/config"
)

func TestWarpAptRepoLine(t *testing.T) {
	got := warpAptRepoLine("bookworm")
	want := "deb [signed-by=/usr/share/keyrings/cloudflare-warp.gpg] https://pkg.cloudflareclient.com/ bookworm main\n"
	if got != want {
		t.Errorf("warpAptRepoLine = %q, want %q", got, want)
	}
}

func TestWarpRPMURL(t *testing.T) {
	cases := map[string]string{
		"8": "https://pkg.cloudflareclient.com/pool/cloudflare-warp-el8.x86_64.rpm",
		"9": "https://pkg.cloudflareclient.com/pool/cloudflare-warp-el9.x86_64.rpm",
	}
	for in, want := range cases {
		got, err := warpRPMURL(in, "amd64")
		if err != nil {
			t.Errorf("warpRPMURL(%q, amd64): unexpected error %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("warpRPMURL(%q, amd64) = %q, want %q", in, got, want)
		}
	}
}

func TestWarpRPMURLUnsupportedArch(t *testing.T) {
	if _, err := warpRPMURL("9", "arm64"); err == nil {
		t.Error("warpRPMURL(_, arm64) should return error: Cloudflare publishes no aarch64 RPM")
	}
}

// recorder is a minimal CommandRunner that captures argv for assertions.
type recorder struct {
	calls [][]string
	reply map[string]string // keyed on argv[0] for canned stdout
}

func (r *recorder) run(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r.reply != nil {
		if out, ok := r.reply[name]; ok {
			return out, nil
		}
	}
	return "", nil
}

func (r *recorder) Run(ctx context.Context, name string, args ...string) (string, error) {
	return r.run(ctx, name, args...)
}
func (r *recorder) RunSilent(ctx context.Context, name string, args ...string) (string, error) {
	return r.run(ctx, name, args...)
}
func (r *recorder) RunWithSudo(ctx context.Context, name string, args ...string) (string, error) {
	return r.run(ctx, name, args...)
}

// TestInstallWarpRHELArgv exercises the shared RHEL installer with a fake
// runner, proving the RHEL version is resolved via a separate rpm -E call and
// the result is embedded into the RPM URL from Go, not from a shell.
func TestInstallWarpRHELArgv(t *testing.T) {
	stubWarpGOARCH(t, "amd64")
	rec := &recorder{reply: map[string]string{"rpm": "9\n"}}

	// First rpm call (probe) returns "9\n"; subsequent rpm call installs.
	// The explicit runner parameter keeps the shell-out fake local to this test.
	// The recorder returns "9\n" for every rpm invocation, which is fine —
	// only the first rpm call's output is consulted.
	if err := installWarpRHEL(context.Background(), rec, pkgManagerDNF); err != nil {
		t.Fatalf("installWarpRHEL: %v", err)
	}

	if len(rec.calls) != 2 {
		t.Fatalf("calls = %d, want 2 (rpm -E, then dnf install); got %v", len(rec.calls), rec.calls)
	}

	wantProbe := []string{"rpm", "-E", "%rhel"}
	if !argvEqual(rec.calls[0], wantProbe) {
		t.Errorf("probe argv = %v, want %v", rec.calls[0], wantProbe)
	}

	install := rec.calls[1]
	wantInstallPrefix := []string{"dnf", "install", "-y"}
	if len(install) != 4 || !argvEqual(install[:3], wantInstallPrefix) {
		t.Fatalf("install argv = %v, want prefix %v", install, wantInstallPrefix)
	}
	if !strings.Contains(install[3], "el9") {
		t.Errorf("install URL %q should contain %q (resolved from rpm -E output)", install[3], "el9")
	}
	if strings.Contains(install[3], "$(") {
		t.Errorf("install URL %q still contains shell substitution", install[3])
	}
}

func TestInstallWarpDispatchesRHELPackageManagers(t *testing.T) {
	for _, tc := range []struct {
		name       string
		pkgManager string
	}{
		{name: "yum", pkgManager: pkgManagerYUM},
		{name: "dnf", pkgManager: pkgManagerDNF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubWarpGOARCH(t, "amd64")
			stubWarpCommandExists(t, func(name string) bool {
				return name == tc.pkgManager
			})

			rec := &recorder{reply: map[string]string{"rpm": "8\n"}}
			if err := installWarp(context.Background(), rec); err != nil {
				t.Fatalf("installWarp: %v", err)
			}

			wantURL := "https://pkg.cloudflareclient.com/pool/cloudflare-warp-el8.x86_64.rpm"
			if !hasCall(rec.calls, []string{"rpm", "-E", "%rhel"}) {
				t.Fatalf("calls = %v, want RHEL version probe", rec.calls)
			}
			if !hasCall(rec.calls, []string{tc.pkgManager, "install", "-y", wantURL}) {
				t.Fatalf("calls = %v, want %s install path", rec.calls, tc.pkgManager)
			}
			if hasCall(rec.calls, []string{pkgManagerAPT, "update"}) {
				t.Fatalf("calls = %v, should not run apt path for %s", rec.calls, tc.pkgManager)
			}
		})
	}
}

func TestSetupWarpPropagatesContextCancellation(t *testing.T) {
	stubWarpCommandExists(t, func(name string) bool {
		return name == "warp-cli"
	})

	ctx, cancel := context.WithCancel(context.Background())
	runner := &cancelingWarpRunner{cancel: cancel}

	err := setupWarp(ctx, &config.Config{WARPPort: 40000}, runner)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("setupWarp error = %v, want context.Canceled", err)
	}

	if !hasCall(runner.calls, []string{"warp-cli", "mode", "proxy"}) {
		t.Fatalf("calls = %v, want warp-cli mode proxy to receive cancellable context", runner.calls)
	}

	if hasCall(runner.calls, []string{"warp-cli", "connect"}) {
		t.Fatalf("calls = %v, should stop before warp-cli connect after cancellation", runner.calls)
	}
}

type cancelingWarpRunner struct {
	calls  [][]string
	cancel context.CancelFunc
}

func (r *cancelingWarpRunner) record(name string, args ...string) {
	r.calls = append(r.calls, append([]string{name}, args...))
}

func (r *cancelingWarpRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	r.record(name, args...)

	if name == "warp-cli" && argvEqual(append([]string{name}, args...), []string{"warp-cli", "mode", "proxy"}) {
		r.cancel()

		return "", ctx.Err()
	}

	return "", ctx.Err()
}

func (r *cancelingWarpRunner) RunSilent(ctx context.Context, name string, args ...string) (string, error) {
	r.record(name, args...)

	if argvEqual(append([]string{name}, args...), []string{"systemctl", "is-active", "warp-svc"}) {
		return "active\n", ctx.Err()
	}

	return "", ctx.Err()
}

func (r *cancelingWarpRunner) RunWithSudo(ctx context.Context, name string, args ...string) (string, error) {
	r.record(name, args...)

	return "", ctx.Err()
}

func stubWarpCommandExists(t *testing.T, exists func(string) bool) {
	t.Helper()

	orig := warpCommandExists
	warpCommandExists = exists

	t.Cleanup(func() {
		warpCommandExists = orig
	})
}

func stubWarpGOARCH(t *testing.T, goarch string) {
	t.Helper()

	orig := warpGOARCH
	warpGOARCH = goarch

	t.Cleanup(func() {
		warpGOARCH = orig
	})
}

func hasCall(calls [][]string, want []string) bool {
	for _, call := range calls {
		if argvEqual(call, want) {
			return true
		}
	}

	return false
}

func argvEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
