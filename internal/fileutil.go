package internal

import (
	"os"
	"path/filepath"
)

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists 检查目录是否存在
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// MkdirIfNotExists 如果目录不存在则创建
func MkdirIfNotExists(path string, perm os.FileMode) error {
	if !DirExists(path) {
		return os.MkdirAll(path, perm)
	}
	return nil
}

// WriteFile 写入文件，自动创建目录
func WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := MkdirIfNotExists(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// ReadFile 读取文件内容
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	data, err := ReadFile(src)
	if err != nil {
		return err
	}
	return WriteFile(dst, data, 0644)
}
