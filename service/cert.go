package service

import (
	"bufio"
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
)

// domainRe is a loose FQDN sanity check: labels of [A-Za-z0-9-], length 1-63 each,
// at least one dot, no leading/trailing hyphen. Not a full RFC 1035 validator —
// acme.sh will reject anything truly broken — this just catches fat-finger input.
var domainRe = regexp.MustCompile(`^(?i)[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

func validateDomain(s string) error {
	if !domainRe.MatchString(s) {
		return fmt.Errorf("无效域名 %q", s)
	}
	return nil
}

func validateEmail(s string) error {
	if _, err := mail.ParseAddress(s); err != nil {
		return fmt.Errorf("无效邮箱 %q: %w", s, err)
	}
	return nil
}

// promptValue prompts the user for a value (or confirms the current one),
// re-prompting until validate accepts the input. A nil validator accepts anything.
// Exits the process if stdin is closed (e.g. running --install non-interactively
// without first persisting domain/email) so we fail fast instead of spinning.
func promptValue(prompt, current string, validate func(string) error) string {
	reader := bufio.NewReader(os.Stdin)

	readLine := func() string {
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			internal.PrintRed("读取输入失败 (stdin 已关闭?): %v", err)
			internal.PrintRed("非交互运行请先用 --domain/--uuid 写入配置，或在交互模式下补全%s。", prompt)
			os.Exit(1)
		}
		return line
	}

	accept := func(v string) (string, bool) {
		if validate == nil {
			return v, true
		}
		if err := validate(v); err != nil {
			internal.PrintRed("%s格式无效: %v", prompt, err)
			return "", false
		}
		return v, true
	}

	if current != "" {
		internal.PrintGreenRaw("当前%s: %s\n", prompt, current)
		for {
			fmt.Print("是否使用此值？(Y/N): ")
			switch ans := strings.TrimSpace(readLine()); {
			case ans == "" || strings.EqualFold(ans, "Y"):
				if v, ok := accept(current); ok {
					return v
				}
				// invalid: fall through to manual entry
			case strings.EqualFold(ans, "N"):
				// fall through to manual entry
			default:
				continue
			}
			break
		}
	}

	for {
		internal.PrintYellowRaw("请输入%s: ", prompt)
		val := strings.TrimSpace(readLine())
		if val == "" {
			fmt.Println("输入不能为空")
			continue
		}
		if v, ok := accept(val); ok {
			return v
		}
	}
}

// SetupCert 申请并配置SSL证书
func SetupCert(cfg *config.Config) error {
	internal.PrintYellow("正在申请 TLS 证书...")

	oldDomain, oldEmail := cfg.Domain, cfg.Email
	cfg.Domain = promptValue("域名", cfg.Domain, validateDomain)
	cfg.Email = promptValue("邮箱", cfg.Email, validateEmail)
	if cfg.Domain != oldDomain || cfg.Email != oldEmail {
		if err := config.SaveConfig(cfg); err != nil {
			internal.PrintYellow("保存配置失败: %v", err)
		}
	}

	if err := ensureAcmeSh(); err != nil {
		return err
	}
	acmeCmd := internal.AcmeShPath

	if _, err := internal.ExecCommand(acmeCmd, "--set-default-ca", "--server", "letsencrypt"); err != nil {
		internal.PrintRed("设置CA失败: %v", err)
		return err
	}

	// acme.sh --standalone binds :80 itself, so nginx has to be down for the
	// duration. Restart it unconditionally on the way out.
	if _, err := internal.ExecCommandWithSudo("systemctl", "stop", internal.ServiceNginx); err != nil {
		internal.PrintYellow("停止 nginx 失败（可能未运行）: %v", err)
	}
	defer func() {
		if _, err := internal.ExecCommandWithSudo("systemctl", "start", internal.ServiceNginx); err != nil {
			internal.PrintYellow("重启 nginx 失败: %v", err)
		}
	}()

	if _, err := internal.ExecCommand(acmeCmd, "--issue", "-d", cfg.Domain, "--standalone"); err != nil {
		internal.PrintRed("证书申请失败: %v", err)
		return err
	}

	internal.MkdirIfNotExists(cfg.CertDir, 0o755)

	if _, err := internal.ExecCommand(acmeCmd, "--install-cert", "-d", cfg.Domain,
		"--key-file", fmt.Sprintf("%s/xray.key", cfg.CertDir),
		"--fullchain-file", fmt.Sprintf("%s/xray.crt", cfg.CertDir)); err != nil {
		internal.PrintRed("证书安装失败: %v", err)
		return err
	}

	internal.ExecCommandWithSudo("chown", "-R", "root:root", cfg.CertDir)
	internal.ExecCommandWithSudo("chmod", "644", fmt.Sprintf("%s/xray.crt", cfg.CertDir))
	internal.ExecCommandWithSudo("chmod", "600", fmt.Sprintf("%s/xray.key", cfg.CertDir))

	if _, err := internal.ExecCommand(acmeCmd, "--install-cronjob"); err != nil {
		internal.PrintYellow("自动续签配置失败，请手动配置")
	} else {
		internal.PrintGreen("证书自动续签已配置")
	}

	internal.PrintGreen("证书申请与配置完成")
	return nil
}

// ensureAcmeSh installs acme.sh if it's not already present at internal.AcmeShPath.
// It tries four delivery methods in sequence and returns the first error seen if
// all of them fail.
func ensureAcmeSh() error {
	if internal.FileExists(internal.AcmeShPath) {
		return nil
	}
	internal.MkdirIfNotExists(internal.AcmeRootPath, 0o755)
	internal.PrintYellow("开始安装acme.sh...")

	installMethods := []struct {
		name string
		cmd  string
	}{
		{"git安装", "git clone https://github.com/acmesh-official/acme.sh.git /root/.acme.sh && cd /root/.acme.sh && ./acme.sh --install --force"},
		{"curl安装", "curl -s https://get.acme.sh | sh -s -- --install --force"},
		{"wget安装", "wget -qO- https://get.acme.sh | sh -s -- --install --force"},
		{"直接下载安装", "cd /tmp && curl -s https://get.acme.sh > acme.sh && chmod +x acme.sh && ./acme.sh --install --force"},
	}

	var firstErr error
	for _, m := range installMethods {
		internal.PrintYellow("尝试%s...", m.name)
		if _, err := internal.ExecCommand("bash", "-c", m.cmd); err != nil {
			internal.PrintYellow("%s失败: %v", m.name, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		internal.PrintGreen("%s成功", m.name)
		if internal.FileExists(internal.AcmeShPath) {
			return nil
		}
	}

	internal.PrintRed("所有 acme.sh 安装方法都失败，请检查网络与 curl/wget/git")
	if firstErr != nil {
		return fmt.Errorf("install acme.sh: %w", firstErr)
	}
	return fmt.Errorf("acme.sh not found at %s after installation", internal.AcmeShPath)
}
