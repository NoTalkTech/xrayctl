package cli

import (
	"flag"
	"os"

	"xrayctl/config"
	"xrayctl/internal"
	"xrayctl/service"
)

// ParseFlags 解析命令行参数
func ParseFlags() bool {
	var (
		install     = flag.Bool("install", false, "一键完整安装所有组件")
		domain      = flag.String("domain", "", "指定域名")
		uuid        = flag.String("uuid", "", "指定UUID")
		warpLicense = flag.String("warp-license", "", "指定WARP+许可证")
		status      = flag.Bool("status", false, "查看运行状态")
		restartWarp = flag.Bool("restart-warp", false, "重启WARP代理")
		updateXray  = flag.Bool("update-xray", false, "更新Xray核心")
		renewCert   = flag.Bool("renew-cert", false, "重新申请/续签证书")
		backup      = flag.Bool("backup", false, "备份配置与证书")
		restore     = flag.String("restore", "", "从指定备份文件恢复")
		uninstall   = flag.Bool("uninstall", false, "卸载所有组件")
	)

	flag.Parse()

	// 如果没有任何参数，返回false，显示交互式菜单
	if len(os.Args) <= 1 {
		return false
	}

	// 检查root权限
	if !internal.IsRoot() {
		internal.PrintRed("错误: 请使用 root 运行！")
		os.Exit(1)
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// 覆盖配置
	if *domain != "" {
		cfg.Domain = *domain
	}
	if *uuid != "" {
		cfg.UUID = *uuid
	}
	if *warpLicense != "" {
		cfg.WARPLicense = *warpLicense
	}
	config.SaveConfig(cfg)

	// 执行操作
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
