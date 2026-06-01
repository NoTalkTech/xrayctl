package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
)

const nginxMainConfigPath = "/etc/nginx/nginx.conf"

const sysctlConfigPath = "/etc/sysctl.conf"

const (
	pkgManagerAPT    = "apt"
	pkgManagerAPTGet = "apt-get"
	pkgManagerDNF    = "dnf"
	pkgManagerYUM    = "yum"
)

// detectAptCommand 检测系统可用的 apt 命令，优先 apt 后退回到 apt-get。
// 最小 Docker 镜像（如 debian:stable-slim）可能仅提供 apt-get。
func detectAptCommand() string {
	if internal.CommandExists(pkgManagerAPT) {
		return pkgManagerAPT
	}
	if internal.CommandExists(pkgManagerAPTGet) {
		return pkgManagerAPTGet
	}
	return ""
}

type sysctlSetting struct {
	Key   string
	Value string
}

var bbrSysctlSettings = []sysctlSetting{
	{Key: "net.core.default_qdisc", Value: "fq"},
	{Key: "net.ipv4.tcp_congestion_control", Value: "bbr"},
}

var (
	environmentCommandExists = internal.CommandExists
	environmentFileExists    = internal.FileExists
	environmentBaseInstaller = InstallBase
)

// EnvironmentReport contains collected environment state without terminal rendering.
type EnvironmentReport struct {
	MissingCommands        []string
	ConfigPath             string
	ConfigExists           bool
	CertPath               string
	CertExists             bool
	XrayConfigPath         string
	XrayConfigExists       bool
	NginxMainConfigPath    string
	NginxMainConfigExists  bool
	NginxVlessConfigPath   string
	NginxVlessConfigExists bool
}

// HasMissingCommands reports whether required commands are unavailable.
func (r EnvironmentReport) HasMissingCommands() bool {
	return len(r.MissingCommands) > 0
}

// ValidationError returns an error for read-only validation contexts such as --check.
func (r EnvironmentReport) ValidationError() error {
	if !r.HasMissingCommands() {
		return nil
	}

	return fmt.Errorf("missing required commands: %s", strings.Join(r.MissingCommands, ", "))
}

// CollectEnvironmentReport inspects commands and key files without printing.
func CollectEnvironmentReport(cfg *config.Config) EnvironmentReport {
	missingCmds := []string{}

	for _, cmd := range requiredEnvironmentCommands() {
		if !environmentCommandExists(cmd) {
			missingCmds = append(missingCmds, cmd)
		}
	}

	certPath := filepath.Join(cfg.CertDir, "xray.crt")

	return EnvironmentReport{
		MissingCommands:        missingCmds,
		ConfigPath:             config.ConfigPath,
		ConfigExists:           environmentFileExists(config.ConfigPath),
		CertPath:               certPath,
		CertExists:             environmentFileExists(certPath),
		XrayConfigPath:         cfg.XrayConfig,
		XrayConfigExists:       environmentFileExists(cfg.XrayConfig),
		NginxMainConfigPath:    nginxMainConfigPath,
		NginxMainConfigExists:  environmentFileExists(nginxMainConfigPath),
		NginxVlessConfigPath:   cfg.NginxConfig,
		NginxVlessConfigExists: environmentFileExists(cfg.NginxConfig),
	}
}

// PrintEnvironmentReport renders a collected environment report without taking action.
func PrintEnvironmentReport(report EnvironmentReport) {
	internal.PrintYellow(">>> 系统环境检查 <<<")
	printDependencyReport(report)
	printEnvironmentFileReport(report)
}

