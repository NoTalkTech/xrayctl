package internal

import (
	"os"
	"testing"
)

func TestFileExists(t *testing.T) {
	// 测试存在的文件
	err := os.WriteFile("test.txt", []byte("test"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test.txt")

	if !FileExists("test.txt") {
		t.Error("Expected test.txt to exist")
	}

	// 测试不存在的文件
	if FileExists("non_existent_file.txt") {
		t.Error("Expected non_existent_file.txt to not exist")
	}
}

func TestDirExists(t *testing.T) {
	// 测试存在的目录
	err := os.Mkdir("test_dir", 0755)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("test_dir")

	if !DirExists("test_dir") {
		t.Error("Expected test_dir to exist")
	}

	// 测试不存在的目录
	if DirExists("non_existent_dir") {
		t.Error("Expected non_existent_dir to not exist")
	}
}

func TestMkdirIfNotExists(t *testing.T) {
	testDir := "test_mkdir"
	defer os.RemoveAll(testDir)

	// 第一次创建
	err := MkdirIfNotExists(testDir, 0755)
	if err != nil {
		t.Fatal(err)
	}
	if !DirExists(testDir) {
		t.Error("Directory not created")
	}

	// 第二次创建应该不会报错
	err = MkdirIfNotExists(testDir, 0755)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteAndReadFile(t *testing.T) {
	testFile := "test_write.txt"
	testContent := []byte("hello world")
	defer os.Remove(testFile)

	// 测试写入
	err := WriteFile(testFile, testContent, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 测试读取
	content, err := ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Expected content %s, got %s", testContent, content)
	}
}

func TestCopyFile(t *testing.T) {
	srcFile := "src_test.txt"
	dstFile := "dst_test.txt"
	testContent := []byte("copy test")
	defer os.Remove(srcFile)
	defer os.Remove(dstFile)

	// 写入源文件
	err := os.WriteFile(srcFile, testContent, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 复制
	err = CopyFile(srcFile, dstFile)
	if err != nil {
		t.Fatal(err)
	}

	// 验证目标文件
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Copy failed, expected %s, got %s", testContent, content)
	}
}