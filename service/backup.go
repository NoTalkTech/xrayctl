package service

import (
	"fmt"
	"strings"
	"time"

	"xrayctl/config"
	"xrayctl/internal"
)

// Backup 备份所有配置和证书
func Backup(cfg *config.Config) error {
	internal.PrintYellow("正在备份数据...")

	// 备份文件名：xrayctl-backup-日期.tar.gz
	backupFile := fmt.Sprintf("xrayctl-backup-%s.tar.gz", time.Now().Format("20060102-150405"))

	// 要备份的路径
	paths := []string{
		"/etc/xrayctl/",
		cfg.CertDir,
		cfg.XrayConfig,
		cfg.NginxConfig,
	}

	// 过滤不存在的路径
	var existPaths []string
	for _, p := range paths {
		if internal.FileExists(p) || internal.DirExists(p) {
			existPaths = append(existPaths, p)
		}
	}

	if len(existPaths) == 0 {
		internal.PrintRed("没有需要备份的文件")
		return nil
	}

	// 打包
	cmd := fmt.Sprintf("tar -zcf %s %s", backupFile, strings.Join(existPaths, " "))
	_, err := internal.ExecCommand("bash", "-c", cmd)
	if err != nil {
		internal.PrintRed("备份失败: %v", err)
		return err
	}

	internal.PrintGreen("备份成功，文件: %s", backupFile)
	return nil
}

// Restore 从备份文件恢复
func Restore(backupFile string) error {
	internal.PrintYellow("正在从 %s 恢复...", backupFile)

	if !internal.FileExists(backupFile) {
		internal.PrintRed("备份文件不存在")
		return fmt.Errorf("backup file not exist")
	}

	// 停止服务
	internal.ExecCommandWithSudo("systemctl", "stop", "xray", "nginx")

	// 解压到根目录
	_, err := internal.ExecCommand("tar", "-zxf", backupFile, "-C", "/")
	if err != nil {
		internal.PrintRed("恢复失败: %v", err)
		// 重启服务
		internal.ExecCommandWithSudo("systemctl", "start", "xray", "nginx")
		return err
	}

	// 重启服务
	internal.ExecCommandWithSudo("systemctl", "start", "xray", "nginx", "warp-svc")

	internal.PrintGreen("恢复完成，所有服务已重启")
	return nil
}
