package internal

import (
	"runtime"
	"testing"
)

func TestCommandExists(t *testing.T) {
	// 测试存在的命令
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cmd"
	} else {
		cmd = "ls"
	}
	if !CommandExists(cmd) {
		t.Errorf("Command %s should exist", cmd)
	}

	// 测试不存在的命令
	if CommandExists("non_existent_command_1234") {
		t.Error("Non existent command should not exist")
	}
}

func TestExecCommand(t *testing.T) {
	var cmd string
	var args []string
	if runtime.GOOS == "windows" {
		cmd = "echo"
		args = []string{"hello"}
	} else {
		cmd = "echo"
		args = []string{"hello"}
	}
	output, err := ExecCommand(cmd, args...)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) == 0 {
		t.Error("Output should not be empty")
	}
	t.Logf("Command output: %s", output)
}