package service

import (
	"context"
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

var (
	warpCommandExists = internal.CommandExists
	warpGOARCH        = runtime.GOARCH
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
		return "", fmt.Errorf("cloudflare 没有为 %s 架构发布 WARP RPM；请改用 apt 系统或手动安装 warp-cli", goarch)
	}

	return fmt.Sprintf("https://pkg.cloudflareclient.com/pool/cloudflare-warp-el%s.x86_64.rpm", rhelVersion), nil
}

// SetupWarp 安装配置WARP代理.
func SetupWarp(cfg *config.Config) error {
	return SetupWarpContext(context.Background(), cfg)
}

// SetupWarpContext installs and configures WARP using ctx for runner-backed
// shell-outs. Existing callers use SetupWarp for the background-context default.
func SetupWarpContext(ctx context.Context, cfg *config.Config) error {
	return setupWarp(ctx, cfg, internal.DefaultRunner)
}

func setupWarp(ctx context.Context, cfg *config.Config, runner internal.CommandRunner) error {
	ctx = backgroundIfNil(ctx)

	internal.PrintYellow("正在部署 Cloudflare WARP 出口...")

	if !warpCommandExists("warp-cli") {
		if err := installWarp(ctx, runner); err != nil {
			return err
		}
	}

	statusOut, statusErr := runner.RunSilent(ctx, "systemctl", "is-active", internal.ServiceWarp)
	status := strings.TrimSpace(statusOut)

	if statusErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		if status == "" {
			status = internal.StatusInactive
		}
	}

	if status != internal.StatusActive {
		internal.PrintYellow("启动 warp-svc 守护进程...")

		if _, err := runner.RunWithSudo(ctx, "systemctl", "start", internal.ServiceWarp); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}

			internal.PrintYellow("启动warp-svc失败: %v", err)
		}

		if err := sleepWithContext(ctx, 3*time.Second); err != nil {
			return err
		}
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
	if _, err := runner.Run(ctx, "sh", "-c",
		`echo "y" | script -q -c "warp-cli registration new" /dev/null 2>&1`); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintYellow("WARP 注册步骤跳过 (已注册或暂时失败，后续命令会再验证)")
	}

	if _, err := runner.Run(ctx, "warp-cli", "mode", "proxy"); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintRed("设置WARP代理模式失败: %v", err)

		return err
	}

	if _, err := runner.Run(ctx, "warp-cli", "proxy", "port", fmt.Sprintf("%d", cfg.WARPPort)); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintRed("设置WARP端口失败: %v", err)

		return err
	}

	if _, err := runner.Run(ctx, "warp-cli", "connect"); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintRed("WARP启动失败: %v", err)

		return err
	}

	ip, err := internal.GetWarpIPContext(ctx, cfg.WARPPort)
	if err != nil {
		internal.PrintRed("WARP连通性测试失败: %v", err)
		return err
	}

	internal.PrintGreen("WARP部署成功，出口IP: %s", ip)

	return nil
}

// installWarp dispatches WARP installation to the detected package manager.
func installWarp(ctx context.Context, runner internal.CommandRunner) error {
	ctx = backgroundIfNil(ctx)

	switch {
	case warpCommandExists(pkgManagerAPT), warpCommandExists(pkgManagerAPTGet):
		aptCmd := pkgManagerAPT
		if warpCommandExists(pkgManagerAPTGet) {
			aptCmd = pkgManagerAPTGet
		}

		return installWarpApt(ctx, runner, aptCmd)

	case warpCommandExists(pkgManagerYUM):
		return installWarpYum(ctx, runner)
	case warpCommandExists(pkgManagerDNF):
		return installWarpDnf(ctx, runner)
	default:
		internal.PrintRed("未找到支持的包管理器（apt/yum/dnf）")
		return fmt.Errorf("no supported package manager found")
	}
}

func installWarpApt(ctx context.Context, runner internal.CommandRunner, aptCmd string) error {
	ctx = backgroundIfNil(ctx)

	// The GPG fetch genuinely needs a pipe between two processes; the URL is
	// a literal so there's no shell-injection surface. The runner uses
	// exec.CommandContext, so cancellation still stops the launched shell.
	if _, err := runner.Run(ctx, "bash", "-c",
		"curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | gpg --yes --dearmor -o "+warpGPGKeyring); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintRed("添加WARP GPG密钥失败: %v", err)

		return err
	}

	codename, err := readOSReleaseCodename()
	if err != nil {
		internal.PrintRed("读取发行版代号失败: %v", err)
		return err
	}

	// os.WriteFile avoids piping user-controlled codename through a shell.
	if err := os.WriteFile(warpAptSourcesList, []byte(warpAptRepoLine(codename)), 0o600); err != nil {
		internal.PrintRed("写入WARP源失败: %v", err)
		return err
	}

	if _, err := runner.RunWithSudo(ctx, aptCmd, "update"); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintRed("apt update 失败: %v", err)

		return err
	}

	if _, err := runner.RunWithSudo(ctx, aptCmd, "install", "-y", "cloudflare-warp"); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintRed("WARP安装失败: %v", err)

		return err
	}

	return nil
}

func installWarpYum(ctx context.Context, runner internal.CommandRunner) error {
	return installWarpRHEL(ctx, runner, pkgManagerYUM)
}

func installWarpDnf(ctx context.Context, runner internal.CommandRunner) error {
	return installWarpRHEL(ctx, runner, pkgManagerDNF)
}

func installWarpRHEL(ctx context.Context, runner internal.CommandRunner, pkgManager string) error {
	ctx = backgroundIfNil(ctx)

	// `rpm -E %rhel` prints the major RHEL version (e.g. "9"). We resolve it
	// here instead of relying on shell $() expansion inside an argv element.
	rhelOut, err := runner.Run(ctx, "rpm", "-E", "%rhel")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintRed("读取RHEL版本失败: %v", err)

		return err
	}

	rhel := strings.TrimSpace(rhelOut)
	if rhel == "" {
		return fmt.Errorf("empty RHEL version from rpm -E %%rhel")
	}

	url, err := warpRPMURL(rhel, warpGOARCH)
	if err != nil {
		internal.PrintRed("%v", err)
		return err
	}

	if _, err := runner.RunWithSudo(ctx, pkgManager, "install", "-y", url); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		internal.PrintRed("WARP安装失败: %v", err)

		return err
	}

	return nil
}

func backgroundIfNil(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}

	return ctx
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// RestartWarp 重启WARP.
func RestartWarp(cfg *config.Config) error {
	if _, err := internal.ExecCommand("warp-cli", "disconnect"); err != nil {
		internal.PrintYellow("断开WARP连接失败: %v", err)
	}

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

// readOSReleaseCodename 读取 /etc/os-release 中的 VERSION_CODENAME。
// 所有现代 Linux 发行版（Debian 8+/Ubuntu 16.04+）都自带此文件。
func readOSReleaseCodename() (string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", fmt.Errorf("读取 /etc/os-release 失败: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VERSION_CODENAME=") {
			continue
		}

		val := strings.TrimPrefix(line, "VERSION_CODENAME=")
		val = strings.Trim(val, `"`)
		val = strings.TrimSpace(val)

		if val != "" {
			return val, nil
		}
	}

	return "", fmt.Errorf("/etc/os-release 未找到 VERSION_CODENAME")
}
