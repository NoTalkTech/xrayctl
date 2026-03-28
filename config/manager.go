package config

import (
	"os"
	"reflect"

	"gopkg.in/yaml.v3"
)

// ConfigPath 配置文件路径，可修改用于测试
var ConfigPath = DefaultConfigPath

// fillDefaults 自动填充空字段为默认值
func fillDefaults(cfg *Config, defaultCfg *Config) {
	cfgVal := reflect.ValueOf(cfg).Elem()
	defaultVal := reflect.ValueOf(defaultCfg).Elem()

	for i := 0; i < cfgVal.NumField(); i++ {
		field := cfgVal.Field(i)
		if field.IsZero() {
			field.Set(defaultVal.Field(i))
		}
	}
}

// LoadConfig 加载配置文件，如果不存在则返回默认配置
func LoadConfig() (*Config, error) {
	defaultCfg := DefaultConfig()

	// 检查配置文件是否存在
	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		// 尝试保存默认配置
		if err := SaveConfig(defaultCfg); err != nil {
			return defaultCfg, nil // 保存失败也返回默认配置
		}
		return defaultCfg, nil
	}

	// 读取配置文件
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 填充默认值
	fillDefaults(&cfg, defaultCfg)

	return &cfg, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg *Config) error {
	// 确保配置目录存在
	if err := os.MkdirAll(DefaultConfigDir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath, data, 0644)
}