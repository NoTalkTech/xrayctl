package main

import "xrayctl/cli"

func main() {
	// 先解析命令行参数，如果有参数则执行，否则显示菜单
	if !cli.ParseFlags() {
		cli.ShowMenu()
	}
}