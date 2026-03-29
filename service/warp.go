package service

import (
	"fmt"
	"strings"
	"time"

	"xrayctl/config"
	"xrayctl/internal"
)

// SetupWarp 安装配置WARP代理
func SetupWarp(cfg *config.Config) error {
	internal.PrintYellow("正在部署 Cloudflare WARP 出口...")

	// 安装WARP源
	if !internal.CommandExists("warp-cli") {
		// 添加GPG密钥
		_, err := internal.ExecCommand("bash", "-c", "curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | gpg --yes --dearmor -o /usr/share/keyrings/cloudflare-warp.gpg")
		if err != nil {
			internal.PrintRed("添加WARP GPG密钥失败: %v", err)
			return err
		}

		// 获取发行版代号
		codename, _ := internal.ExecCommand("lsb_release", "-cs")
		codename = strings.TrimSpace(codename)

		// 添加源
		repoLine := fmt.Sprintf("deb [signed-by=/usr/share/keyrings/cloudflare-warp.gpg] https://pkg.cloudflareclient.com/ %s main", codename)
		_, err = internal.ExecCommand("bash", "-c", fmt.Sprintf("echo '%s' | tee /etc/apt/sources.list.d/cloudflare-client.list", repoLine))
		if err != nil {
			internal.PrintRed("添加WARP源失败: %v", err)
			return err
		}

		// 安装
		if internal.CommandExists("apt") {
			_, err = internal.ExecCommandWithSudo("apt", "update")
			_, err = internal.ExecCommandWithSudo("apt", "install", "-y", "cloudflare-warp")
		} else if internal.CommandExists("yum") || internal.CommandExists("dnf") {
			// CentOS系用rpm安装
			_, err = internal.ExecCommand("rpm", "-ivh", "https://pkg.cloudflareclient.com/pool/cloudflare-warp-el$(rpm -E %rhel).x86_64.rpm")
		}
		if err != nil {
			internal.PrintRed("WARP安装失败: %v", err)
			return err
		}
	}
	// 启动 warp-svc 守护进程（如果未运行）
	if status := internal.ServiceStatus("warp-svc"); status != internal.StatusActive {
		internal.PrintYellow("启动 warp-svc 守护进程...")
		// 尝试通过systemctl启动
		internal.ExecCommandWithSudo("systemctl", "start", "warp-svc")
		// 等待守护进程就绪
		time.Sleep(3 * time.Second)
	}

	// 注册WARP
	_, err := internal.ExecCommand("sh", "-c", `echo "y" | script -q -c "warp-cli registration new" /dev/null 2>&1`)
	if err != nil {
		// 已经注册过的话忽略错误
		internal.PrintYellow("WARP已注册，跳过注册步骤")
	}

	// 设置为代理模式
	_, err = internal.ExecCommand("warp-cli", "mode", "proxy")
	if err != nil {
		internal.PrintRed("设置WARP代理模式失败: %v", err)
		return err
	}

	// 设置代理端口
	_, err = internal.ExecCommand("warp-cli", "proxy", "port", fmt.Sprintf("%d", cfg.WARPPort))
	if err != nil {
		internal.PrintRed("设置WARP端口失败: %v", err)
		return err
	}

	// 启动WARP
	_, err = internal.ExecCommand("warp-cli", "connect")
	if err != nil {
		internal.PrintRed("WARP启动失败: %v", err)
		return err
	}

	// 测试连通性
	ip, err := internal.GetWarpIP(cfg.WARPPort)
	if err != nil {
		internal.PrintRed("WARP连通性测试失败: %v", err)
		return err
	}
	internal.PrintGreen("WARP部署成功，出口IP: %s", ip)
	return nil
}

// RestartWarp 重启WARP
func RestartWarp(cfg *config.Config) error {
	internal.ExecCommand("warp-cli", "disconnect")
	_, err := internal.ExecCommand("warp-cli", "connect")
	if err != nil {
		return err
	}
	// 测试连通性
	ip, err := internal.GetWarpIP(cfg.WARPPort)
	if err != nil {
		return err
	}
	internal.PrintGreen("WARP重启成功，出口IP: %s", ip)
	return nil
}

// WarpStatus 获取WARP运行状态
func WarpStatus() string {
	output, _ := internal.ExecCommandSilent("warp-cli", "status")
	return strings.TrimSpace(output)
}
