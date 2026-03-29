package config

// 路径常量定义
const (
	// 配置相关
	DefaultConfigDir  = "/etc/xrayctl"
	DefaultConfigPath = DefaultConfigDir + "/config.yaml"

	// Xray相关
	DefaultXrayConfigPath = "/usr/local/etc/xray/config.json"
	DefaultCertDir        = "/usr/local/etc/xray/cert"

	// Nginx相关
	DefaultNginxConfigPath = "/etc/nginx/conf.d/vless.conf"

	// 日志相关
	DefaultLogDir = "/var/log/xrayctl"
)
