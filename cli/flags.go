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
	if dirty {
		if err := config.SaveConfig(cfg); err != nil {
			internal.PrintYellow("保存配置失败: %v", err)
		}
	}

	// 无动作 flag：只是做了 --domain/--uuid 的配置覆盖
	if len(chosen) == 0 {
		if dirty {
			internal.PrintGreen("配置已更新")
		} else {
			fmt.Fprintln(os.Stderr, "未指定任何操作，使用 -h 查看帮助")
		}
		return true
	}

	switch {
	case *install:
		internal.PrintGreen("开始一键安装...")
		service.InstallBase()
		service.SetupCert(cfg)
		service.SetupNginx(cfg)
		service.SetupWarp(cfg)
		service.SetupXray(cfg)
		service.CheckStatus(cfg)
	case *status:
		service.CheckStatus(cfg)
	case *restartWarp:
		service.RestartWarp(cfg)
		service.CheckStatus(cfg)
	case *updateXray:
		service.SetupXray(cfg)
	case *renewCert:
		service.SetupCert(cfg)
	case *backup:
		service.Backup(cfg)
	case *restore != "":
		service.Restore(*restore)
	case *uninstall:
		service.Uninstall()
	}

	return true
}
