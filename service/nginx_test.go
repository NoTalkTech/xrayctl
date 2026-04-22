package service

import (
	"strings"
	"testing"

	"xrayctl/config"
)

func TestBuildVlessConf(t *testing.T) {
	cfg := &config.Config{
		Domain:      "test.example.com",
		NginxPort:   8080,
		FallbackURL: "https://example.com",
	}
	conf := buildVlessConf(cfg)
	if !strings.Contains(conf, cfg.Domain) {
		t.Error("Nginx vless config should contain domain")
	}
	if !strings.Contains(conf, cfg.FallbackURL) {
		t.Error("Nginx vless config should contain fallback URL")
	}
	if !strings.Contains(conf, "proxy_protocol") {
		t.Error("Nginx vless config should declare proxy_protocol")
	}
	if !strings.Contains(conf, "127.0.0.1:8080") {
		t.Error("Nginx vless config should listen on the configured port")
	}
}

func TestBuildMainConfDefaults(t *testing.T) {
	cfg := &config.Config{} // deliberately empty → defaults applied.
	conf := buildMainConf(cfg)
	if !strings.Contains(conf, "user nginx nginx;") {
		t.Error("main conf should default user to nginx")
	}
	if !strings.Contains(conf, "worker_processes auto;") {
		t.Error("main conf should default worker_processes to auto")
	}
	if !strings.Contains(conf, "default_server") {
		t.Error("main conf should include default_server deny block")
	}
}
