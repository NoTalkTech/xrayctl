package service

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"xrayctl/config"
	"xrayctl/internal"
)

//go:embed templates/nginx.conf.tmpl templates/vless.conf.tmpl
var nginxTemplates embed.FS

var (
	mainConfTmpl  = template.Must(template.ParseFS(nginxTemplates, "templates/nginx.conf.tmpl"))
	vlessConfTmpl = template.Must(template.ParseFS(nginxTemplates, "templates/vless.conf.tmpl"))
)

type nginxMainParams struct {
	User            string
	WorkerProcesses string
}

type nginxVlessParams struct {
	Domain      string
	NginxPort   int
	FallbackURL string
}

// buildMainConf renders /etc/nginx/nginx.conf from the embedded template.
func buildMainConf(cfg *config.Config) string {
	user := cfg.NginxUser
	if user == "" {
		user = "nginx"
	}

	workers := cfg.NginxWorkerProcesses
	if workers == "" {
		workers = "auto"
	}

	var buf bytes.Buffer
	if err := mainConfTmpl.Execute(&buf, nginxMainParams{User: user, WorkerProcesses: workers}); err != nil {
		// Templates are embedded and tested — an error here is a programmer bug.
		panic(fmt.Errorf("render nginx main template: %w", err))
	}

	return buf.String()
}

// buildVlessConf renders cfg.NginxConfig from the embedded template.
func buildVlessConf(cfg *config.Config) string {
	var buf bytes.Buffer
	if err := vlessConfTmpl.Execute(&buf, nginxVlessParams{
		Domain:      cfg.Domain,
		NginxPort:   cfg.NginxPort,
		FallbackURL: cfg.FallbackURL,
	}); err != nil {
		panic(fmt.Errorf("render nginx vless template: %w", err))
	}

	return buf.String()
}

// SetupNginxMainConf 生成 Nginx 主配置文件.
func SetupNginxMainConf(cfg *config.Config) error {
	if err := internal.WriteFile("/etc/nginx/nginx.conf", []byte(buildMainConf(cfg)), 0o644); err != nil {
		internal.PrintRed("Nginx主配置写入失败: %v", err)
		return err
	}

	if err := internal.MkdirIfNotExists("/usr/share/nginx/html", 0o755); err != nil {
		internal.PrintYellow("创建Nginx静态目录失败: %v", err)
	}

	return nil
}

// SetupNginxVlessConf 生成 Vless 回落配置.
func SetupNginxVlessConf(cfg *config.Config) error {
	if cfg.Domain == "" {
		// Reuse the shared prompt helper so we get EOF-safe behavior and
		// domain validation instead of fmt.Scanln spinning on closed stdin.
		cfg.Domain = promptValue("域名", "", validateDomain)
		if err := config.SaveConfig(cfg); err != nil {
			internal.PrintYellow("保存配置失败: %v", err)
		}
	}

	if err := internal.WriteFile(cfg.NginxConfig, []byte(buildVlessConf(cfg)), 0o644); err != nil {
		internal.PrintRed("Nginx Vless配置写入失败: %v", err)
		return err
	}

	return nil
}

// SetupNginx 配置 Nginx 回落.
func SetupNginx(cfg *config.Config) error {
	internal.PrintYellow("正在配置 Nginx 回落模块...")

	if err := SetupNginxMainConf(cfg); err != nil {
		return err
	}

	if err := SetupNginxVlessConf(cfg); err != nil {
		return err
	}

	if _, err := internal.ExecCommandWithSudo("nginx", "-t"); err != nil {
		internal.PrintRed("Nginx配置错误: %v", err)
		return err
	}

	if err := internal.RestartService(internal.ServiceNginx); err != nil {
		internal.PrintRed("Nginx启动失败: %v", err)
		return err
	}

	internal.EnableService(internal.ServiceNginx)

	internal.PrintGreen("Nginx配置完成")

	return nil
}

// RestartNginx 重启 Nginx.
func RestartNginx() error {
	return internal.RestartService(internal.ServiceNginx)
}

// NginxStatus 获取 Nginx 运行状态.
func NginxStatus() string {
	return internal.ServiceStatus(internal.ServiceNginx)
}
