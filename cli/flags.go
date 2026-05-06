// Package cli provides command-line and interactive menu interfaces for xrayctl.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"xrayctl/config"
	"xrayctl/internal"
	"xrayctl/service"
)

// ParseFlags 解析命令行参数。
// 返回 -1 表示应显示交互菜单，0 表示成功，1 表示失败。
//
//nolint:funlen,gocyclo // flag 定义和校验导致行数和复杂度偏高
func ParseFlags() int {
	var (
		install     = flag.Bool("install", false, "一键完整安装所有组件")
		domain      = flag.String("domain", "", "指定域名")
		uuid        = flag.String("uuid", "", "指定UUID")
		email       = flag.String("email", "", "指定证书申请邮箱 (acme.sh 注册用)")
		check       = flag.Bool("check", false, "只读预检环境与服务状态")
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
		return -1
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
		{*check, "--check"},
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

	cfg, err := loadConfigForFlagAction(*check)
	if err != nil {
		if *check {
			internal.PrintRed("检查模式无法加载配置: %v", err)
			return 1
		}

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

	if dirty && *check {
		internal.PrintYellow("检查模式仅使用命令行配置覆盖，不写入配置文件")
	} else if dirty {
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

		return 0
	}

	if err := executeFlagAction(
		cfg, *install, *check, *status, *restartWarp,
		*updateXray, *renewCert, *backup, *restore, *uninstall,
	); err != nil {
		internal.PrintRed("操作失败: %v", err)

		return 1
	}

	return 0
}

func loadConfigForFlagAction(check bool) (*config.Config, error) {
	if check {
		return config.LoadConfigReadOnly()
	}

	return config.LoadConfig()
}

// executeFlagAction runs the action corresponding to the parsed flags,
// checking errors on every service call.
func executeFlagAction(
	cfg *config.Config,
	install, check, status, restartWarp, updateXray, renewCert, backup bool,
	restore string,
	uninstall bool,
) error {
	switch {
	case install:
		internal.PrintGreen("开始一键安装...")

		if err := service.InstallAll(cfg); err != nil {
			return fmt.Errorf("一键安装失败: %w", err)
		}

	case check:
		if err := runPreflightCheck(cfg); err != nil {
			return fmt.Errorf("预检失败: %w", err)
		} else {
			internal.PrintGreen("预检通过")
		}

	case status:
		renderStatus(cfg)

	case restartWarp:
		if err := service.RestartWarp(cfg); err != nil {
			return fmt.Errorf("重启WARP失败: %w", err)
		}

		renderStatus(cfg)

	case updateXray:
		if err := service.SetupXray(cfg); err != nil {
			return fmt.Errorf("更新Xray失败: %w", err)
		}

	case renewCert:
		if err := service.SetupCert(cfg); err != nil {
			return fmt.Errorf("证书续签失败: %w", err)
		}

	case backup:
		if err := service.Backup(cfg); err != nil {
			return fmt.Errorf("备份失败: %w", err)
		}

	case restore != "":
		if err := service.Restore(restore); err != nil {
			return fmt.Errorf("恢复失败: %w", err)
		}

	case uninstall:
		if err := service.Uninstall(); err != nil {
			return fmt.Errorf("卸载失败: %w", err)
		}
	}

	return nil
}

func runPreflightCheck(cfg *config.Config) error {
	environmentReport := service.CollectEnvironmentReport(cfg)
	service.PrintEnvironmentReport(environmentReport)

	statusReport := service.CollectStatus(cfg)
	service.PrintStatusReport(statusReport)

	return errors.Join(environmentReport.ValidationError(), statusReport.ValidationError())
}
