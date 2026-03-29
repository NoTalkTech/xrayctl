package service

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
)

// SetupXray 安装配置Xray
func SetupXray(cfg *config.Config) error {
	internal.PrintYellow("正在部署 Xray 核心...")

	// 安装Xray
	if !internal.CommandExists("xray") {
		_, err := internal.ExecCommand("bash", "-c", "curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh | bash -s -- install")
		if err != nil {
			internal.PrintRed("Xray安装失败: %v", err)
			return err
		}
	}

	// 提取现有UUID（如果已经有配置）
	var oldUUID string
	if internal.FileExists(cfg.XrayConfig) {
		data, err := internal.ReadFile(cfg.XrayConfig)
		if err == nil {
			var oldCfg map[string]interface{}
			if json.Unmarshal(data, &oldCfg) == nil {
				if inbounds, ok := oldCfg["inbounds"].([]interface{}); ok {
					if len(inbounds) > 0 {
						if firstInbound, ok := inbounds[0].(map[string]interface{}); ok {
							if settings, ok := firstInbound["settings"].(map[string]interface{}); ok {
								if clients, ok := settings["clients"].([]interface{}); ok {
									if len(clients) > 0 {
										if firstClient, ok := clients[0].(map[string]interface{}); ok {
											if id, ok := firstClient["id"].(string); ok {
												oldUUID = id
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// 生成UUID：优先使用配置文件中的UUID，然后是现有配置的UUID，最后生成新的
	uuid := cfg.UUID
	if uuid == "" {
		uuid = oldUUID
	}
	if uuid == "" {
		if cfg.Email != "" {
			uuid = generateUUIDFromEmail(cfg.Email)
		} else {
			uuid = internal.GenerateUUID()
		}
	}
	cfg.UUID = uuid
	config.SaveConfig(cfg)

	// 生成Xray配置JSON
	jsonConfig := buildXrayConfigJSON(cfg, uuid)

	// 写入配置文件
	err := internal.WriteFile(cfg.XrayConfig, []byte(jsonConfig), 0644)
	if err != nil {
		internal.PrintRed("写入Xray配置失败: %v", err)
		return err
	}

	// 重启Xray
	_, err = internal.ExecCommandWithSudo("systemctl", "restart", "xray")
	if err != nil {
		internal.PrintRed("Xray重启失败: %v", err)
		return err
	}

	// 设置开机自启
	internal.EnableService("xray")

	internal.PrintGreen("Xray部署成功！UUID: %s", uuid)
	return nil
}

// buildXrayConfigJSON 构建Xray配置JSON字符串
func buildXrayConfigJSON(cfg *config.Config, uuid string) string {
	var builder strings.Builder

	// 日志配置
	builder.WriteString(`{
  "log": { "loglevel": "warning" },
  "inbounds": [{
    "port": `)
	builder.WriteString(fmt.Sprintf("%d, ", cfg.XrayPort))
	builder.WriteString(`"protocol": "VLESS",
    "tag": "VLESS-XTLS-in",
    "settings": {
      "clients": [{
        "id": "`)
	builder.WriteString(uuid)
	builder.WriteString(`",
        "flow": "xtls-rprx-vision"
      }],
      "decryption": "none",
      "fallbacks": [{
        "dest": `)
	builder.WriteString(fmt.Sprintf("%d, ", cfg.NginxPort))
	builder.WriteString(`"xver": 1
      }]
    },
    "streamSettings": {
      "network": "raw",
      "security": "tls",
      "tlsSettings": {
        "rejectUnknownSni": true,
        "certificates": [{
          "certificateFile": "`)
	builder.WriteString(fmt.Sprintf("%s/xray.crt", cfg.CertDir))
	builder.WriteString(`",
          "keyFile": "`)
	builder.WriteString(fmt.Sprintf("%s/xray.key", cfg.CertDir))
	builder.WriteString(`"
        }]
      }
    }
  }],
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "direct"
    },
    {
      "protocol": "socks",
      "tag": "warp",
      "settings": {
        "servers": [{
          "address": "127.0.0.1",
          "port": `)
	builder.WriteString(fmt.Sprintf("%d", cfg.WARPPort))
	builder.WriteString(`
        }]
      }
    }
  ],
  "routing": {
    "domainStrategy": "AsIs",
    "rules": [`)

	// WARP分流规则
	for _, domain := range cfg.RouteDomains {
		builder.WriteString(`{
      "type": "field",
      "outboundTag": "warp",
      "domain": ["`)
		builder.WriteString(domain)
		builder.WriteString(`"]
    }, `)
	}

	// 默认直连规则
	builder.WriteString(`{
      "type": "field",
      "outboundTag": "direct",
      "inboundTag": ["VLESS-XTLS-in"]
      }
    ]
  }
}
`)

	return builder.String()
}

// XrayStatus 获取Xray运行状态
func XrayStatus() string {
	return internal.ServiceStatus("xray")
}

// generateUUIDFromEmail 基于email生成确定性UUID
func generateUUIDFromEmail(email string) string {
	baseUUID := internal.GenerateUUID()
	if email == "" {
		return baseUUID
	}
	// 使用email的MD5哈希作为UUID的后半部分，确保相同email生成相同UUID
	hash := md5.Sum([]byte(email))
	hashStr := fmt.Sprintf("%x", hash)
	// 替换baseUUID的尾部
	return baseUUID[:23] + hashStr[9:13]
}
