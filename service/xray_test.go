package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"xrayctl/config"
	"xrayctl/internal"
)

var uuidV4ShapeRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestBuildXrayConfigJSON(t *testing.T) {
	cfg := &config.Config{
		Domain:    "test.example.com",
		UUID:      "11111111-1111-4111-8111-111111111111",
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

func TestResolveXrayUUIDPriority(t *testing.T) {
	const (
		explicitUUID  = "11111111-1111-4111-8111-111111111111"
		existingUUID  = "22222222-2222-4222-8222-222222222222"
		generatedUUID = "33333333-3333-4333-8333-333333333333"
	)

	existingConfigPath := writeXrayConfigWithUUID(t, existingUUID)
	missingConfigPath := filepath.Join(t.TempDir(), "missing.json")

	tests := []struct {
		name          string
		cfg           config.Config
		wantUUID      string
		wantGenerated bool
	}{
		{
			name: "explicit config UUID wins over existing Xray config",
			cfg: config.Config{
				UUID:       explicitUUID,
				Email:      "user@example.com",
				XrayConfig: existingConfigPath,
			},
			wantUUID: explicitUUID,
		},
		{
			name: "existing Xray config UUID is preserved",
			cfg: config.Config{
				Email:      "user@example.com",
				XrayConfig: existingConfigPath,
			},
			wantUUID: existingUUID,
		},
		{
			name: "random UUID is generated only without explicit or existing UUID",
			cfg: config.Config{
				Email:      "user@example.com",
				XrayConfig: missingConfigPath,
			},
			wantUUID:      generatedUUID,
			wantGenerated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generated := false
			got := resolveXrayUUID(&tt.cfg, func() string {
				generated = true
				return generatedUUID
			})

			if got != tt.wantUUID {
				t.Errorf("resolveXrayUUID() = %q, want %q", got, tt.wantUUID)
			}
			if generated != tt.wantGenerated {
				t.Errorf("generator called = %t, want %t", generated, tt.wantGenerated)
			}
		})
	}
}

func TestResolveXrayUUIDGeneratesValidRandomUUID(t *testing.T) {
	cfg := &config.Config{
		Email:      "user@example.com",
		XrayConfig: filepath.Join(t.TempDir(), "missing.json"),
	}

	got := resolveXrayUUID(cfg, internal.GenerateUUID)
	if !uuidV4ShapeRE.MatchString(got) {
		t.Errorf("generated UUID = %q, want canonical UUID v4", got)
	}
}

func writeXrayConfigWithUUID(t *testing.T, uuid string) string {
	t.Helper()

	data, err := json.Marshal(XrayConfig{
		Inbounds: []XrayInbound{{
			Settings: XrayInboundSettings{
				Clients: []XrayClient{{ID: uuid}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("marshal Xray config fixture: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write Xray config fixture: %v", err)
	}

	return path
}
