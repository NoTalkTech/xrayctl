package internal

import "testing"

func TestGenerateUUID(t *testing.T) {
	uuid := GenerateUUID()
	if len(uuid) != 36 {
		t.Errorf("UUID length expected 36, got %d", len(uuid))
	}
	t.Logf("Generated UUID: %s", uuid)
}

func TestColorOutput(t *testing.T) {
	// 测试颜色输出不会panic
	PrintRed("test red")
	PrintGreen("test green")
	PrintYellow("test yellow")
	t.Log("Color output test passed")
}
