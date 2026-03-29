package service

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
)

func confirmOrInput(prompt string, current string, defaultFunc func() string) string {
	reader := bufio.NewReader(os.Stdin)
	// 情况1：已有值，询问是否保留
	if current != "" {
		internal.PrintGreenRaw("当前%s: %s\n", prompt, current)
		fmt.Print("是否使用此值？(Y/N): ")
		for {
			line, _ := reader.ReadString('\n')
			ans := strings.TrimSpace(line)
			if ans == "" || strings.EqualFold(ans, "Y") {
				return current
			} else if strings.EqualFold(ans, "N") {
				break
			}
			fmt.Print("输入无效，请输入 Y 或 N: ")
		}
	}

	// 情况2：无当前值，但有默认值生成函数
	if defaultFunc != nil {
		defaultVal := defaultFunc()
		if defaultVal != "" {
			internal.PrintGreenRaw("%s为空，默认使用: %s\n", prompt, defaultVal)
			fmt.Print("是否使用此值？(Y/N): ")
			for {
				line, _ := reader.ReadString('\n')
				ans := strings.TrimSpace(line)
				if ans == "" || strings.EqualFold(ans, "Y") {
					return defaultVal
				} else if strings.EqualFold(ans, "N") {
					break
				}
				fmt.Print("输入无效，请输入 Y 或 N: ")
			}
		}
	}

	// 手动输入
	var val string
	for {
		internal.PrintYellowRaw("请输入%s: ", prompt)
		fmt.Scan(&val)
		val = strings.TrimSpace(val)
		if val != "" {
			return val
		}
		fmt.Print("输入不能为空，请重新输入: ")
	}
}

