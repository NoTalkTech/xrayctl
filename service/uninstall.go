package service

import (
	"errors"
	"fmt"

	"xrayctl/config"
	"xrayctl/internal"
)

var (
	uninstallCommandExists = internal.CommandExists
	uninstallRunWithSudo   = internal.ExecCommandWithSudo
)

// Uninstall 卸载所有组件.
func Uninstall() error {
	internal.PrintYellow("正在卸载所有组件...")

	var errs []error
	if err := stopServicesForUninstall(); err != nil {
		errs = append(errs, err)
	}

	// Xray is typically installed via the upstream release script, not as a
	// system package. Remove known system packages first, then attempt the
	// Xray package separately — its failure is non-fatal.
	pkgs := []string{"cloudflare-warp", internal.ServiceNginx}

	if err := removePackagesForUninstall(pkgs); err != nil {
		errs = append(errs, err)
	}

	if err := removePackagesForUninstall([]string{internal.ServiceXray}); err != nil {
		internal.PrintYellow("移除Xray软件包失败（可能并非通过包管理器安装）: %v", err)
	}

	if _, err := uninstallRunWithSudo(
		"rm", "-rf", "/etc/xray", "/usr/local/etc/xray", config.DefaultConfigDir,
	); err != nil {
		internal.PrintYellow("清理残留文件失败: %v", err)
		errs = append(errs, fmt.Errorf("remove residual files: %w", err))
	}

	if err := errors.Join(errs...); err != nil {
		return err
	}

	internal.PrintGreen("卸载完成")

	return nil
}

func stopServicesForUninstall() error {
	var errs []error

	for _, serviceName := range []string{internal.ServiceXray, internal.ServiceNginx, internal.ServiceWarp} {
		if _, err := uninstallRunWithSudo("systemctl", "stop", serviceName); err != nil {
			internal.PrintYellow("停止 %s 失败: %v", serviceName, err)
			errs = append(errs, fmt.Errorf("stop %s: %w", serviceName, err))
		}
	}

	return errors.Join(errs...)
}

func removePackagesForUninstall(pkgs []string) error {
	switch {
	case uninstallCommandExists(pkgManagerAPT):
		return runPackageUninstall(pkgManagerAPT, append([]string{"purge", "-y"}, pkgs...)...)
	case uninstallCommandExists(pkgManagerAPTGet):
		return runPackageUninstall(pkgManagerAPTGet, append([]string{"purge", "-y"}, pkgs...)...)
	case uninstallCommandExists(pkgManagerDNF):
		return runPackageUninstall(pkgManagerDNF, append([]string{"remove", "-y"}, pkgs...)...)
	case uninstallCommandExists(pkgManagerYUM):
		return runPackageUninstall(pkgManagerYUM, append([]string{"remove", "-y"}, pkgs...)...)
	default:
		internal.PrintRed("未找到支持的包管理器（apt/yum/dnf）")
		return fmt.Errorf("no supported package manager found")
	}
}

func runPackageUninstall(name string, args ...string) error {
	if _, err := uninstallRunWithSudo(name, args...); err != nil {
		internal.PrintYellow("%s卸载失败: %v", name, err)

		return fmt.Errorf("%s uninstall: %w", name, err)
	}

	return nil
}
