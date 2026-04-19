package service

import (
	"fmt"
	"time"

	"xrayctl/config"
	"xrayctl/internal"
)

// Backup 备份所有配置和证书
func Backup(cfg *config.Config) error {
	internal.PrintYellow("正在备份数据...")

	backupFile := fmt.Sprintf("xrayctl-backup-%s.tar.gz", time.Now().Format("20060102-150405"))

	paths := []string{
		"/etc/xrayctl/",
		cfg.CertDir,
		cfg.XrayConfig,
		cfg.NginxConfig,
	}

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

	// Build argv directly instead of handing a concatenated command to bash -c,
	// so shell metacharacters in paths can never be interpreted.
	args := append([]string{"-zcf", backupFile}, existPaths...)
	if _, err := internal.ExecCommand("tar", args...); err != nil {
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

	// 停止服务（涵盖恢复目标的所有进程，含 warp-svc，避免恢复后状态错位）
	services := []string{internal.ServiceXray, internal.ServiceNginx, internal.ServiceWarp}
	stopArgs := append([]string{"stop"}, services...)
	startArgs := append([]string{"start"}, services...)
	internal.ExecCommandWithSudo("systemctl", stopArgs...)

	// 解压到根目录
	_, err := internal.ExecCommand("tar", "-zxf", backupFile, "-C", "/")
	if err != nil {
		internal.PrintRed("恢复失败: %v", err)
		internal.ExecCommandWithSudo("systemctl", startArgs...)
		return err
	}

	internal.ExecCommandWithSudo("systemctl", startArgs...)

	internal.PrintGreen("恢复完成，所有服务已重启")
	return nil
}
