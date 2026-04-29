package service

import (
	"fmt"

	"xrayctl/config"
	"xrayctl/internal"
)

// CheckStatus 检查所有服务状态.
func CheckStatus(cfg *config.Config) {
	internal.PrintYellow("\n>>> 系统健康检查 <<<")

	fmt.Print("Nginx: ")

	if NginxStatus() == internal.StatusActive {
		internal.PrintGreen("运行中")
	} else {
		internal.PrintRed("未运行")
	}

	fmt.Print("Xray:  ")

	if XrayStatus() == internal.StatusActive {
		internal.PrintGreen("运行中")
	} else {
		internal.PrintRed("未运行")
	}

	fmt.Print("WARP:  ")

	if WarpStatus() == internal.StatusActive {
		internal.PrintGreen("已连接")
	} else {
		internal.PrintRed("未连接")
	}

	internal.PrintYellow("\n>>> 物理链路测试 <<<")

	warpIP, err := internal.GetWarpIP(cfg.WARPPort)
	if err == nil {
		internal.PrintGreen("WARP 出口正常 (IP: %s)", warpIP)
	} else {
		internal.PrintRed("WARP 链路中断，请检查 warp-cli 状态")
	}

	// 显示连接参数
	if cfg.UUID != "" && cfg.Domain != "" {
		internal.PrintYellow("\n>>> 连接参数 <<<")
		internal.PrintGreen("域名: %s", cfg.Domain)
		internal.PrintGreen("端口: %d", cfg.XrayPort)
		internal.PrintGreen("UUID: %s", cfg.UUID)
		internal.PrintGreen("协议: VLESS + XTLS-Vision")
		// 生成分享链接
		shareLink := "vless://" + cfg.UUID + "@" + cfg.Domain + ":" +
			fmt.Sprintf("%d?flow=%s&security=%s&type=tcp#Xray-WARP",
				cfg.XrayPort, internal.FlowXTLSVision, internal.ProtocolTLS)
		internal.PrintGreen("分享链接: %s", shareLink)
	}
}