// CheckSystemEnvironment 检查系统环境并自动安装缺失依赖.
func CheckSystemEnvironment(cfg *config.Config) (EnvironmentReport, error) {
	internal.PrintYellow(">>> 系统环境检查 <<<")

	report := CollectEnvironmentReport(cfg)

	// 如果有缺失依赖，自动安装
	if report.HasMissingCommands() { //nolint:nestif // sequential checks with user interaction
		printDependencyReport(report)

		fmt.Println()
		fmt.Print("是否需要自动安装这些依赖？(Y/n): ")

		var confirm string

		_, _ = fmt.Scanln(&confirm) //nolint:errcheck // user input prompt, best-effort

		if strings.EqualFold(confirm, "y") || strings.EqualFold(confirm, "yes") || confirm == "" {
			if err := environmentBaseInstaller(); err != nil {
				return report, fmt.Errorf("install missing dependencies: %w", err)
			}

			// 安装完成后，重新检查
			report = CollectEnvironmentReport(cfg)

			if report.HasMissingCommands() {
				internal.PrintRed("依赖安装失败，请手动安装")

				return report, fmt.Errorf("missing dependencies after installation: %s", strings.Join(report.MissingCommands, ", "))
			}

			internal.PrintGreen("所有依赖安装完成")
		} else {
			internal.PrintGreen("跳过依赖安装")
		}
	} else {
		printDependencyReport(report)
	}

	printEnvironmentFileReport(report)

	return report, nil
}

func printDependencyReport(report EnvironmentReport) {
	if !report.HasMissingCommands() {
		internal.PrintGreen("所有依赖已安装")

		return
	}

	internal.PrintRed("检测到缺失依赖:")

	for _, cmd := range report.MissingCommands {
		fmt.Printf("  - %s\n", cmd)
	}
}

func printEnvironmentFileReport(report EnvironmentReport) {
	internal.PrintYellow("\n>>> 配置文件检查 <<<")

	if report.ConfigExists {
		internal.PrintGreen("配置文件: %s", report.ConfigPath)
	} else {
		internal.PrintYellow("配置文件: 不存在（首次运行或未配置）")
	}

	// 检查证书目录
	if report.CertExists {
		internal.PrintGreen("证书文件: %s", report.CertPath)
	} else {
		internal.PrintYellow("证书文件: 不存在")
	}

	// 检查Xray配置
	if report.XrayConfigExists {
		internal.PrintGreen("Xray配置: %s", report.XrayConfigPath)
	} else {
		internal.PrintYellow("Xray配置: 不存在")
	}

	// 检查Nginx配置
	if report.NginxMainConfigExists {
		internal.PrintGreen("Nginx主配置: %s", report.NginxMainConfigPath)
	} else {
		internal.PrintYellow("Nginx主配置: 不存在")
	}

	if report.NginxVlessConfigExists {
		internal.PrintGreen("Nginx Vless配置: %s", report.NginxVlessConfigPath)
	} else {
		internal.PrintYellow("Nginx Vless配置: 不存在")
	}
}

func requiredEnvironmentCommands() []string {
	return []string{"curl", "jq", "nginx", "systemctl"}
}

