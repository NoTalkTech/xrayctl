package service

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestUninstallReturnsPackageAndCleanupErrors(t *testing.T) {
	aptErr := errors.New("apt failed")
	rmErr := errors.New("rm failed")
	var calls []string

	stubUninstall(t, func(name string) bool {
		return name == "apt"
	}, func(name string, args ...string) (string, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))

		switch name {
		case "apt":
			return "", aptErr
		case "rm":
			return "", rmErr
		default:
			return "", nil
		}
	})

	err := Uninstall()
	if err == nil {
		t.Fatal("Uninstall returned nil, want joined error")
	}

	for _, want := range []string{"apt uninstall", "remove residual files"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Uninstall error = %q, want %q", err, want)
		}
	}

	for _, want := range []string{
		"systemctl stop xray",
		"systemctl stop nginx",
		"systemctl stop warp-svc",
		"apt purge -y cloudflare-warp nginx",
			"apt purge -y xray",
		"rm -rf /etc/xray /usr/local/etc/xray /etc/xrayctl",
	} {
		if !containsCall(calls, want) {
			t.Fatalf("calls = %v, missing %q", calls, want)
		}
	}
}

func TestUninstallReturnsUnsupportedPackageManager(t *testing.T) {
	stubUninstall(t, func(string) bool {
		return false
	}, func(name string, args ...string) (string, error) {
		return "", nil
	})

	err := Uninstall()
	if err == nil {
		t.Fatal("Uninstall returned nil, want unsupported package manager error")
	}
	if !strings.Contains(err.Error(), "no supported package manager") {
		t.Fatalf("Uninstall error = %q, want unsupported package manager", err)
	}
}

func TestUninstallReturnsStopServiceErrors(t *testing.T) {
	stopErr := errors.New("stop failed")

	stubUninstall(t, func(name string) bool {
		return name == "apt"
	}, func(name string, args ...string) (string, error) {
		call := strings.Join(append([]string{name}, args...), " ")
		if call == "systemctl stop nginx" {
			return "", stopErr
		}

		return "", nil
	})

	err := Uninstall()
	if err == nil {
		t.Fatal("Uninstall returned nil, want stop service error")
	}
	if !strings.Contains(err.Error(), "stop nginx") {
		t.Fatalf("Uninstall error = %q, want stop nginx context", err)
	}
}

func stubUninstall(
	t *testing.T,
	commandExists func(string) bool,
	runWithSudo func(string, ...string) (string, error),
) {
	t.Helper()

	origCommandExists := uninstallCommandExists
	origRunWithSudo := uninstallRunWithSudo

	uninstallCommandExists = commandExists
	uninstallRunWithSudo = runWithSudo

	t.Cleanup(func() {
		uninstallCommandExists = origCommandExists
		uninstallRunWithSudo = origRunWithSudo
	})
}

func containsCall(calls []string, want string) bool {
	return slices.Contains(calls, want)
}