// SetupCert 申请并配置SSL证书
func SetupCert(cfg *config.Config) error {
	internal.PrintYellow("正在申请 TLS 证书...")

	// 处理域名
	oldDomain := cfg.Domain
	cfg.Domain = confirmOrInput("域名", cfg.Domain, nil)

	// 处理邮箱（带默认值生成函数）
	oldEmail := cfg.Email
	cfg.Email = confirmOrInput("邮箱", cfg.Email, nil)

	// 修改后保存配置（如果值发生变化）
	if cfg.Domain != oldDomain || cfg.Email != oldEmail {
		config.SaveConfig(cfg)
	}

	// 安装acme.sh
	if !internal.DirExists(internal.AcmeRootPath) {
		internal.MkdirIfNotExists(internal.AcmeRootPath, 0755)
	}
	if !internal.FileExists(internal.AcmeShPath) {
		internal.PrintYellow("开始安装acme.sh...")

		// 尝试多种安装方法
		installMethods := []struct {
			name string
			cmd  string
		}{
			{
				name: "git安装",
				cmd:  "git clone https://github.com/acmesh-official/acme.sh.git /root/.acme.sh && cd /root/.acme.sh && ./acme.sh --install --force",
			},
			{
				name: "curl安装",
				cmd:  "curl -s https://get.acme.sh | sh -s -- --install --force",
			},
			{
				name: "wget安装",
				cmd:  "wget -qO- https://get.acme.sh | sh -s -- --install --force",
			},
			{
				name: "直接下载安装",
				cmd:  "cd /tmp && curl -s https://get.acme.sh > acme.sh && chmod +x acme.sh && ./acme.sh --install --force",
			},
		}

		var installErr error
		var installOutput string
		installed := false

		for _, method := range installMethods {
			internal.PrintYellow("尝试%s...", method.name)
			output, err := internal.ExecCommand("bash", "-c", method.cmd)
			if err == nil {
				installOutput = output
				internal.PrintGreen("%s成功", method.name)
				installed = true
				break
			}
			internal.PrintYellow("%s失败: %v", method.name, err)
			installErr = err
			installOutput = output
		}

		if !installed {
			internal.PrintRed("所有安装方法都失败: %v", installErr)
			internal.PrintRed("最后安装输出: %s", installOutput)
			// 尝试显示更多系统信息
			internal.ExecCommand("bash", "-c", "echo '检查系统信息...' && uname -a && which curl wget git 2>/dev/null || true")
			return installErr
		}

		// 等待一下让安装完成
		internal.ExecCommand("sleep", "3")

		// 检查目录内容
		internal.PrintYellow("检查安装目录内容...")
		lsOutput, _ := internal.ExecCommand("bash", "-c", "ls -la /root/.acme.sh/ 2>/dev/null || echo '目录不存在'")
		internal.PrintYellow("目录内容: %s", lsOutput)

		// 检查acme.sh文件是否存在
		internal.PrintYellow("检查acme.sh文件...")
		checkOutput, _ := internal.ExecCommand("bash", "-c", "find /root/.acme.sh/ -name 'acme.sh' -type f 2>/dev/null || echo '文件未找到'")
		internal.PrintYellow("查找结果: %s", checkOutput)

		// 检查是否安装到了其他位置
		internal.PrintYellow("检查可能的安装位置...")
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "/root"
		}
		altPath := fmt.Sprintf("%s/.acme.sh/acme.sh", homeDir)
		internal.PrintYellow("检查HOME目录: %s/.acme.sh/acme.sh", homeDir)

		// 再次检查文件是否存在
		if !internal.FileExists(internal.AcmeShPath) {
			// 尝试检查HOME目录
			if homeDir != "/root" && internal.FileExists(altPath) {
				internal.PrintYellow("acme.sh安装在 %s，更新路径...", altPath)
				// 更新内部路径（这里需要更复杂的处理，暂时跳过）
			} else {
				internal.PrintRed("acme.sh安装后文件仍未找到: %s", internal.AcmeShPath)
				// 尝试查找系统任何位置的acme.sh
				whichOutput, _ := internal.ExecCommand("bash", "-c", "which acme.sh 2>/dev/null || whereis acme.sh 2>/dev/null || echo '未在任何PATH中找到'")
				internal.PrintYellow("系统查找结果: %s", whichOutput)
				return fmt.Errorf("acme.sh not found after installation")
			}
		}
	}

	acmeCmd := internal.AcmeShPath

	// 设置默认CA为Let's Encrypt
	_, err := internal.ExecCommand(acmeCmd, "--set-default-ca", "--server", "letsencrypt")
	if err != nil {
		internal.PrintRed("设置CA失败: %v", err)
		return err
	}

	// 停止Nginx以便80端口申请
	internal.ExecCommandWithSudo("systemctl", "stop", "nginx")

	// 申请证书
	_, err = internal.ExecCommand(acmeCmd, "--issue", "-d", cfg.Domain, "--standalone")
	if err != nil {
		internal.PrintRed("证书申请失败: %v", err)
		// 恢复Nginx
		internal.ExecCommandWithSudo("systemctl", "start", "nginx")
		return err
	}

	// 启动Nginx
	internal.ExecCommandWithSudo("systemctl", "start", "nginx")

	// 创建证书目录
	internal.MkdirIfNotExists(cfg.CertDir, 0755)

	// 安装证书
	_, err = internal.ExecCommand(acmeCmd, "--install-cert", "-d", cfg.Domain,
		"--key-file", fmt.Sprintf("%s/xray.key", cfg.CertDir),
		"--fullchain-file", fmt.Sprintf("%s/xray.crt", cfg.CertDir))
	if err != nil {
		internal.PrintRed("证书安装失败: %v", err)
		return err
	}

	// 设置权限
	internal.ExecCommandWithSudo("chown", "-R", "root:root", cfg.CertDir)
	internal.ExecCommandWithSudo("chmod", "644", fmt.Sprintf("%s/xray.crt", cfg.CertDir))
	internal.ExecCommandWithSudo("chmod", "600", fmt.Sprintf("%s/xray.key", cfg.CertDir))

	// 配置自动续签
	_, err = internal.ExecCommand(acmeCmd, "--install-cronjob")
	if err != nil {
		internal.PrintYellow("自动续签配置失败，请手动配置")
	} else {
		internal.PrintGreen("证书自动续签已配置")
	}

	internal.PrintGreen("证书申请与配置完成")
	return nil
}
