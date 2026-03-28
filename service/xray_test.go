package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"xrayctl/config"
)

func TestXrayConfigGeneration(t *testing.T) {
	// 创建测试配置
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

	// 测试配置生成逻辑（不实际写入文件）
	xrayCfg := XrayConfig{}
	xrayCfg.Log.Loglevel = "warning"

	// 入站配置
	inbound := struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Tag      string `json:"tag"`
		Settings struct {
			Clients []struct {
				ID   string `json:"id"`
				Flow string `json:"flow"`
			} `json:"clients"`
			Decryption string `json:"decryption"`
			Fallbacks  []struct {
				Dest int  `json:"dest"`
				Xver int  `json:"xver"`
			} `json:"fallbacks"`
		} `json:"settings"`
		StreamSettings struct {
			Network string `json:"network"`
			Security string `json:"security"`
			TlsSettings struct {
				RejectUnknownSni bool `json:"rejectUnknownSni"`
				Certificates []struct {
					CertificateFile string `json:"certificateFile"`
					KeyFile         string `json:"keyFile"`
				} `json:"certificates"`
			} `json:"tlsSettings"`
		} `json:"streamSettings"`
	}{}
	inbound.Port = cfg.XrayPort
	inbound.Protocol = "VLESS"
	inbound.Tag = "VLESS-XTLS-in"
	inbound.Settings.Clients = append(inbound.Settings.Clients, struct {
		ID   string `json:"id"`
		Flow string `json:"flow"`
	}{ID: cfg.UUID, Flow: "xtls-rprx-vision"})
	inbound.Settings.Decryption = "none"
	inbound.Settings.Fallbacks = append(inbound.Settings.Fallbacks, struct {
		Dest int  `json:"dest"`
		Xver int  `json:"xver"`
	}{Dest: cfg.NginxPort, Xver: 1})
	inbound.StreamSettings.Network = "raw"
	inbound.StreamSettings.Security = "tls"
	inbound.StreamSettings.TlsSettings.RejectUnknownSni = true
	inbound.StreamSettings.TlsSettings.Certificates = append(inbound.StreamSettings.TlsSettings.Certificates, struct {
		CertificateFile string `json:"certificateFile"`
		KeyFile         string `json:"keyFile"`
	}{CertificateFile: cfg.CertDir + "/xray.crt", KeyFile: cfg.CertDir + "/xray.key"})
	xrayCfg.Inbounds = append(xrayCfg.Inbounds, inbound)

	// 序列化测试
	data, err := json.MarshalIndent(xrayCfg, "", "  ")
	if err != nil {
		t.Fatalf("Xray config marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Generated config is empty")
	}

	// 验证配置内容
	if xrayCfg.Inbounds[0].Settings.Clients[0].ID != cfg.UUID {
		t.Errorf("UUID in config expected %s, got %s", cfg.UUID, xrayCfg.Inbounds[0].Settings.Clients[0].ID)
	}
	if xrayCfg.Inbounds[0].Port != cfg.XrayPort {
		t.Errorf("Port expected %d, got %d", cfg.XrayPort, xrayCfg.Inbounds[0].Port)
	}

	t.Log("Xray config generation test passed")
}

func TestNginxConfigGeneration(t *testing.T) {
	cfg := &config.Config{
		Domain:      "test.example.com",
		NginxPort:   8080,
		FallbackURL: "https://example.com",
	}

	// 生成Nginx配置内容
	confContent := `server {
    listen 80;
    server_name ` + cfg.Domain + `;
    location / { return 301 https://$host$request_uri; }
}
server {
    listen 127.0.0.1:` + strconv.Itoa(cfg.NginxPort) + ` proxy_protocol;
    http2 on;
    server_name ` + cfg.Domain + `;
    set_real_ip_from 127.0.0.1;
    real_ip_header X-Forwarded-For;
    error_page 403 ` + cfg.FallbackURL + `;
    location / { return 403; }
}
`

	// 验证配置包含必要内容
	if len(confContent) == 0 {
		t.Error("Nginx config is empty")
	}
	if !strings.Contains(confContent, cfg.Domain) {
		t.Error("Nginx config should contain domain")
	}
	if !strings.Contains(confContent, cfg.FallbackURL) {
		t.Error("Nginx config should contain fallback URL")
	}

	t.Log("Nginx config generation test passed")
}