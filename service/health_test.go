package service

import (
	"errors"
	"strings"
	"testing"

	"xrayctl/config"
	"xrayctl/internal"
)

func TestBuildStatusReportIncludesConnectionParameters(t *testing.T) {
	cfg := &config.Config{
		Domain:   "vpn.example.com",
		UUID:     "11111111-1111-4111-8111-111111111111",
		XrayPort: 443,
	}

	report := buildStatusReport(
		cfg,
		internal.StatusActive,
		internal.StatusActive,
		internal.StatusActive,
		"203.0.113.10",
		nil,
	)

	if !report.HasConnectionParameters() {
		t.Fatal("report should have connection parameters")
	}

	wantShareLink := "vless://11111111-1111-4111-8111-111111111111@vpn.example.com:443" +
		"?flow=xtls-rprx-vision&security=tls&type=tcp#Xray-WARP"
	if report.ShareLink != wantShareLink {
		t.Errorf("share link = %q, want %q", report.ShareLink, wantShareLink)
	}
	if report.Protocol != statusProtocolLabel {
		t.Errorf("protocol = %q, want %q", report.Protocol, statusProtocolLabel)
	}
	if err := report.ValidationError(); err != nil {
		t.Fatalf("ValidationError() = %v, want nil", err)
	}
}

func TestStatusReportValidationIncludesFailedWarpProbe(t *testing.T) {
	cfg := &config.Config{}
	probeErr := errors.New("proxy unavailable")

	report := buildStatusReport(
		cfg,
		internal.StatusActive,
		internal.StatusActive,
		internal.StatusActive,
		"",
		probeErr,
	)

	if !report.HasFailures() {
		t.Fatal("report should fail validation")
	}

	err := report.ValidationError()
	if err == nil {
		t.Fatal("ValidationError() = nil, want error")
	}
	if !strings.Contains(err.Error(), "warp ip probe: proxy unavailable") {
		t.Errorf("validation error = %q, want WARP probe detail", err)
	}
}

func TestStatusReportValidationIncludesDownServices(t *testing.T) {
	cfg := &config.Config{}

	report := buildStatusReport(
		cfg,
		internal.StatusInactive,
		internal.StatusFailed,
		"",
		"203.0.113.10",
		nil,
	)

	err := report.ValidationError()
	if err == nil {
		t.Fatal("ValidationError() = nil, want error")
	}

	errText := err.Error()
	for _, want := range []string{"nginx=inactive", "xray=failed", "warp-svc=unknown"} {
		if !strings.Contains(errText, want) {
			t.Errorf("validation error = %q, want %q", errText, want)
		}
	}
}
