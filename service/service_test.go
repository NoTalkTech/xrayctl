package service

import (
	"testing"

	"xrayctl/internal"
)

func TestServiceStatus(t *testing.T) {
	// 测试非交互式的状态查询
	status := internal.ServiceStatus("nginx")
	t.Logf("Nginx status: %s", status)
}

func TestRestartService(t *testing.T) {
	// 跳过需要root权限的测试
	t.Skip("Skip restart test - requires root permission")
	err := internal.RestartService("nginx")
	t.Logf("Restart result: %v", err)
}

func TestUninstall(t *testing.T) {
	// 跳过卸载测试，避免删除系统组件
	t.Skip("Skip uninstall test")
	Uninstall()
}
