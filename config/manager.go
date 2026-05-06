package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigPath 配置文件路径，可修改用于测试.
var ConfigPath = DefaultConfigPath

// ApplyDefaults fills unset configuration fields with the built-in defaults.
func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	defaultCfg := DefaultConfig()

	applyStringDefault(&cfg.CertDir, defaultCfg.CertDir)
	applyStringDefault(&cfg.XrayConfig, defaultCfg.XrayConfig)
	applyStringDefault(&cfg.NginxConfig, defaultCfg.NginxConfig)
	applyIntDefault(&cfg.WARPPort, defaultCfg.WARPPort)
	applyIntDefault(&cfg.XrayPort, defaultCfg.XrayPort)
	applyIntDefault(&cfg.NginxPort, defaultCfg.NginxPort)

	if cfg.RouteDomains == nil {
		cfg.RouteDomains = defaultCfg.RouteDomains
	}

	applyStringDefault(&cfg.FallbackURL, defaultCfg.FallbackURL)
	applyStringDefault(&cfg.NginxUser, defaultCfg.NginxUser)
	applyStringDefault(&cfg.NginxWorkerProcesses, defaultCfg.NginxWorkerProcesses)
}

func applyStringDefault(field *string, value string) {
	if *field == "" {
		*field = value
	}
}

func applyIntDefault(field *int, value int) {
	if *field == 0 {
		*field = value
	}
}

// LoadConfig 加载配置文件，如果不存在则返回默认配置并尝试持久化.
func LoadConfig() (*Config, error) {
	cfg, missing, err := loadConfigFromDisk()
	if err != nil {
		return nil, err
	}

	if missing {
		if err := SaveConfig(cfg); err != nil {
			return cfg, nil //nolint:nilerr // 保存失败也返回默认配置
		}
	}

	return cfg, nil
}

// LoadConfigReadOnly loads configuration without creating or modifying files.
func LoadConfigReadOnly() (*Config, error) {
	cfg, _, err := loadConfigFromDisk()

	return cfg, err
}

func loadConfigFromDisk() (*Config, bool, error) {
	// 检查配置文件是否存在
	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		return DefaultConfig(), true, nil
	}

	// 读取配置文件
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return nil, false, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, false, err
	}

	// 填充默认值
	ApplyDefaults(&cfg)

	return &cfg, false, nil
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
	defer func() {
		_ = os.Remove(tmpPath) //nolint:errcheck // cleanup-only, error ignored
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() //nolint:errcheck // close error ignored on write failure
		return err
	}

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close() //nolint:errcheck // close error ignored on chmod failure
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, ConfigPath)
}
