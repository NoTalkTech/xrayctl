package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
	"xrayctl/service"
)

// installCompleteMarker is the sentinel file touched after a successful install.
// Set as a var so tests can override it.
var installCompleteMarker = internal.InstallCompleteMarker

const defaultNginxUser = "nginx"

// ShouldShowWizard returns true when the guided wizard should run instead of
// the main menu. Exported so cmd/main.go can route without importing config.
func ShouldShowWizard() bool {
	if _, err := os.Stat(config.ConfigPath); os.IsNotExist(err) {
		return true
	}

	if _, err := config.LoadConfigReadOnly(); err != nil {
		return true
	}

	if _, err := os.Stat(installCompleteMarker); os.IsNotExist(err) {
		return true
	}

	return false
}

// ShowGuidedSetup runs the guided first-run wizard.
func ShowGuidedSetup() {
	if !internal.IsRoot() {
		internal.PrintRed("错误: 请使用 root 运行！")
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)

	internal.PrintGreen("==========================================")
	internal.PrintGreen("|         欢迎使用 xrayctl 安装向导       |")
	internal.PrintGreen("|      让我们一步步完成 VPN 服务部署      |")
	internal.PrintGreen("==========================================")
	fmt.Println()

	// Step 1: Domain
	domain := promptDomain(scanner)

	// Step 2: Email
	email := promptEmail(scanner)

	// Pre-flight checks
	fmt.Println()
	internal.PrintYellow(">>> 环境预检 <<<")
	showPreflightWarnings(domain)

	// Confirmation
	fmt.Println()
	internal.PrintGreen("==========================================")
	internal.PrintGreen("  安装确认")
	internal.PrintGreen("==========================================")
	fmt.Printf("  域名:   %s\n", domain)
	fmt.Printf("  邮箱:   %s\n", email)
	fmt.Println()
	fmt.Print("确认开始安装？(Y/n): ")

	if !scanner.Scan() {
		fmt.Println()
		internal.PrintYellow("安装已取消。")

		return
	}

	confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))

	if confirm != "" && confirm != "y" && confirm != "yes" {
		internal.PrintYellow("安装已取消。")

		return
	}

	// Build config and install
	cfg := &config.Config{
		Domain: domain,
		Email:  email,
	}
	config.ApplyDefaults(cfg)

	fmt.Println()

	if err := service.InstallAll(cfg); err != nil {
		internal.PrintRed("安装失败: %v", internal.TranslateError(err))
		fmt.Println()
		internal.PrintYellow("常见问题排查：")
		internal.PrintYellow("  1. 确认域名已正确解析到此服务器 IP")
		internal.PrintYellow("  2. 确认 80/443 端口未被其他服务占用")
		internal.PrintYellow("  3. 运行 sudo ./xrayctl 重新开始安装")

		return
	}

	// Mark install as complete
	if err := touchInstallComplete(); err != nil {
		internal.PrintYellow("无法写入安装标记: %v", err)
	}

	printPostInstallGuidance()
}

func promptDomain(scanner *bufio.Scanner) string {
	for {
		fmt.Print("请输入你的域名 (如 vpn.example.com): ")

		if !scanner.Scan() {
			fmt.Println()
			internal.PrintYellow("输入已结束，退出向导。")
			os.Exit(0)
		}

		domain := strings.TrimSpace(scanner.Text())

		if domain == "" {
			internal.PrintRed("域名不能为空，请重新输入。")

			continue
		}

		return domain
	}
}

func promptEmail(scanner *bufio.Scanner) string {
	for {
		fmt.Print("请输入证书申请邮箱 (acme.sh 注册用): ")

		if !scanner.Scan() {
			fmt.Println()
			internal.PrintYellow("输入已结束，退出向导。")
			os.Exit(0)
		}

		email := strings.TrimSpace(scanner.Text())

		if email == "" {
			internal.PrintRed("邮箱不能为空，请重新输入。")

			continue
		}

		return email
	}
}

func showPreflightWarnings(domain string) {
	// Check if domain resolves (best-effort advisory).
	internal.PrintYellow("域名解析检查：请在浏览器确认 %s 已指向本服务器 IP", domain)

	// Distro nginx user check.
	passwdBytes, err := os.ReadFile("/etc/passwd")
	if err == nil {
		detectedUser := detectNginxUserFromPasswd(string(passwdBytes))
		if detectedUser != defaultNginxUser {
			internal.PrintYellow("检测到当前系统 Nginx 用户为 '%s'（非默认 '%s'）", detectedUser, defaultNginxUser)
		}
	}
}

// detectNginxUserFromPasswd scans passwd content and returns the appropriate
// nginx user: "www-data" if present (Debian/Ubuntu), "nginx" otherwise.
func detectNginxUserFromPasswd(passwdContent string) string {
	for _, line := range strings.Split(passwdContent, "\n") {
		if strings.HasPrefix(line, "www-data:") {
			return "www-data"
		}
	}

	return defaultNginxUser
}

// touchInstallComplete creates the marker file indicating a successful install.
func touchInstallComplete() error {
	dir := config.DefaultConfigDir

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	//nolint:gosec // installCompleteMarker is a package-level var (not user input), set for testability
	f, err := os.Create(installCompleteMarker)
	if err != nil {
		return fmt.Errorf("create marker: %w", err)
	}

	defer f.Close()

	return nil
}

// printPostInstallGuidance prints the post-install usage instructions.
func printPostInstallGuidance() {
	fmt.Println()
	internal.PrintGreen("==========================================")
	internal.PrintGreen("|        ✅ 安装完成！                   |")
	internal.PrintGreen("==========================================")
	fmt.Println()
	internal.PrintGreen("🎉 VPN 服务已成功部署！")
	fmt.Println()
	fmt.Println("接下来你可以：")
	fmt.Println("  sudo ./xrayctl              — 打开管理菜单，查看服务状态")
	fmt.Println("  sudo ./xrayctl --status     — 查看连接参数和分享链接")
	fmt.Println("  sudo ./xrayctl --backup     — 备份所有配置和证书")
	fmt.Println()
	internal.PrintYellow("提示：如需修改分流域名，编辑 /etc/xrayctl/config.yaml 后重启 Xray 即可。")
}