// InstallBase 安装基础依赖和开启BBR.
func InstallBase() error {
	internal.PrintYellow("正在安装系统依赖与开启 BBR 加速...")

	// 检查所有必需命令是否已存在
	requiredCmds := []string{"curl", "jq", "nginx", "socat", "gpg", "systemctl", "wget", "git"}
	missingCmds := []string{}

	for _, cmd := range requiredCmds {
		if !internal.CommandExists(cmd) {
			missingCmds = append(missingCmds, cmd)
		}
	}

	// 如果所有命令都存在，跳过安装
	if len(missingCmds) == 0 {
		internal.PrintGreen("所有依赖已安装，跳过安装步骤")
	} else {
		// 检测系统类型
		var pkgManager string

		switch {
		case detectAptCommand() != "":
			// Debian/Ubuntu系
			pkgManager = detectAptCommand()
		case internal.CommandExists(pkgManagerYUM):
			// CentOS/RHEL系
			pkgManager = pkgManagerYUM
		case internal.CommandExists(pkgManagerDNF):
			// 新CentOS/Fedora
			pkgManager = pkgManagerDNF
		default:
			internal.PrintRed("不支持的系统类型，仅支持Debian/Ubuntu/CentOS")
			return fmt.Errorf("unsupported system")
		}

		internal.PrintGreen("检测到包管理器: %s", pkgManager)
		internal.PrintYellow("缺失依赖: %v", strings.Join(missingCmds, " "))

		// 执行安装
		if err := installMissingPackages(pkgManager, missingCmds); err != nil {
			internal.PrintRed("依赖安装失败: %v", err)
			return err
		}

		internal.PrintGreen("系统依赖安装完成")
	}

	// 检查BBR是否已经开启
	bbrOutput, err := internal.ExecCommand("sysctl", "net.ipv4.tcp_congestion_control")
	if err != nil {
		bbrOutput = ""
	}

	if !strings.Contains(bbrOutput, "bbr") {
		if err := applySysctlSettings(sysctlConfigPath, bbrSysctlSettings); err != nil {
			internal.PrintRed("写入BBR sysctl配置失败: %v", err)
			return err
		}

		if _, err := internal.ExecCommandWithSudo("sysctl", "-p"); err != nil {
			// 容器中 /proc/sys 默认只读，sysctl -p 必然失败。
			// sysctl.conf 已写入，宿主机重启或特权模式运行后会生效。
			internal.PrintYellow("BBR运行时参数设置失败（容器中需 --privileged 模式），"+
				"已写入sysctl.conf待下次重启生效: %v", err)
		}

		internal.PrintGreen("BBR加速开启成功")
	} else {
		internal.PrintGreen("BBR加速已经开启")
	}

	internal.PrintGreen("基础环境配置完成")

	return nil
}

func installMissingPackages(pkgManager string, missingCmds []string) error {
	switch pkgManager {
	case pkgManagerAPT, pkgManagerAPTGet:
		if _, err := internal.ExecCommandWithSudo(pkgManager, "update"); err != nil {
			return err
		}

		return installPackages(pkgManager, append([]string{"install", "-y"}, missingCmds...)...)

	case pkgManagerYUM, pkgManagerDNF:
		return installPackages(pkgManager, append([]string{"install", "-y"}, missingCmds...)...)

	default:
		return fmt.Errorf("unsupported package manager %q", pkgManager)
	}
}

func installPackages(pkgManager string, args ...string) error {
	_, err := internal.ExecCommandWithSudo(pkgManager, args...)

	return err
}

func applySysctlSettings(path string, settings []sysctlSetting) error {
	data, err := os.ReadFile(path) //nolint:gosec // sysctl path is fixed in production and temp-scoped in tests
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := splitConfigLines(string(data))
	seen := make(map[string]bool, len(settings))
	output := make([]string, 0, len(lines)+len(settings))

	for _, line := range lines {
		key, ok := sysctlAssignmentKey(line)
		setting, managed := lookupSysctlSetting(settings, key)

		if !ok || !managed {
			output = append(output, line)
			continue
		}

		if seen[key] {
			continue
		}

		output = append(output, formatSysctlSetting(setting))
		seen[key] = true
	}

	for _, setting := range settings {
		if !seen[setting.Key] {
			output = append(output, formatSysctlSetting(setting))
		}
	}

	result := []byte(strings.Join(output, "\n") + "\n")

	return os.WriteFile(path, result, 0o644) //nolint:gosec // sysctl.conf is conventionally world-readable
}

func splitConfigLines(data string) []string {
	data = strings.TrimSuffix(data, "\n")
	if data == "" {
		return nil
	}

	return strings.Split(data, "\n")
}

func sysctlAssignmentKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}

	key, _, ok := strings.Cut(trimmed, "=")
	if !ok {
		return "", false
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}

	return key, true
}

func lookupSysctlSetting(settings []sysctlSetting, key string) (sysctlSetting, bool) {
	for _, setting := range settings {
		if setting.Key == key {
			return setting, true
		}
	}

	return sysctlSetting{}, false
}

func formatSysctlSetting(setting sysctlSetting) string {
	return setting.Key + " = " + setting.Value
}
