package internal

import "strings"

// ServiceStatus 获取systemd服务状态.
func ServiceStatus(name string) string {
	output, _ := ExecCommandSilent("systemctl", "is-active", name) //nolint:errcheck // best-effort status query
	return strings.TrimSpace(output)
}

// RestartService 重启systemd服务.
func RestartService(name string) error {
	_, err := ExecCommandWithSudo("systemctl", "restart", name)
	return err
}

// EnableService 设置服务开机自启.
func EnableService(name string) {
	ExecCommandWithSudo("systemctl", "enable", name) //nolint:errcheck,gosec // one-shot enable, best-effort
}

// StopService 停止服务.
func StopService(name string) {
	ExecCommandWithSudo("systemctl", "stop", name) //nolint:errcheck,gosec // one-shot stop, best-effort
}
