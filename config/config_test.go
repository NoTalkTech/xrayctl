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
	testCfg.WARPLicense = "test-license"

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
	if loadedCfg.WARPLicense != testCfg.WARPLicense {
		t.Errorf("WARPLicense expected %s, got %s", testCfg.WARPLicense, loadedCfg.WARPLicense)
	}
	if loadedCfg.WARPPort != testCfg.WARPPort {
		t.Errorf("WARPPort expected %d, got %d", testCfg.WARPPort, loadedCfg.WARPPort)
	}

	t.Log("Save and load config test passed")
}
