package internal

import (
	"testing"
)

func TestGetPublicIP(t *testing.T) {
	// 测试公网IP获取，可能因为网络问题失败，标记为可选
	t.Skip("Skipping public IP test in offline environment")
	ip, err := GetPublicIP()
	if err != nil {
		t.Logf("GetPublicIP failed (expected if offline): %v", err)
		return
	}
	t.Logf("Public IP: %s", ip)
}

func TestPortOpen(t *testing.T) {
	// 测试未开放的端口
	if PortOpen(9999) {
		t.Error("Port 9999 should not be open")
	}
}