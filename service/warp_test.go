package service

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"xrayctl/internal"
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

func (r *recorder) run(name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.reply != nil {
		if out, ok := r.reply[name]; ok {
			return out, nil
		}
	}
	return "", nil
}

func (r *recorder) Run(_ context.Context, name string, args ...string) (string, error) {
	return r.run(name, args...)
}
func (r *recorder) RunSilent(_ context.Context, name string, args ...string) (string, error) {
	return r.run(name, args...)
}
func (r *recorder) RunWithSudo(_ context.Context, name string, args ...string) (string, error) {
	return r.run(name, args...)
}

// TestInstallWarpRPMArgv exercises installWarpRPM end-to-end with a fake
// runner, proving the shell-substitution bug is gone: the RHEL version is
// resolved via a separate rpm -E call and the result is embedded into the
// RPM URL from Go, not from a shell.
func TestInstallWarpRPMArgv(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skipf("Cloudflare 仅发布 x86_64 RPM；当前 GOARCH=%s 上 installWarpRPM 故意失败，跳过", runtime.GOARCH)
	}
	rec := &recorder{reply: map[string]string{"rpm": "9\n"}}
	orig := internal.DefaultRunner
	internal.DefaultRunner = rec
	t.Cleanup(func() { internal.DefaultRunner = orig })

	// First rpm call (probe) returns "9\n"; subsequent rpm call installs.
	// The recorder returns "9\n" for every rpm invocation, which is fine —
	// only the first rpm call's output is consulted.
	if err := installWarpRPM(); err != nil {
		t.Fatalf("installWarpRPM: %v", err)
	}

	if len(rec.calls) != 2 {
		t.Fatalf("calls = %d, want 2 (rpm -E, then rpm -ivh); got %v", len(rec.calls), rec.calls)
	}

	wantProbe := []string{"rpm", "-E", "%rhel"}
	if !argvEqual(rec.calls[0], wantProbe) {
		t.Errorf("probe argv = %v, want %v", rec.calls[0], wantProbe)
	}

	install := rec.calls[1]
	wantInstallPrefix := []string{"rpm", "-ivh"}
	if len(install) != 3 || !argvEqual(install[:2], wantInstallPrefix) {
		t.Fatalf("install argv = %v, want prefix %v", install, wantInstallPrefix)
	}
	if !strings.Contains(install[2], "el9") {
		t.Errorf("install URL %q should contain %q (resolved from rpm -E output)", install[2], "el9")
	}
	if strings.Contains(install[2], "$(") {
		t.Errorf("install URL %q still contains shell substitution", install[2])
	}
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
