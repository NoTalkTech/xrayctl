// Package cli provides command-line and interactive menu interfaces for xrayctl.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
	"xrayctl/service"
)

// ShowMenu 显示主菜单.
func ShowMenu() {
	// 检查是否是root用户
	if !internal.IsRoot() {
		internal.PrintRed("错误: 请使用 root 运行！")
		os.Exit(1)
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		internal.PrintYellow("配置加载失败，使用默认配置: %v", err)

		cfg = config.DefaultConfig()
	}

	// 首次启动时进行环境检查
	service.CheckSystemEnvironment(cfg)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		clearScreen()

		// 显示当前状态摘要
		internal.PrintGreen("==========================================")
		internal.PrintGreen("|      Xray-WARP 精准分流管理系统        |")
		internal.PrintGreen("|      (VLESS + XTLS + Nginx Fallback)   |")
		internal.PrintGreen("==========================================")

		if cfg.Domain != "" {
			fmt.Printf("  域名: %s\n", cfg.Domain)
		} else {
			fmt.Println("  域名: 未配置")
		}

		if cfg.Email != "" {
			fmt.Printf("  Email: %s\n", cfg.Email)
		} else {
			fmt.Println("  Email: 未配置")
		}

		// 服务状态摘要
		fmt.Println("  服务状态:")
		renderServiceStatus("Nginx", service.NginxStatus())
		renderServiceStatus("Xray ", service.XrayStatus())
		renderServiceStatus("WARP ", service.WarpStatus())

		internal.PrintGreen("==========================================")
		fmt.Println("1. 🚀 完整安装 (一键打通全链路)")
		fmt.Println("2. 🔄 仅更新 Xray 核心 (保留配置与 UUID)")
		fmt.Println("3. 🔑 重新申请/续签 SSL 证书")
		fmt.Println("4. 📡 重启并校验 WARP 代理出口")
		fmt.Println("5. 🔍 查看运行状态与连接参数")
		fmt.Println("6. 📋 系统环境详细检查")
		fmt.Println("7. 💾 备份配置与证书")
		fmt.Println("8. 📤 从备份恢复")
		fmt.Println("9. 🗑️  彻底卸载相关组件")
		fmt.Println("0. 退出")
		internal.PrintGreen("==========================================")
		fmt.Print("选择操作 [0-9]: ")

		if !scanner.Scan() {
			// stdin closed (EOF or read error) — exit instead of spinning.
			fmt.Println()
			internal.PrintYellow("输入已结束，退出菜单。")

			return
		}

		executeMenuAction(cfg, scanner)

		fmt.Println("\n按回车键继续...")

		if !scanner.Scan() {
			internal.PrintYellow("输入已结束，退出菜单。")

			return
		}
	}
}

// executeMenuInstallAction runs the full install pipeline, checking
// errors at each step.
func executeMenuInstallAction(cfg *config.Config) {
	internal.PrintGreen("开始一键安装...")

	if err := service.InstallBase(); err != nil {
		internal.PrintRed("安装基础依赖失败: %v", err)
		return
	}

	if err := service.SetupCert(cfg); err != nil {
		internal.PrintRed("证书配置失败: %v", err)
		return
	}

	if err := service.SetupNginx(cfg); err != nil {
		internal.PrintRed("Nginx配置失败: %v", err)
		return
	}

	if err := service.SetupWarp(cfg); err != nil {
		internal.PrintRed("WARP配置失败: %v", err)
		return
	}

	if err := service.SetupXray(cfg); err != nil {
		internal.PrintRed("Xray配置失败: %v", err)
		return
	}

	service.CheckStatus(cfg)
}

// executeMenuAction runs the menu action selected by the user,
// checking errors on every service call.
func executeMenuAction(cfg *config.Config, scanner *bufio.Scanner) { //nolint:gocyclo // switch dispatch
	opt := strings.TrimSpace(scanner.Text())
	switch opt {
	case "1":
		// 完整安装
		executeMenuInstallAction(cfg)
	case "2":
		// 更新Xray
		if err := service.SetupXray(cfg); err != nil {
			internal.PrintRed("Xray更新失败: %v", err)
		}

	case "3":
		// 重新申请证书
		if err := service.SetupCert(cfg); err != nil {
			internal.PrintRed("证书申请失败: %v", err)
		} else {
			if err := internal.RestartService(internal.ServiceXray); err != nil {
				internal.PrintYellow("Xray重启失败: %v", err)
			}
		}

	case "4":
		// 重启WARP
		if err := service.RestartWarp(cfg); err != nil {
			internal.PrintRed("WARP重启失败: %v", err)
		}

		service.CheckStatus(cfg)

	case "5":
		// 查看状态
		service.CheckStatus(cfg)
	case "6":
		// 系统环境检查
		service.CheckSystemEnvironment(cfg)
	case "7":
		// 备份
		if err := service.Backup(cfg); err != nil {
			internal.PrintRed("备份失败: %v", err)
		}

	case "8":
		// 恢复
		fmt.Print("请输入备份文件路径: ")

		if !scanner.Scan() {
			fmt.Println()
			internal.PrintYellow("输入已结束，退出菜单。")

			return
		}

		backupFile := strings.TrimSpace(scanner.Text())
		if backupFile != "" {
			if err := service.Restore(backupFile); err != nil {
				internal.PrintRed("恢复失败: %v", err)
			}
		}

	case "9":
		// 卸载
		fmt.Print("确定要卸载所有组件吗？(y/N): ")

		if !scanner.Scan() {
			fmt.Println()
			internal.PrintYellow("输入已结束，退出菜单。")

			return
		}

		confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if confirm == "y" || confirm == "yes" {
			service.Uninstall()
		}

	case "0":
		os.Exit(0)
	default:
		internal.PrintRed("无效选择，请重新输入")
	}
}

// renderServiceStatus prints a colorized one-liner for a single systemd
// service status, matching the menu header layout.
func renderServiceStatus(label, status string) {
	fmt.Printf("    %s: ", label)

	switch status {
	case internal.StatusActive:
		fmt.Println(internal.Green + "running" + internal.Reset)
	case internal.StatusFailed:
		fmt.Println(internal.Red + "fail" + internal.Reset)
	default:
		fmt.Println(internal.Yellow + "not started" + internal.Reset)
	}
}

// clearScreen 清屏.
func clearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	_ = cmd.Run() //nolint:errcheck // clear screen, best-effort
}
