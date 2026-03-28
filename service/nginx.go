package service

import (
	"fmt"

	"xrayctl/config"
	"xrayctl/internal"
)

// SetupNginxMainConf 生成Nginx主配置文件
func SetupNginxMainConf(cfg *config.Config) error {
	mainConfPath := "/etc/nginx/nginx.conf"

	// 生成主配置内容
	mainConf := fmt.Sprintf(`user %s;
worker_processes %s;
error_log /var/log/nginx/error.log notice;
pid /var/run/nginx.pid;

events {
    worker_connections 4096;
}

http {
    server_tokens off;
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    sendfile on;
    keepalive_timeout 65;

    # 包含自定义配置
    include %s/*.conf;
}
`, cfg.NginxUser, cfg.NginxWorkerProcesses, cfg.NginxConfDir)

	// 写入文件
	err := internal.WriteFile(mainConfPath, []byte(mainConf), 0644)
	if err != nil {
		internal.PrintRed("Nginx主配置写入失败: %v", err)
		return err
	}

	// 创建网站根目录
	internal.MkdirIfNotExists(cfg.FallbackPath, 0755)

	return nil
}

// SetupNginxVlessConf 生成Vless回落配置
func SetupNginxVlessConf(cfg *config.Config) error {
	if cfg.Domain == "" {
		fmt.Print("请输入域名: ")
		fmt.Scanln(&cfg.Domain)
		config.SaveConfig(cfg)
	}

	// 生成Vless配置内容
	vlessConf := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    location ^~ /.well-known/acme-challenge/ {
        root %s;
        default_type "text/plain";
        allow all;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 127.0.0.1:%d proxy_protocol;
    http2 on;
    server_name %s;

    set_real_ip_from 127.0.0.1;
    real_ip_header X-Forwarded-For;
    real_ip_recursive on;

    add_header Strict-Transport-Security "max-age=63072000" always;
    root %s;

    error_page 403 %s;

    location / {
        return 403;
    }
}
`, cfg.Domain, cfg.FallbackPath, cfg.NginxPort, cfg.Domain, cfg.FallbackPath, cfg.FallbackURL)

	// 写入文件
	err := internal.WriteFile(cfg.NginxConfig, []byte(vlessConf), 0644)
	if err != nil {
		internal.PrintRed("Nginx Vless配置写入失败: %v", err)
		return err
	}

	return nil
}

// SetupNginx 配置Nginx回落
func SetupNginx(cfg *config.Config) error {
	internal.PrintYellow("正在配置 Nginx 回落模块...")

	// 生成主配置
	err := SetupNginxMainConf(cfg)
	if err != nil {
		return err
	}

	// 生成Vless配置
	err = SetupNginxVlessConf(cfg)
	if err != nil {
		return err
	}

	// 测试配置
	_, err = internal.ExecCommandWithSudo("nginx", "-t")
	if err != nil {
		internal.PrintRed("Nginx配置错误: %v", err)
		return err
	}

	// 重启Nginx
	err = internal.RestartService(internal.ServiceNginx)
	if err != nil {
		internal.PrintRed("Nginx重启失败: %v", err)
		return err
	}

	// 设置开机自启
	internal.EnableService(internal.ServiceNginx)

	internal.PrintGreen("Nginx配置完成")
	return nil
}

// RestartNginx 重启Nginx
func RestartNginx() error {
	return internal.RestartService(internal.ServiceNginx)
}

// NginxStatus 获取Nginx运行状态
func NginxStatus() string {
	return internal.ServiceStatus(internal.ServiceNginx)
}
