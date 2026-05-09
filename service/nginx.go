package service

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"xrayctl/config"
	"xrayctl/internal"
)

//go:embed templates/nginx.conf.tmpl templates/vless.conf.tmpl
var nginxTemplates embed.FS

var (
	mainConfTmpl  = template.Must(template.ParseFS(nginxTemplates, "templates/nginx.conf.tmpl"))
	vlessConfTmpl = template.Must(template.ParseFS(nginxTemplates, "templates/vless.conf.tmpl"))
)

type nginxMainParams struct {
	User            string
	WorkerProcesses string
}

type nginxVlessParams struct {
	Domain      string
	NginxPort   int
	FallbackURL string
}

type nginxConfigBackup struct {
	dir   string
	files []nginxConfigBackupFile
}

type nginxConfigBackupFile struct {
	path       string
	backupPath string
	mode       os.FileMode
	existed    bool
}

// buildMainConf renders /etc/nginx/nginx.conf from the embedded template.
func buildMainConf(cfg *config.Config) string {
	user := cfg.NginxUser
	if user == "" {
		user = "nginx"
	}

	workers := cfg.NginxWorkerProcesses
	if workers == "" {
		workers = "auto"
	}

	var buf bytes.Buffer
	if err := mainConfTmpl.Execute(&buf, nginxMainParams{User: user, WorkerProcesses: workers}); err != nil {
		// Templates are embedded and tested — an error here is a programmer bug.
		panic(fmt.Errorf("render nginx main template: %w", err))
	}

	return buf.String()
}

// buildVlessConf renders cfg.NginxConfig from the embedded template.
func buildVlessConf(cfg *config.Config) string {
	var buf bytes.Buffer
	if err := vlessConfTmpl.Execute(&buf, nginxVlessParams{
		Domain:      cfg.Domain,
		NginxPort:   cfg.NginxPort,
		FallbackURL: cfg.FallbackURL,
	}); err != nil {
		panic(fmt.Errorf("render nginx vless template: %w", err))
	}

	return buf.String()
}

// SetupNginxMainConf 生成 Nginx 主配置文件.
func SetupNginxMainConf(cfg *config.Config) error {
	if err := atomicWriteFile(nginxMainConfigPath, []byte(buildMainConf(cfg)), 0o644); err != nil {
		internal.PrintRed("Nginx主配置写入失败: %v", err)
		return err
	}

	if err := internal.MkdirIfNotExists("/usr/share/nginx/html", 0o755); err != nil {
		internal.PrintYellow("创建Nginx静态目录失败: %v", err)
	}

	return nil
}

// SetupNginxVlessConf 生成 Vless 回落配置.
func SetupNginxVlessConf(cfg *config.Config) error {
	if cfg.Domain == "" {
		// Reuse the shared prompt helper so we get EOF-safe behavior and
		// domain validation instead of fmt.Scanln spinning on closed stdin.
		domain, err := promptValue("域名", "", validateDomain)
		if err != nil {
			return fmt.Errorf("读取 Nginx Vless 域名: %w", err)
		}

		cfg.Domain = domain

		if err := config.SaveConfig(cfg); err != nil {
			internal.PrintYellow("保存配置失败: %v", err)
		}
	}

	if err := atomicWriteFile(cfg.NginxConfig, []byte(buildVlessConf(cfg)), 0o644); err != nil {
		internal.PrintRed("Nginx Vless配置写入失败: %v", err)
		return err
	}

	return nil
}

// SetupNginx 配置 Nginx 回落.
func SetupNginx(cfg *config.Config) error {
	internal.PrintYellow("正在配置 Nginx 回落模块...")

	backup, err := backupNginxConfigs(nginxMainConfigPath, cfg.NginxConfig)
	if err != nil {
		internal.PrintRed("Nginx配置备份失败: %v", err)
		return err
	}
	defer backup.cleanup()

	if err := SetupNginxMainConf(cfg); err != nil {
		return rollbackNginxConfigs(backup, err, false)
	}

	if err := SetupNginxVlessConf(cfg); err != nil {
		return rollbackNginxConfigs(backup, err, false)
	}

	if _, err := internal.ExecCommandWithSudo("nginx", "-t"); err != nil {
		internal.PrintRed("Nginx配置错误: %v", err)
		return rollbackNginxConfigs(backup, err, false)
	}

	if err := internal.RestartService(internal.ServiceNginx); err != nil {
		internal.PrintRed("Nginx启动失败: %v", err)
		return rollbackNginxConfigs(backup, err, true)
	}

	internal.EnableService(internal.ServiceNginx)

	internal.PrintGreen("Nginx配置完成")

	return nil
}

