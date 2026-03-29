package service

import (
	"fmt"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
)

// CheckSystemEnvironment 检查系统环境并自动安装缺失依赖
func CheckSystemEnvironment(cfg *config.Config) {
	internal.PrintYellow(">>> 系统环境检查 <<<")

	// 检查必需命令
	requiredCmds := []string{"curl", "jq", "nginx", "systemctl"}
	missingCmds := []string{}
	for _, cmd := range requiredCmds {
		if !internal.CommandExists(cmd) {
			missingCmds = append(missingCmds, cmd)
		}
	}

	// 如果有缺失依赖，自动安装
	if len(missingCmds) > 0 {
		internal.PrintRed("检测到缺失依赖:")
		for _, cmd := range missingCmds {
			fmt.Printf("  - %s\n", cmd)
		}
		fmt.Println()
		fmt.Print("是否需要自动安装这些依赖？(Y/n): ")

		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) == "y" || strings.ToLower(confirm) == "yes" || confirm == "" {
			err := InstallBase()
			if err != nil {
				internal.PrintYellow("依赖安装完成，请重试")
				return
			}
			// 安装完成后，重新检查
			missingCmds = []string{}
			for _, cmd := range requiredCmds {
				if !internal.CommandExists(cmd) {
					missingCmds = append(missingCmds, cmd)
				}
			}
			if len(missingCmds) > 0 {
				internal.PrintRed("依赖安装失败，请手动安装")
				return
			}
			internal.PrintGreen("所有依赖安装完成")
		} else {
			internal.PrintGreen("跳过依赖安装")
		}
	} else {
		internal.PrintGreen("所有依赖已安装")
	}

	// 检查配置文件
	internal.PrintYellow("\n>>> 配置文件检查 <<<")
	if internal.FileExists(config.ConfigPath) {
		internal.PrintGreen("配置文件: %s", config.ConfigPath)
	} else {
		internal.PrintYellow("配置文件: 不存在（首次运行或未配置）")
	}

	// 检查证书目录
	if internal.FileExists(fmt.Sprintf("%s/xray.crt", cfg.CertDir)) {
		internal.PrintGreen("证书文件: %s/xray.crt", cfg.CertDir)
	} else {
		internal.PrintYellow("证书文件: 不存在")
	}

	// 检查Xray配置
	if internal.FileExists(cfg.XrayConfig) {
		internal.PrintGreen("Xray配置: %s", cfg.XrayConfig)
	} else {
		internal.PrintYellow("Xray配置: 不存在")
	}

	// 检查Nginx配置
	if internal.FileExists("/etc/nginx/nginx.conf") {
		internal.PrintGreen("Nginx主配置: /etc/nginx/nginx.conf")
	} else {
		internal.PrintYellow("Nginx主配置: 不存在")
	}
	if internal.FileExists(cfg.NginxConfig) {
		internal.PrintGreen("Nginx Vless配置: %s", cfg.NginxConfig)
	} else {
		internal.PrintYellow("Nginx Vless配置: 不存在")
	}
}

// InstallBase 安装基础依赖和开启BBR
func InstallBase() error {
	internal.PrintYellow("正在安装系统依赖与开启 BBR 加速...")

	// 检查所有必需命令是否已存在
	requiredCmds := []string{"curl", "jq", "nginx", "uuidgen", "socat", "lsb_release", "gpg", "systemctl", "wget"}
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
		missedCmdsStr := strings.Join(missingCmds, " ")
		// 检测系统类型
		var installCmd string
		var pkgManager string
		if internal.CommandExists("apt") {
			// Debian/Ubuntu系
			installCmd = "apt update && apt install -y " + missedCmdsStr
			pkgManager = "apt"
		} else if internal.CommandExists("yum") {
			// CentOS/RHEL系
			installCmd = "yum install -y " + missedCmdsStr
			pkgManager = "yum"
		} else if internal.CommandExists("dnf") {
			// 新CentOS/Fedora
			installCmd = "dnf install -y " + missedCmdsStr
			pkgManager = "dnf"
		} else {
			internal.PrintRed("不支持的系统类型，仅支持Debian/Ubuntu/CentOS")
			return fmt.Errorf("unsupported system")
		}

		internal.PrintGreen("检测到包管理器: %s", pkgManager)
		internal.PrintYellow("缺失依赖: %v", missedCmdsStr)

		// 执行安装
		_, err := internal.ExecCommandWithSudo("bash", "-c", installCmd)
		if err != nil {
			internal.PrintRed("依赖安装失败: %v", err)
			return err
		}
		internal.PrintGreen("系统依赖安装完成")
	}

	// 检查BBR是否已经开启
	bbrOutput, _ := internal.ExecCommand("sysctl", "net.ipv4.tcp_congestion_control")
	if !strings.Contains(bbrOutput, "bbr") {
		// 开启BBR
		cmds := []string{
			"echo 'net.core.default_qdisc=fq' >> /etc/sysctl.conf",
			"echo 'net.ipv4.tcp_congestion_control=bbr' >> /etc/sysctl.conf",
			"sysctl -p",
		}
		for _, cmd := range cmds {
			_, err := internal.ExecCommandWithSudo("bash", "-c", cmd)
			if err != nil {
				internal.PrintRed("开启BBR失败: %v", err)
				return err
			}
		}
		internal.PrintGreen("BBR加速开启成功")
	} else {
		internal.PrintGreen("BBR加速已经开启")
	}

	internal.PrintGreen("基础环境配置完成")
	return nil
}
