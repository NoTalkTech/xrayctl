package internal

import (
	"bytes"
	"fmt"
	"os/exec"
)

// ExecCommand 执行shell命令，返回输出和错误（带日志）
func ExecCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 打印执行的命令
	fmt.Printf("[CMD] %s", name)
	for _, arg := range args {
		fmt.Printf(" %s", arg)
	}
	fmt.Println()

	err := cmd.Run()

	// 打印标准输出
	if stdout.String() != "" {
		fmt.Printf("[STDOUT] %s\n", stdout.String())
	}

	// 打印错误输出
	if err != nil {
		if stderr.String() != "" {
			fmt.Printf("[STDERR] %s\n", stderr.String())
		}
		return stderr.String(), err
	}

	return stdout.String(), nil
}

// ExecCommandSilent 执行shell命令，返回输出和错误（无日志）
func ExecCommandSilent(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}

// ExecCommandWithSudo 用sudo执行命令（自动检测当前是否是root）
func ExecCommandWithSudo(name string, args ...string) (string, error) {
	// 如果已经是root用户，直接执行
	if IsRoot() {
		return ExecCommand(name, args...)
	}
	// 非root用户，使用sudo
	fullArgs := append([]string{name}, args...)
	return ExecCommand("sudo", fullArgs...)
}

// CommandExists 检查命令是否存在
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}