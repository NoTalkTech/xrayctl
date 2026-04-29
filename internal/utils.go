package internal

import (
	"crypto/rand"
	"fmt"
	"os"
)

// 颜色定义.
const (
	Red    = "\033[0;31m"
	Green  = "\033[0;32m"
	Yellow = "\033[0;33m"
	Reset  = "\033[0m"
)

// PrintRed 红色输出.
func PrintRed(format string, a ...interface{}) {
	fmt.Printf(Red+format+Reset+"\n", a...)
}

// PrintRedRaw 红色输出（无换行）.
func PrintRedRaw(format string, a ...interface{}) {
	fmt.Printf(Red+format+Reset, a...)
}

// PrintGreen 绿色输出.
func PrintGreen(format string, a ...interface{}) {
	fmt.Printf(Green+format+Reset+"\n", a...)
}

// PrintGreenRaw 绿色输出（无换行）.
func PrintGreenRaw(format string, a ...interface{}) {
	fmt.Printf(Green+format+Reset, a...)
}

// PrintYellow 黄色输出.
func PrintYellow(format string, a ...interface{}) {
	fmt.Printf(Yellow+format+Reset+"\n", a...)
}

// PrintYellowRaw 黄色输出（无换行）.
func PrintYellowRaw(format string, a ...interface{}) {
	fmt.Printf(Yellow+format+Reset, a...)
}

// GenerateUUID 生成UUID v4。crypto/rand 失败时直接 panic，因为沉默返回会
// 产出常数 UUID (全零 bytes + 固定 version/variant 位 = 同一个值)，而它正是
// VLESS inbound 的 client.id — 相当于把服务端认证彻底废掉。
func GenerateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("crypto/rand unavailable: %w", err))
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// IsRoot 检查是否是root用户运行.
func IsRoot() bool {
	return os.Geteuid() == 0
}
