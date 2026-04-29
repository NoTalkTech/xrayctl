// Package config handles YAML configuration persistence for xrayctl.
package config

// Config 全局配置结构.
type Config struct {
	// 基础配置
	Domain string `yaml:"domain"`
	UUID   string `yaml:"uuid"`
	Email  string `yaml:"email"` // 证书申请用邮箱

	// 路径配置
	CertDir     string `default:"/usr/local/etc/xray/cert"        yaml:"cert_dir"`
	XrayConfig  string `default:"/usr/local/etc/xray/config.json" yaml:"xray_config"`
	NginxConfig string `default:"/etc/nginx/conf.d/vless.conf"    yaml:"nginx_config"`

	// 端口配置
	WARPPort  int `default:"40000" yaml:"warp_port"`
	XrayPort  int `default:"443"   yaml:"xray_port"`
	NginxPort int `default:"8080"  yaml:"nginx_port"`

	// 分流配置
	RouteDomains []string `yaml:"route_domains"` // 需要走WARP的域名列表

	// 伪装配置
	FallbackURL string `default:"https://biyuhuang.github.io/WallaceHuangBlog" yaml:"fallback_url"`

	// Nginx配置
	NginxUser            string `default:"nginx" yaml:"nginx_user"`
	NginxWorkerProcesses string `default:"auto"  yaml:"nginx_worker_processes"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		CertDir:     DefaultCertDir,
		XrayConfig:  DefaultXrayConfigPath,
		NginxConfig: DefaultNginxConfigPath,
		WARPPort:    40000,
		XrayPort:    443,
		NginxPort:   8080,
		FallbackURL: "https://biyuhuang.github.io/WallaceHuangBlog",
		RouteDomains: []string{
			"chatgpt.com", "openai.com", "oaistatic.com", "oaiusercontent.com",
			"x.ai", "grok.com", "x.com", "anthropic.com", "claude.ai",
			"bing.com", "edgeservices.bing.com",
		},
	}
}