// RestartNginx 重启 Nginx.
func RestartNginx() error {
	return internal.RestartService(internal.ServiceNginx)
}

// NginxStatus 获取 Nginx 运行状态.
func NginxStatus() string {
	return internal.ServiceStatus(internal.ServiceNginx)
}

func backupNginxConfigs(paths ...string) (*nginxConfigBackup, error) {
	dir, err := os.MkdirTemp("", "xrayctl-nginx-backup-*")
	if err != nil {
		return nil, fmt.Errorf("创建Nginx配置备份目录: %w", err)
	}

	backup := &nginxConfigBackup{dir: dir}
	seen := make(map[string]struct{}, len(paths))

	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}

		seen[path] = struct{}{}

		file := nginxConfigBackupFile{
			path:       path,
			backupPath: filepath.Join(dir, fmt.Sprintf("%d-%s", len(backup.files), filepath.Base(path))),
			mode:       0o644,
		}

		info, err := os.Stat(path)

		switch {
		case err == nil:
			if !info.Mode().IsRegular() {
				backup.cleanup()
				return nil, fmt.Errorf("备份Nginx配置 %s: 不是普通文件", path)
			}

			file.existed = true
			file.mode = info.Mode().Perm()

			if err := copyFileAtomic(path, file.backupPath, file.mode); err != nil {
				backup.cleanup()
				return nil, fmt.Errorf("备份Nginx配置 %s: %w", path, err)
			}

		case os.IsNotExist(err):
		default:
			backup.cleanup()
			return nil, fmt.Errorf("读取Nginx配置状态 %s: %w", path, err)
		}

		backup.files = append(backup.files, file)
	}

	return backup, nil
}

func rollbackNginxConfigs(
	backup *nginxConfigBackup,
	cause error,
	restartAfterRestore bool,
) error {
	internal.PrintRed("正在恢复Nginx配置备份...")

	if err := backup.restore(); err != nil {
		internal.PrintRed("Nginx配置回滚失败: %v", err)
		return errors.Join(cause, err)
	}

	if restartAfterRestore {
		if err := internal.RestartService(internal.ServiceNginx); err != nil {
			internal.PrintRed("恢复原配置后重启Nginx失败: %v", err)
			return errors.Join(cause, err)
		}
	}

	internal.PrintYellow("Nginx配置已恢复")

	return cause
}

func (b *nginxConfigBackup) restore() error {
	var restoreErr error

	for _, file := range b.files {
		if file.existed {
			if err := restoreNginxConfigFile(file); err != nil {
				restoreErr = errors.Join(restoreErr, err)
			}

			continue
		}

		if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
			restoreErr = errors.Join(
				restoreErr,
				fmt.Errorf("移除新增Nginx配置 %s: %w", file.path, err),
			)
		}
	}

	return restoreErr
}

func restoreNginxConfigFile(file nginxConfigBackupFile) error {
	data, err := os.ReadFile(file.backupPath)
	if err != nil {
		return fmt.Errorf("读取Nginx配置备份 %s: %w", file.path, err)
	}

	if err := atomicWriteFile(file.path, data, file.mode); err != nil {
		return fmt.Errorf("恢复Nginx配置 %s: %w", file.path, err)
	}

	return nil
}

func (b *nginxConfigBackup) cleanup() {
	_ = os.RemoveAll(b.dir) //nolint:errcheck // cleanup-only, error ignored
}

func copyFileAtomic(src, dst string, perm os.FileMode) error {
	data, err := os.ReadFile(src) //nolint:gosec // nginx config path comes from local xrayctl config.
	if err != nil {
		return err
	}

	return atomicWriteFile(dst, data, perm)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // nginx config dirs are conventional.
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}

	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath) //nolint:errcheck // cleanup-only, error ignored
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() //nolint:errcheck // close error ignored on write failure
		return err
	}

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close() //nolint:errcheck // close error ignored on chmod failure
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}
