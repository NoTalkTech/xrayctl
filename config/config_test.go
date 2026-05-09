package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.WARPPort != 40000 {
		t.Errorf("Default WARPPort expected 40000, got %d", cfg.WARPPort)
	}
	if cfg.XrayPort != 443 {
		t.Errorf("Default XrayPort expected 443, got %d", cfg.XrayPort)
	}
	if len(cfg.RouteDomains) == 0 {
		t.Error("Default RouteDomains should not be empty")
	}
	if cfg.NginxUser != "nginx" || cfg.NginxWorkerProcesses != "auto" {
		t.Errorf("Default Nginx settings = (%q, %q), want (nginx, auto)", cfg.NginxUser, cfg.NginxWorkerProcesses)
	}
	t.Log("Default config test passed")
}

func TestSaveAndLoadConfig(t *testing.T) {
	// 临时修改配置路径
	originalPath := ConfigPath
	ConfigPath = "./test_config.yaml"
	defer func() {
		ConfigPath = originalPath
		os.Remove("./test_config.yaml")
	}()

	// 创建测试配置
	testCfg := DefaultConfig()
	testCfg.Domain = "test.example.com"
	testCfg.UUID = "test-uuid-1234"

	// 保存配置
	err := SaveConfig(testCfg)
	if err != nil {
		t.Fatalf("Save config failed: %v", err)
	}

	// 加载配置
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Load config failed: %v", err)
	}

	// 验证
	if loadedCfg.Domain != testCfg.Domain {
		t.Errorf("Domain expected %s, got %s", testCfg.Domain, loadedCfg.Domain)
	}
	if loadedCfg.UUID != testCfg.UUID {
		t.Errorf("UUID expected %s, got %s", testCfg.UUID, loadedCfg.UUID)
	}
	if loadedCfg.WARPPort != testCfg.WARPPort {
		t.Errorf("WARPPort expected %d, got %d", testCfg.WARPPort, loadedCfg.WARPPort)
	}

	t.Log("Save and load config test passed")
}

func TestLoadConfigReadOnlyDoesNotCreateMissingConfig(t *testing.T) {
	originalPath := ConfigPath
	configPath := t.TempDir() + "/missing/config.yaml"
	ConfigPath = configPath
	defer func() {
		ConfigPath = originalPath
	}()

	cfg, err := LoadConfigReadOnly()
	if err != nil {
		t.Fatalf("LoadConfigReadOnly failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfigReadOnly returned nil config")
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("LoadConfigReadOnly created config file or returned unexpected stat error: %v", err)
	}
}

func TestApplyDefaultsFillsUnsetValues(t *testing.T) {
	cfg := &Config{
		Domain:               "example.com",
		UUID:                 "existing-uuid",
		Email:                "ops@example.com",
		RouteDomains:         []string{},
		NginxUser:            "www-data",
		NginxWorkerProcesses: "2",
	}

	ApplyDefaults(cfg)

	if cfg.Domain != "example.com" || cfg.UUID != "existing-uuid" || cfg.Email != "ops@example.com" {
		t.Fatalf("ApplyDefaults changed identity fields: %#v", cfg)
	}
	if len(cfg.RouteDomains) != 0 {
		t.Fatalf("ApplyDefaults replaced explicit empty RouteDomains: %v", cfg.RouteDomains)
	}
	if cfg.CertDir == "" || cfg.XrayConfig == "" || cfg.NginxConfig == "" {
		t.Fatalf("ApplyDefaults did not fill path defaults: %#v", cfg)
	}
	if cfg.WARPPort != 40000 || cfg.XrayPort != 443 || cfg.NginxPort != 8080 {
		t.Fatalf("ApplyDefaults did not fill port defaults: %#v", cfg)
	}
	if cfg.FallbackURL == "" {
		t.Fatalf("ApplyDefaults did not fill fallback URL: %#v", cfg)
	}
	if cfg.NginxUser != "www-data" || cfg.NginxWorkerProcesses != "2" {
		t.Fatalf("ApplyDefaults changed explicit nginx fields: %#v", cfg)
	}
}
