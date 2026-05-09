package service

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"xrayctl/config"
)

func TestCollectEnvironmentReportReturnsStructuredState(t *testing.T) {
	cfg := &config.Config{
		CertDir:     "/tmp/xrayctl-test/cert",
		XrayConfig:  "/tmp/xrayctl-test/xray.json",
		NginxConfig: "/tmp/xrayctl-test/vless.conf",
	}
	certPath := filepath.Join(cfg.CertDir, "xray.crt")

	stubEnvironment(t, func(cmd string) bool {
		return map[string]bool{
			"curl":  true,
			"nginx": true,
		}[cmd]
	}, func(path string) bool {
		return map[string]bool{
			config.ConfigPath:   true,
			certPath:            true,
			nginxMainConfigPath: true,
		}[path]
	}, nil)

	report := CollectEnvironmentReport(cfg)

	if !slices.Equal(report.MissingCommands, []string{"jq", "systemctl"}) {
		t.Fatalf("MissingCommands = %v, want [jq systemctl]", report.MissingCommands)
	}

	if report.ConfigPath != config.ConfigPath || !report.ConfigExists {
		t.Errorf("config presence = (%q, %v), want (%q, true)", report.ConfigPath, report.ConfigExists, config.ConfigPath)
	}

	if report.CertPath != certPath || !report.CertExists {
		t.Errorf("cert presence = (%q, %v), want (%q, true)", report.CertPath, report.CertExists, certPath)
	}

	if report.XrayConfigPath != cfg.XrayConfig || report.XrayConfigExists {
		t.Errorf("xray presence = (%q, %v), want (%q, false)", report.XrayConfigPath, report.XrayConfigExists, cfg.XrayConfig)
	}

	if report.NginxMainConfigPath != nginxMainConfigPath || !report.NginxMainConfigExists {
		t.Errorf(
			"nginx main presence = (%q, %v), want (%q, true)",
			report.NginxMainConfigPath,
			report.NginxMainConfigExists,
			nginxMainConfigPath,
		)
	}

	if report.NginxVlessConfigPath != cfg.NginxConfig || report.NginxVlessConfigExists {
		t.Errorf(
			"nginx vless presence = (%q, %v), want (%q, false)",
			report.NginxVlessConfigPath,
			report.NginxVlessConfigExists,
			cfg.NginxConfig,
		)
	}
}

func TestEnvironmentReportValidationIncludesMissingCommands(t *testing.T) {
	report := EnvironmentReport{MissingCommands: []string{"jq", "systemctl"}}

	err := report.ValidationError()
	if err == nil {
		t.Fatal("ValidationError() = nil, want error")
	}

	for _, want := range []string{"missing required commands", "jq", "systemctl"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("validation error = %q, want %q", err, want)
		}
	}
}

func TestCheckSystemEnvironmentReturnsInstallError(t *testing.T) {
	installErr := errors.New("apt failed")
	installerCalled := false

	stubEnvironment(t, func(cmd string) bool {
		return cmd == "curl"
	}, func(string) bool {
		return false
	}, func() error {
		installerCalled = true

		return installErr
	})
	withInputStdin(t, "y\n")

	cfg := &config.Config{
		CertDir:     "/tmp/xrayctl-test/cert",
		XrayConfig:  "/tmp/xrayctl-test/xray.json",
		NginxConfig: "/tmp/xrayctl-test/vless.conf",
	}

	var (
		report EnvironmentReport
		err    error
	)

	output := captureStdout(t, func() {
		report, err = CheckSystemEnvironment(cfg)
	})

	if !installerCalled {
		t.Fatal("installer was not called")
	}

	if err == nil {
		t.Fatal("CheckSystemEnvironment returned nil error, want install error")
	}

	if !strings.Contains(err.Error(), "install missing dependencies") || !strings.Contains(err.Error(), installErr.Error()) {
		t.Fatalf("error = %q, want install context and cause", err)
	}

	if !slices.Equal(report.MissingCommands, []string{"jq", "nginx", "systemctl"}) {
		t.Fatalf("MissingCommands = %v, want [jq nginx systemctl]", report.MissingCommands)
	}

	for _, unexpected := range []string{"依赖安装完成，请重试", "所有依赖安装完成"} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("output contains misleading success message %q: %q", unexpected, output)
		}
	}
}

func TestApplySysctlSettingsIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sysctl.conf")
	initial := strings.Join([]string{
		"# existing config",
		"net.core.default_qdisc = pfifo_fast",
		"net.ipv4.ip_forward = 1",
		"net.core.default_qdisc = fq",
		"net.ipv4.tcp_congestion_control = cubic",
		"",
	}, "\n")

	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("write sysctl fixture: %v", err)
	}

	settings := []sysctlSetting{
		{Key: "net.core.default_qdisc", Value: "fq"},
		{Key: "net.ipv4.tcp_congestion_control", Value: "bbr"},
	}

	if err := applySysctlSettings(path, settings); err != nil {
		t.Fatalf("first applySysctlSettings failed: %v", err)
	}
	if err := applySysctlSettings(path, settings); err != nil {
		t.Fatalf("second applySysctlSettings failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sysctl result: %v", err)
	}

	result := string(data)
	for _, want := range []string{
		"# existing config",
		"net.ipv4.ip_forward = 1",
		"net.core.default_qdisc = fq",
		"net.ipv4.tcp_congestion_control = bbr",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("sysctl result missing %q:\n%s", want, result)
		}
	}

	if count := strings.Count(result, "net.core.default_qdisc"); count != 1 {
		t.Fatalf("default_qdisc count = %d, want 1:\n%s", count, result)
	}
	if count := strings.Count(result, "net.ipv4.tcp_congestion_control"); count != 1 {
		t.Fatalf("tcp_congestion_control count = %d, want 1:\n%s", count, result)
	}
}

func stubEnvironment(
	t *testing.T,
	commandExists func(string) bool,
	fileExists func(string) bool,
	installer func() error,
) {
	t.Helper()

	origCommandExists := environmentCommandExists
	origFileExists := environmentFileExists
	origInstaller := environmentBaseInstaller

	environmentCommandExists = commandExists
	environmentFileExists = fileExists

	if installer != nil {
		environmentBaseInstaller = installer
	}

	t.Cleanup(func() {
		environmentCommandExists = origCommandExists
		environmentFileExists = origFileExists
		environmentBaseInstaller = origInstaller
	})
}

func withInputStdin(t *testing.T, input string) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("write stdin pipe: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	origStdin := os.Stdin
	os.Stdin = r

	t.Cleanup(func() {
		os.Stdin = origStdin

		if err := r.Close(); err != nil {
			t.Errorf("close stdin reader: %v", err)
		}
	})
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	run()

	if err := w.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	os.Stdout = origStdout

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	return string(output)
}
