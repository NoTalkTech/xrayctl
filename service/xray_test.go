package service

import (
	"encoding/json"
	"strings"
	"testing"

	"xrayctl/config"
)

func TestBuildXrayConfigJSON(t *testing.T) {
	cfg := &config.Config{
		Domain:    "test.example.com",
		UUID:      "test-uuid-1234-5678",
		CertDir:   "/tmp/cert",
		WARPPort:  40000,
		XrayPort:  443,
		NginxPort: 8080,
		RouteDomains: []string{
			"chatgpt.com",
			"openai.com",
		},
	}

	raw, err := buildXrayConfigJSON(cfg, cfg.UUID)
	if err != nil {
		t.Fatalf("buildXrayConfigJSON failed: %v", err)
	}

	var parsed XrayConfig
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("generated JSON does not round-trip: %v", err)
	}

	if parsed.Log.Loglevel != "warning" {
		t.Errorf("log level = %q, want warning", parsed.Log.Loglevel)
	}

	if len(parsed.Inbounds) != 1 {
		t.Fatalf("inbounds = %d, want 1", len(parsed.Inbounds))
	}
	in := parsed.Inbounds[0]
	if in.Port != cfg.XrayPort {
		t.Errorf("inbound port = %d, want %d", in.Port, cfg.XrayPort)
	}
	if len(in.Settings.Clients) != 1 || in.Settings.Clients[0].ID != cfg.UUID {
		t.Errorf("inbound client UUID mismatch: %+v", in.Settings.Clients)
	}
	if len(in.Settings.Fallbacks) != 1 || in.Settings.Fallbacks[0].Dest != "127.0.0.1:8080" {
		t.Errorf("fallback dest mismatch: %+v", in.Settings.Fallbacks)
	}
	if !in.StreamSettings.TLSSettings.RejectUnknownSni {
		t.Error("rejectUnknownSni should be true")
	}

	if len(parsed.Outbounds) != 2 {
		t.Fatalf("outbounds = %d, want 2", len(parsed.Outbounds))
	}
	warpOut := parsed.Outbounds[1]
	if warpOut.Tag != "warp" || warpOut.Settings == nil ||
		len(warpOut.Settings.Servers) != 1 ||
		warpOut.Settings.Servers[0].Port != cfg.WARPPort {
		t.Errorf("warp outbound malformed: %+v", warpOut)
	}

	// Every RouteDomain produces a warp rule, plus one trailing direct rule.
	if got, want := len(parsed.Routing.Rules), len(cfg.RouteDomains)+1; got != want {
		t.Errorf("routing rules = %d, want %d", got, want)
	}
	last := parsed.Routing.Rules[len(parsed.Routing.Rules)-1]
	if last.OutboundTag != "direct" || len(last.InboundTag) != 1 || last.InboundTag[0] != xrayInboundTag {
		t.Errorf("last routing rule should be direct fallback, got %+v", last)
	}
}

func TestGenerateUUIDFromEmailIsValid(t *testing.T) {
	u := generateUUIDFromEmail("user@example.com")
	if len(u) != 36 {
		t.Errorf("UUID length = %d, want 36 (%q)", len(u), u)
	}
	// Deterministic: same email → same UUID.
	if u2 := generateUUIDFromEmail("user@example.com"); u != u2 {
		t.Errorf("UUID not deterministic: %q vs %q", u, u2)
	}
	// Different email → different UUID (collisions are astronomically unlikely).
	if u3 := generateUUIDFromEmail("other@example.com"); u == u3 {
		t.Errorf("UUID collision across distinct emails: %q", u)
	}
	// Shape check: 8-4-4-4-12.
	parts := strings.Split(u, "-")
	wantLens := []int{8, 4, 4, 4, 12}
	if len(parts) != 5 {
		t.Fatalf("UUID parts = %d, want 5 (%q)", len(parts), u)
	}
	for i, p := range parts {
		if len(p) != wantLens[i] {
			t.Errorf("UUID part %d len = %d, want %d (%q)", i, len(p), wantLens[i], u)
		}
	}
}
