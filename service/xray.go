package service

import (
	"encoding/json"
	"fmt"

	"xrayctl/config"
	"xrayctl/internal"
)

// XrayConfig is the top-level Xray JSON schema subset this project emits.
type XrayConfig struct {
	Log       XrayLog        `json:"log"`
	Inbounds  []XrayInbound  `json:"inbounds"`
	Outbounds []XrayOutbound `json:"outbounds"`
	Routing   XrayRouting    `json:"routing"`
}

// XrayLog defines the log configuration for Xray.
type XrayLog struct {
	Loglevel string `json:"loglevel"`
}

// XrayInbound defines an inbound connection configuration.
type XrayInbound struct {
	Port           int                 `json:"port"`
	Protocol       string              `json:"protocol"`
	Tag            string              `json:"tag"`
	Settings       XrayInboundSettings `json:"settings"`
	StreamSettings XrayStreamSettings  `json:"streamSettings"`
}

// XrayInboundSettings defines the settings for an inbound connection.
type XrayInboundSettings struct {
	Clients    []XrayClient   `json:"clients"`
	Decryption string         `json:"decryption"`
	Fallbacks  []XrayFallback `json:"fallbacks"`
}

// XrayClient defines a VLESS client with ID and flow.
type XrayClient struct {
	ID   string `json:"id"`
	Flow string `json:"flow"`
}

// XrayFallback defines a fallback destination for VLESS.
type XrayFallback struct {
	Dest string `json:"dest"`
	Xver int    `json:"xver"`
}

// XrayStreamSettings defines stream transport settings.
type XrayStreamSettings struct {
	Network     string          `json:"network"`
	Security    string          `json:"security"`
	TLSSettings XrayTLSSettings `json:"tlsSettings"`
}

// XrayTLSSettings defines TLS settings for Xray stream.
type XrayTLSSettings struct {
	RejectUnknownSni bool              `json:"rejectUnknownSni"`
	Certificates     []XrayCertificate `json:"certificates"`
}

// XrayCertificate defines a TLS certificate entry.
type XrayCertificate struct {
	CertificateFile string `json:"certificateFile"`
	KeyFile         string `json:"keyFile"`
}

// XrayOutbound defines an outbound connection configuration.
type XrayOutbound struct {
	Protocol string                `json:"protocol"`
	Tag      string                `json:"tag"`
	Settings *XrayOutboundSettings `json:"settings,omitempty"`
}

// XrayOutboundSettings defines settings for an outbound connection.
type XrayOutboundSettings struct {
	Servers []XrayOutboundServer `json:"servers,omitempty"`
}

// XrayOutboundServer defines a SOCKS outbound server entry.
type XrayOutboundServer struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// XrayRouting defines the routing configuration.
type XrayRouting struct {
	DomainStrategy string            `json:"domainStrategy"`
	Rules          []XrayRoutingRule `json:"rules"`
}

// XrayRoutingRule defines a single routing rule.
type XrayRoutingRule struct {
	Type        string   `json:"type"`
	OutboundTag string   `json:"outboundTag"`
	Domain      []string `json:"domain,omitempty"`
	InboundTag  []string `json:"inboundTag,omitempty"`
}

const xrayInboundTag = "VLESS-XTLS-in"

// SetupXray 安装配置Xray.
func SetupXray(cfg *config.Config) error {
	internal.PrintYellow("正在部署 Xray 核心...")

	if err := internal.MkdirIfNotExists(cfg.CertDir, 0o755); err != nil {
		internal.PrintYellow("创建证书目录失败: %v", err)
	}

	if !internal.CommandExists("xray") {
		// The upstream installer is distributed as a script stream into bash; keep
		// this shell-backed while simple package/service calls use argv directly.
		_, err := internal.ExecCommand("bash", "-c",
			"curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh | bash -s -- install")
		if err != nil {
			internal.PrintRed("Xray安装失败: %v", err)
			return err
		}
	}

	uuid := resolveXrayUUID(cfg, internal.GenerateUUID)
	cfg.UUID = uuid

	if err := config.SaveConfig(cfg); err != nil {
		internal.PrintYellow("保存配置失败（UUID 可能未持久化）: %v", err)
	}

	jsonConfig, err := buildXrayConfigJSON(cfg, uuid)
	if err != nil {
		internal.PrintRed("生成Xray配置失败: %v", err)
		return err
	}

	// 0o600: config embeds the VLESS UUID (effectively a credential). This
	// project already assumes Xray runs as root — cert.go chmods xray.key
	// to 0o600 root:root, which would otherwise be unreadable to a nobody
	// worker — so locking the config the same way adds no new constraint.
	if err := internal.WriteFile(cfg.XrayConfig, []byte(jsonConfig), 0o600); err != nil {
		internal.PrintRed("写入Xray配置失败: %v", err)
		return err
	}

	if _, err := internal.ExecCommandWithSudo("systemctl", "restart", internal.ServiceXray); err != nil {
		internal.PrintRed("Xray重启失败: %v", err)
		return err
	}

	internal.EnableService(internal.ServiceXray)
	internal.PrintGreen("Xray部署成功！UUID: %s", uuid)

	return nil
}

