package service

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"xrayctl/config"
	"xrayctl/internal"
)

const (
	warpAptSourcesList = "/etc/apt/sources.list.d/cloudflare-client.list"
	warpGPGKeyring     = "/usr/share/keyrings/cloudflare-warp.gpg"
)

// warpAptRepoLine builds the single-line apt source entry for the Cloudflare
// WARP repo, pinned to the given distro codename.
func warpAptRepoLine(codename string) string {
	return fmt.Sprintf("deb [signed-by=%s] https://pkg.cloudflareclient.com/ %s main\n", warpGPGKeyring, codename)
}

// warpRPMURL builds the download URL for the RHEL-family WARP RPM for the
// given major RHEL version (e.g. "8", "9") and Go arch (e.g. "amd64").
// Cloudflare only publishes x86_64 RPMs in the public pool, so any other arch
// is rejected with a clear error rather than silently 404'ing later.
func warpRPMURL(rhelVersion, goarch string) (string, error) {
	if goarch != "amd64" {
		return "", fmt.Errorf("Cloudflare 没有为 %s 架构发布 WARP RPM；请改用 apt 系统或手动安装 warp-cli", goarch)
	}
	return fmt.Sprintf("https://pkg.cloudflareclient.com/pool/cloudflare-warp-el%s.x86_64.rpm", rhelVersion), nil
}

// SetupWarp 安装配置WARP代理
func SetupWarp(cfg *config.Config) error {
	internal.PrintYellow("正在部署 Cloudflare WARP 出口...")

	if !internal.CommandExists("warp-cli") {
		if err := installWarp(); err != nil {
			return err
		}
	}

	if status := internal.ServiceStatus(internal.ServiceWarp); status != internal.StatusActive {
		internal.PrintYellow("启动 warp-svc 守护进程...")
		internal.ExecCommandWithSudo("systemctl", "start", internal.ServiceWarp)
		time.Sleep(3 * time.Second)
	}

	// `warp-cli registration new` prompts Y/N on a TTY to accept TOS; piping
	// "y" into it directly doesn't work because warp-cli refuses to read
	// from a non-terminal. `script -q` wraps the invocation in a fake pty
	// so the echo goes through. If upstream adds a flag like --accept-tos,
	// replace this with the proper non-interactive form.
	//
	// We don't error out on a non-zero exit: the common case is "already
	// registered" which returns non-zero, and the next mandatory step
	// (`warp-cli mode proxy`) will surface any real problem.
	if _, err := internal.ExecCommand("sh", "-c", `echo "y" | script -q -c "warp-cli registration new" /dev/null 2>&1`); err != nil {
		internal.PrintYellow("WARP 注册步骤跳过 (已注册或暂时失败，后续命令会再验证)")
	}

	if _, err := internal.ExecCommand("warp-cli", "mode", "proxy"); err != nil {
		internal.PrintRed("设置WARP代理模式失败: %v", err)
		return err
	}

	if _, err := internal.ExecCommand("warp-cli", "proxy", "port", fmt.Sprintf("%d", cfg.WARPPort)); err != nil {
		internal.PrintRed("设置WARP端口失败: %v", err)
		return err
	}

	if _, err := internal.ExecCommand("warp-cli", "connect"); err != nil {
		internal.PrintRed("WARP启动失败: %v", err)
		return err
	}

	ip, err := internal.GetWarpIP(cfg.WARPPort)
	if err != nil {
		internal.PrintRed("WARP连通性测试失败: %v", err)
		return err
	}
	internal.PrintGreen("WARP部署成功，出口IP: %s", ip)
	return nil
}

// installWarp adds the Cloudflare apt/rpm repo and installs the cloudflare-warp
// package. It dispatches by package manager: apt on Debian/Ubuntu, rpm on
// RHEL-family hosts.
func installWarp() error {
	// The GPG fetch genuinely needs a pipe between two processes; the URL is
	// a literal so there's no shell-injection surface.
	if _, err := internal.ExecCommand("bash", "-c",
		"curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | gpg --yes --dearmor -o "+warpGPGKeyring); err != nil {
		internal.PrintRed("添加WARP GPG密钥失败: %v", err)
		return err
	}

	switch {
	case internal.CommandExists("apt"):
		return installWarpApt()
	case internal.CommandExists("yum") || internal.CommandExists("dnf"):
		return installWarpRPM()
	default:
		internal.PrintRed("未找到支持的包管理器（apt/yum/dnf）")
		return fmt.Errorf("no supported package manager found")
	}
}

func installWarpApt() error {
	codenameOut, err := internal.ExecCommand("lsb_release", "-cs")
	if err != nil {
		internal.PrintRed("读取发行版代号失败: %v", err)
		return err
	}
	codename := strings.TrimSpace(codenameOut)
	if codename == "" {
		return fmt.Errorf("empty distro codename from lsb_release")
	}

	// os.WriteFile avoids piping user-controlled codename through a shell.
	if err := os.WriteFile(warpAptSourcesList, []byte(warpAptRepoLine(codename)), 0o644); err != nil {
		internal.PrintRed("写入WARP源失败: %v", err)
		return err
	}

	if _, err := internal.ExecCommandWithSudo("apt", "update"); err != nil {
		internal.PrintRed("apt update 失败: %v", err)
		return err
	}
	if _, err := internal.ExecCommandWithSudo("apt", "install", "-y", "cloudflare-warp"); err != nil {
		internal.PrintRed("WARP安装失败: %v", err)
		return err
	}
	return nil
}

func installWarpRPM() error {
	// `rpm -E %rhel` prints the major RHEL version (e.g. "9"). We resolve it
	// here instead of relying on shell $() expansion inside an argv element.
	rhelOut, err := internal.ExecCommand("rpm", "-E", "%rhel")
	if err != nil {
		internal.PrintRed("读取RHEL版本失败: %v", err)
		return err
	}
	rhel := strings.TrimSpace(rhelOut)
	if rhel == "" {
		return fmt.Errorf("empty RHEL version from rpm -E %%rhel")
	}

	url, err := warpRPMURL(rhel, runtime.GOARCH)
	if err != nil {
		internal.PrintRed("%v", err)
		return err
	}
	if _, err := internal.ExecCommand("rpm", "-ivh", url); err != nil {
		internal.PrintRed("WARP安装失败: %v", err)
		return err
	}
	return nil
}

// RestartWarp 重启WARP
func RestartWarp(cfg *config.Config) error {
	internal.ExecCommand("warp-cli", "disconnect")
	if _, err := internal.ExecCommand("warp-cli", "connect"); err != nil {
		return err
	}
	ip, err := internal.GetWarpIP(cfg.WARPPort)
	if err != nil {
		return err
	}
	internal.PrintGreen("WARP重启成功，出口IP: %s", ip)
	return nil
}

// WarpStatus 获取WARP运行状态。
// 返回 systemd 对 warp-svc 的 is-active 状态（与其他服务一致），
// 调用方应与 internal.StatusActive 比较。
func WarpStatus() string {
	return internal.ServiceStatus(internal.ServiceWarp)
}
