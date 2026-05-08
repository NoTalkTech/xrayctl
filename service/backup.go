// Package service implements the installation and management logic for Xray,
// Nginx, Cloudflare WARP, and related system services.
package service

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"xrayctl/config"
	"xrayctl/internal"
)

// Backup 备份所有配置和证书.
func Backup(cfg *config.Config) error {
	internal.PrintYellow("正在备份数据...")

	backupFile := fmt.Sprintf("xrayctl-backup-%s.tar.gz", time.Now().Format("20060102-150405"))

	paths := []string{
		"/etc/xrayctl/",
		cfg.CertDir,
		cfg.XrayConfig,
		cfg.NginxConfig,
	}

	// Collect existing paths to back up.
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

	// The tarball contains the 0o600 TLS key and the UUID-bearing Xray config;
	// tar writes with the process umask (typically 0o022 → 0o644), so clamp it
	// down explicitly before anyone else on the host can read it.
	if err := os.Chmod(backupFile, 0o600); err != nil {
		internal.PrintYellow("备份文件权限收紧失败: %v", err)
	}

	internal.PrintGreen("备份成功，文件: %s", backupFile)

	return nil
}

// Restore 从备份文件恢复.
func Restore(backupFile string) error {
	internal.PrintYellow("正在从 %s 恢复...", backupFile)

	if !internal.FileExists(backupFile) {
		internal.PrintRed("备份文件不存在")
		return fmt.Errorf("backup file not exist")
	}

	prefixes, exactPaths := restorePrefixes()
	if err := validateTarForRootExtraction(backupFile, prefixes, exactPaths); err != nil {
		internal.PrintRed("备份文件不安全: %v", err)
		return err
	}

	// 停止服务（涵盖恢复目标的所有进程，含 warp-svc，避免恢复后状态错位）
	services := []string{internal.ServiceXray, internal.ServiceNginx, internal.ServiceWarp}
	stopArgs := append([]string{"stop"}, services...)
	startArgs := append([]string{"start"}, services...)

	if _, err := internal.ExecCommandWithSudo("systemctl", stopArgs...); err != nil {
		internal.PrintYellow("停止服务失败: %v", err)
	}

	// 解压到根目录
	_, err := internal.ExecCommand("tar", "-zxf", backupFile, "-C", "/")
	if err != nil {
		internal.PrintRed("恢复失败: %v", err)

		if _, restartErr := internal.ExecCommandWithSudo("systemctl", startArgs...); restartErr != nil {
			internal.PrintYellow("重启服务失败: %v", restartErr)

			return errors.Join(err, fmt.Errorf("restart services after failed restore: %w", restartErr))
		}

		return err
	}

	if _, err := internal.ExecCommandWithSudo("systemctl", startArgs...); err != nil {
		internal.PrintYellow("重启服务失败: %v", err)

		return fmt.Errorf("restart services after restore: %w", err)
	}

	internal.PrintGreen("恢复完成，所有服务已重启")

	return nil
}

// safeRestorePrefixes lists the directory prefixes that Restore is allowed to
// extract to /. Any entry in a restore tarball whose path falls outside these
// prefixes is rejected — this prevents an attacker-crafted or malformed archive
// from overwriting arbitrary root-owned files (e.g. /etc/passwd, /root/.ssh/…).
var safeRestorePrefixes = []string{
	"etc/xrayctl/",
	"etc/nginx/",
	"usr/local/etc/xray/",
	"etc/systemd/",
	"root/.acme.sh/",
	"etc/letsencrypt/",
}

// isPermittedRestorePath reports whether cleanPath (a cleaned archive entry
// path relative to /) is within one of the given restore prefixes.
//
// path.Clean strips the trailing slash from directory entries emitted by tar
// (e.g. "etc/xrayctl/" becomes "etc/xrayctl"), so we also accept the directory
// name itself when it exactly matches a prefix without its trailing "/".
func isPermittedRestorePath(cleanPath string, prefixes, exactPaths []string) bool {
	for _, exact := range exactPaths {
		if cleanPath == exact {
			return true
		}
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(cleanPath, prefix) {
			return true
		}

		namedDir := strings.TrimSuffix(prefix, "/")
		if cleanPath == namedDir {
			return true
		}
	}

	return false
}

// restorePrefixes returns the set of permitted restore prefixes, starting from
// the built-in safe prefixes and extending them with any custom paths from the
// on-disk config. This ensures Restore does not reject backups produced by
// Backup when the user has customized cert_dir, xray_config, or nginx_config.
//
//nolint:nonamedreturns // gocritic/unnamedResult wants names; nonamedreturns forbids them.
func restorePrefixes() (prefixes, exactPaths []string) {
	prefixes = make([]string, 0, len(safeRestorePrefixes))
	prefixes = append(prefixes, safeRestorePrefixes...)

	cfg, err := config.LoadConfigReadOnly()
	if err != nil {
		return prefixes, exactPaths
	}

	// CertDir is a directory — add its path as a prefix directly.
	if cfg.CertDir != "" {
		prefix := dirToArchivePrefix(cfg.CertDir)
		if prefix != "" && !slices.Contains(prefixes, prefix) {
			prefixes = append(prefixes, prefix)
		}
	}

	// XrayConfig and NginxConfig are files — add their exact archive paths.
	for _, p := range []string{cfg.XrayConfig, cfg.NginxConfig} {
		if p != "" {
			if exact := fileToArchivePath(p); exact != "" {
				exactPaths = append(exactPaths, exact)
			}
		}
	}

	return prefixes, exactPaths
}

