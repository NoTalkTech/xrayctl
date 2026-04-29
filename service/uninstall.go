package service

import (
	"xrayctl/config"
	"xrayctl/internal"
)

// Uninstall 卸载所有组件.
func Uninstall() {
	internal.PrintYellow("正在卸载所有组件...")
	internal.StopService(internal.ServiceXray)
	internal.StopService(internal.ServiceNginx)
	internal.StopService(internal.ServiceWarp)

	pkgs := []string{"cloudflare-warp", internal.ServiceNginx, internal.ServiceXray}

	switch {
	case internal.CommandExists("apt"):
		if _, err := internal.ExecCommandWithSudo("apt", append([]string{"purge", "-y"}, pkgs...)...); err != nil {
			internal.PrintYellow("apt卸载失败: %v", err)
		}

	case internal.CommandExists("dnf"):
		if _, err := internal.ExecCommandWithSudo("dnf", append([]string{"remove", "-y"}, pkgs...)...); err != nil {
			internal.PrintYellow("dnf卸载失败: %v", err)
		}

	case internal.CommandExists("yum"):
		if _, err := internal.ExecCommandWithSudo("yum", append([]string{"remove", "-y"}, pkgs...)...); err != nil {
			internal.PrintYellow("yum卸载失败: %v", err)
		}
	}

	if _, err := internal.ExecCommandWithSudo(
		"rm", "-rf", "/etc/xray", "/usr/local/etc/xray", config.DefaultConfigDir); err != nil {
		internal.PrintYellow("清理残留文件失败: %v", err)
	}

	internal.PrintGreen("卸载完成")
}
