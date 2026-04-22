package config

import (
	"os"
	"path/filepath"
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

// SaveConfig 保存配置到文件。配置含 UUID 等敏感字段，所以目录 0700 / 文件 0600，
// 仅 root 可读。写入采用 tmp + rename 的原子模式，避免崩溃或断电时留下截断的
// 半成品 yaml。
func SaveConfig(cfg *Config) error {
	dir := filepath.Dir(ConfigPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(ConfigPath)+".tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// 失败路径上清掉 tmp；成功 rename 之后这次 Remove 是 no-op (文件已不在旧名)。
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, ConfigPath)
}
