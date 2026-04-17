package config

// Config 全局配置结构
type Config struct {
	// 基础配置
	Domain string `yaml:"domain"`
	UUID   string `yaml:"uuid"`
	Email  string `yaml:"email"` // 证书申请用邮箱

	// 路径配置
	CertDir      string `yaml:"cert_dir" default:"/etc/xray/cert"`
	XrayConfig   string `yaml:"xray_config" default:"/usr/local/etc/xray/config.json"`
	NginxConfDir string `yaml:"nginx_conf_dir" default:"/etc/nginx/conf.d"`
	NginxConfig  string `yaml:"nginx_config" default:"/etc/nginx/conf.d/vless.conf"`
	LogDir       string `yaml:"log_dir" default:"/var/log/xrayctl"`

	// 端口配置
	WARPPort  int `yaml:"warp_port" default:"40000"`
	XrayPort  int `yaml:"xray_port" default:"443"`
	NginxPort int `yaml:"nginx_port" default:"8080"`

	// 分流配置
	RouteDomains []string `yaml:"route_domains_domain"` // 需要走WARP的域名列表

	// 伪装配置
	FallbackURL  string `yaml:"fallback_url" default:"https://biyuhuang.github.io/WallaceHuangBlog"`
	FallbackPath string `yaml:"fallback_path" default:"/var/www/html"`

	// Nginx配置
	NginxUser            string `yaml:"nginx_user" default:"nginx"`
	NginxWorkerProcesses string `yaml:"nginx_worker_processes" default:"auto"`

	// WARP配置
	WARPLicense string `yaml:"warp_license"` // WARP+许可证

	// AdminEmail string `yaml:"admin_email"` // 管理员邮箱
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		CertDir:     DefaultCertDir,
		XrayConfig:  DefaultXrayConfigPath,
		NginxConfig: DefaultNginxConfigPath,
		LogDir:      DefaultLogDir,
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
