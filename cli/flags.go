// Package cli provides command-line and interactive menu interfaces for xrayctl.
package cli

import (
	"flag"
	"fmt"
	"os"

	"xrayctl/config"
	"xrayctl/internal"
	"xrayctl/service"
)

// ParseFlags 解析命令行参数；无参数时返回 false 以便 main 回落到交互菜单。
func ParseFlags() bool {
	var (
		install     = flag.Bool("install", false, "一键完整安装所有组件")
		domain      = flag.String("domain", "", "指定域名")
		uuid        = flag.String("uuid", "", "指定UUID")
		email       = flag.String("email", "", "指定证书申请邮箱 (acme.sh 注册用)")
		status      = flag.Bool("status", false, "查看运行状态")
		restartWarp = flag.Bool("restart-warp", false, "重启WARP代理")
		updateXray  = flag.Bool("update-xray", false, "更新Xray核心")
		renewCert   = flag.Bool("renew-cert", false, "重新申请/续签证书")
		backup      = flag.Bool("backup", false, "备份配置与证书")
		restore     = flag.String("restore", "", "从指定备份文件恢复")
		uninstall   = flag.Bool("uninstall", false, "卸载所有组件")
	)

	flag.Parse()

	// 没有任何参数：回到交互菜单
	if len(os.Args) <= 1 {
		return false
	}

	if !internal.IsRoot() {
		internal.PrintRed("错误: 请使用 root 运行！")
		os.Exit(1)
	}

	// 互斥检查：最多允许一个动作 flag
	actions := []struct {
		set  bool
		name string
	}{
		{*install, "--install"},
		{*status, "--status"},
		{*restartWarp, "--restart-warp"},
		{*updateXray, "--update-xray"},
		{*renewCert, "--renew-cert"},
		{*backup, "--backup"},
		{*restore != "", "--restore"},
		{*uninstall, "--uninstall"},
	}
	var chosen []string

	for _, a := range actions {
		if a.set {
			chosen = append(chosen, a.name)
		}
	}

	if len(chosen) > 1 {
		internal.PrintRed("错误: 一次只能指定一个动作，当前传入了 %v", chosen)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// 仅在真正被用户覆盖时才写回配置文件
	dirty := false

	if *domain != "" && *domain != cfg.Domain {
		cfg.Domain = *domain
		dirty = true
	}

	if *uuid != "" && *uuid != cfg.UUID {
		cfg.UUID = *uuid
		dirty = true
	}

	if *email != "" && *email != cfg.Email {
		cfg.Email = *email
		dirty = true
	}

	if dirty {
		if err := config.SaveConfig(cfg); err != nil {
			internal.PrintYellow("保存配置失败: %v", err)
		}
	}

	// 无动作 flag：只是做了 --domain/--uuid/--email 的配置覆盖
	if len(chosen) == 0 {
		if dirty {
			internal.PrintGreen("配置已更新")
		} else {
			fmt.Fprintln(os.Stderr, "未指定任何操作，使用 -h 查看帮助")
		}

		return true
	}

	executeFlagAction(cfg, *install, *status, *restartWarp, *updateXray, *renewCert, *backup, *restore, *uninstall)

	return true
}

// executeFlagAction runs the action corresponding to the parsed flags,
// checking errors on every service call.
func executeFlagAction(
	cfg *config.Config,
	install, status, restartWarp, updateXray, renewCert, backup bool,
	restore string,
	uninstall bool,
) {
	switch {
	case install:
		internal.PrintGreen("开始一键安装...")

		if err := service.InstallBase(); err != nil {
			internal.PrintRed("安装基础依赖失败: %v", err)
			break
		}

		if err := service.SetupCert(cfg); err != nil {
			internal.PrintRed("证书配置失败: %v", err)
			break
		}

		if err := service.SetupNginx(cfg); err != nil {
			internal.PrintRed("Nginx配置失败: %v", err)
			break
		}

		if err := service.SetupWarp(cfg); err != nil {
			internal.PrintRed("WARP配置失败: %v", err)
			break
		}

		if err := service.SetupXray(cfg); err != nil {
			internal.PrintRed("Xray配置失败: %v", err)
			break
		}

		service.CheckStatus(cfg)

	case status:
		service.CheckStatus(cfg)

	case restartWarp:
		if err := service.RestartWarp(cfg); err != nil {
			internal.PrintRed("WARP重启失败: %v", err)
		}

		service.CheckStatus(cfg)

	case updateXray:
		if err := service.SetupXray(cfg); err != nil {
			internal.PrintRed("Xray更新失败: %v", err)
		}

	case renewCert:
		if err := service.SetupCert(cfg); err != nil {
			internal.PrintRed("证书续签失败: %v", err)
		}

	case backup:
		if err := service.Backup(cfg); err != nil {
			internal.PrintRed("备份失败: %v", err)
		}

	case restore != "":
		if err := service.Restore(restore); err != nil {
			internal.PrintRed("恢复失败: %v", err)
		}

	case uninstall:
		service.Uninstall()
	}
}