// dirToArchivePrefix converts an absolute directory path to the archive prefix
// used during tar validation (e.g. "/etc/xrayctl" → "etc/xrayctl/").
func dirToArchivePrefix(p string) string {
	clean := path.Clean(p)
	if !strings.HasPrefix(clean, "/") || clean == "/" {
		return ""
	}

	return clean[1:] + "/"
}

// fileToArchivePath converts an absolute file path to its archive path
// (e.g. "/usr/local/etc/xray/config.json" → "usr/local/etc/xray/config.json").
func fileToArchivePath(p string) string {
	clean := path.Clean(p)
	if !strings.HasPrefix(clean, "/") || clean == "/" {
		return ""
	}

	return clean[1:]
}

func validateTarForRootExtraction(backupFile string, prefixes, exactPaths []string) error {
	f, err := os.Open(backupFile) //nolint:gosec // user-selected archive path must be opened for validation
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}

	defer func() {
		_ = f.Close() //nolint:errcheck // read-only validation cleanup
	}()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}

	defer func() {
		_ = gz.Close() //nolint:errcheck // read-only validation cleanup
	}()

	tr := tar.NewReader(gz)
	symlinkPaths := make(map[string]bool)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		if err := validateTarHeader(hdr, symlinkPaths, prefixes, exactPaths); err != nil {
			return err
		}
	}
}

func validateTarHeader(hdr *tar.Header, symlinkPaths map[string]bool, prefixes, exactPaths []string) error {
	cleanName, err := validateArchivePath(hdr.Name)
	if err != nil {
		return fmt.Errorf("unsafe archive path %q: %w", hdr.Name, err)
	}

	if pathUsesSymlinkPrefix(cleanName, symlinkPaths) {
		return fmt.Errorf("unsafe archive path %q: traverses archive symlink", hdr.Name)
	}

	if !isPermittedRestorePath(cleanName, prefixes, exactPaths) {
		return fmt.Errorf("archive path %q is not in permitted restore paths", hdr.Name)
	}

	switch hdr.Typeflag {
	case tar.TypeSymlink:
		if err := validateArchiveSymlink(hdr.Linkname); err != nil {
			return fmt.Errorf("unsafe symlink %q -> %q: %w", hdr.Name, hdr.Linkname, err)
		}

		symlinkPaths[cleanName] = true

	case tar.TypeLink:
		cleanLinkName, err := validateArchivePath(hdr.Linkname)
		if err != nil {
			return fmt.Errorf("unsafe hardlink %q -> %q: %w", hdr.Name, hdr.Linkname, err)
		}

		if pathUsesSymlinkPrefix(cleanLinkName, symlinkPaths) {
			return fmt.Errorf("unsafe hardlink %q -> %q: traverses archive symlink", hdr.Name, hdr.Linkname)
		}

		if !isPermittedRestorePath(cleanLinkName, prefixes, exactPaths) {
			return fmt.Errorf("unsafe hardlink %q -> %q: target not in permitted restore paths", hdr.Name, hdr.Linkname)
		}
	}

	return nil
}

func validateArchivePath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty path")
	}

	normalized := strings.ReplaceAll(name, "\\", "/")
	if path.IsAbs(normalized) {
		return "", fmt.Errorf("absolute path")
	}

	if hasParentTraversal(normalized) {
		return "", fmt.Errorf("parent traversal")
	}

	clean := path.Clean(normalized)
	if clean == "." {
		return "", fmt.Errorf("empty path")
	}

	return clean, nil
}

func validateArchiveSymlink(target string) error {
	if target == "" {
		return fmt.Errorf("empty target")
	}

	normalized := strings.ReplaceAll(target, "\\", "/")
	if path.IsAbs(normalized) {
		return fmt.Errorf("absolute target")
	}

	if hasParentTraversal(normalized) {
		return fmt.Errorf("parent traversal target")
	}

	return nil
}

func pathUsesSymlinkPrefix(name string, symlinkPaths map[string]bool) bool {
	for {
		if symlinkPaths[name] {
			return true
		}

		parent := path.Dir(name)
		if parent == "." || parent == "/" || parent == name {
			return false
		}

		name = parent
	}
}

func hasParentTraversal(name string) bool {
	return slices.Contains(strings.Split(name, "/"), "..")
}
