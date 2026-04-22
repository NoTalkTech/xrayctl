package service

import (
	"xrayctl/config"
	"xrayctl/internal"
)

// Uninstall 卸载所有组件
func Uninstall() {
	internal.PrintYellow("正在卸载所有组件...")
	internal.StopService(internal.ServiceXray)
	internal.StopService(internal.ServiceNginx)
	internal.StopService(internal.ServiceWarp)

	pkgs := []string{"cloudflare-warp", internal.ServiceNginx, internal.ServiceXray}
	switch {
	case internal.CommandExists("apt"):
		internal.ExecCommandWithSudo("apt", append([]string{"purge", "-y"}, pkgs...)...)
	case internal.CommandExists("dnf"):
		internal.ExecCommandWithSudo("dnf", append([]string{"remove", "-y"}, pkgs...)...)
	case internal.CommandExists("yum"):
		internal.ExecCommandWithSudo("yum", append([]string{"remove", "-y"}, pkgs...)...)
	}

	internal.ExecCommandWithSudo("rm", "-rf", "/etc/xray", "/usr/local/etc/xray", config.DefaultConfigDir)
	internal.PrintGreen("卸载完成")
}
