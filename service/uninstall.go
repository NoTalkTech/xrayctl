package service

import (
	"xrayctl/config"
	"xrayctl/internal"
)

// Uninstall 卸载所有组件
func Uninstall() {
	internal.PrintYellow("正在卸载所有组件...")
	// 停止服务
	internal.StopService(internal.ServiceXray)
	internal.StopService(internal.ServiceNginx)

	// 卸载软件包
	if internal.CommandExists("apt") {
		internal.ExecCommandWithSudo("apt", "purge", "-y", "cloudflare-warp", "nginx", "xray")
	} else if internal.CommandExists("yum") || internal.CommandExists("dnf") {
		internal.ExecCommandWithSudo("yum", "remove", "-y", "cloudflare-warp", "nginx", "xray")
	}

	// 清理文件
	internal.ExecCommandWithSudo("rm", "-rf", "/etc/xray", "/usr/local/etc/xray", config.DefaultConfigDir)
	internal.PrintGreen("卸载完成")
}