// extractExistingUUID reads the first client UUID out of an on-disk Xray config,
// returning "" if the file is missing, unreadable, or has no client.
func extractExistingUUID(path string) string {
	if !internal.FileExists(path) {
		return ""
	}

	data, err := internal.ReadFile(path)
	if err != nil {
		return ""
	}

	var parsed XrayConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ""
	}

	if len(parsed.Inbounds) == 0 || len(parsed.Inbounds[0].Settings.Clients) == 0 {
		return ""
	}

	return parsed.Inbounds[0].Settings.Clients[0].ID
}

func resolveXrayUUID(cfg *config.Config, generateUUID func() string) string {
	if cfg.UUID != "" {
		return cfg.UUID
	}

	if uuid := extractExistingUUID(cfg.XrayConfig); uuid != "" {
		return uuid
	}

	return generateUUID()
}

// buildXrayConfigJSON builds the Xray configuration JSON text for cfg+uuid.
func buildXrayConfigJSON(cfg *config.Config, uuid string) (string, error) {
	xc := XrayConfig{
		Log: XrayLog{Loglevel: "warning"},
		Inbounds: []XrayInbound{{
			Port:     cfg.XrayPort,
			Protocol: internal.ProtocolVLESS,
			Tag:      xrayInboundTag,
			Settings: XrayInboundSettings{
				Clients:    []XrayClient{{ID: uuid, Flow: internal.FlowXTLSVision}},
				Decryption: "none",
				Fallbacks: []XrayFallback{{
					Dest: fmt.Sprintf("127.0.0.1:%d", cfg.NginxPort),
					Xver: 1,
				}},
			},
			StreamSettings: XrayStreamSettings{
				Network:  internal.NetworkRaw,
				Security: internal.ProtocolTLS,
				TLSSettings: XrayTLSSettings{
					RejectUnknownSni: true,
					Certificates: []XrayCertificate{{
						CertificateFile: fmt.Sprintf("%s/xray.crt", cfg.CertDir),
						KeyFile:         fmt.Sprintf("%s/xray.key", cfg.CertDir),
					}},
				},
			},
		}},
		Outbounds: []XrayOutbound{
			{Protocol: internal.ProtocolFreedom, Tag: "direct"},
			{
				Protocol: internal.ProtocolSOCKS,
				Tag:      "warp",
				Settings: &XrayOutboundSettings{
					Servers: []XrayOutboundServer{{Address: "127.0.0.1", Port: cfg.WARPPort}},
				},
			},
		},
		Routing: XrayRouting{
			DomainStrategy: "AsIs",
			Rules:          buildXrayRoutingRules(cfg.RouteDomains),
		},
	}

	data, err := json.MarshalIndent(xc, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data) + "\n", nil
}

func buildXrayRoutingRules(warpDomains []string) []XrayRoutingRule {
	rules := make([]XrayRoutingRule, 0, len(warpDomains)+1)

	for _, d := range warpDomains {
		rules = append(rules, XrayRoutingRule{
			Type:        "field",
			OutboundTag: "warp",
			Domain:      []string{d},
		})
	}

	rules = append(rules, XrayRoutingRule{
		Type:        "field",
		OutboundTag: "direct",
		InboundTag:  []string{xrayInboundTag},
	})

	return rules
}

// XrayStatus 获取Xray运行状态.
func XrayStatus() string {
	return internal.ServiceStatus(internal.ServiceXray)
}
