package service

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
)

const statusProtocolLabel = "VLESS + XTLS-Vision"

// StatusReport contains collected health state without any terminal rendering.
type StatusReport struct {
	NginxStatus string
	XrayStatus  string
	WarpStatus  string
	WarpIP      string
	WarpIPError error
	Domain      string
	XrayPort    int
	UUID        string
	Protocol    string
	ShareLink   string
}

// CollectStatus collects service, WARP, and connection status for callers that
// need structured results instead of parsing terminal output.
func CollectStatus(cfg *config.Config) StatusReport {
	warpIP, err := internal.GetWarpIP(cfg.WARPPort)

	return buildStatusReport(cfg, NginxStatus(), XrayStatus(), WarpStatus(), warpIP, err)
}

func buildStatusReport(
	cfg *config.Config,
	nginxStatus string,
	xrayStatus string,
	warpStatus string,
	warpIP string,
	warpIPErr error,
) StatusReport {
	report := StatusReport{
		NginxStatus: nginxStatus,
		XrayStatus:  xrayStatus,
		WarpStatus:  warpStatus,
		WarpIP:      warpIP,
		WarpIPError: warpIPErr,
		Domain:      cfg.Domain,
		XrayPort:    cfg.XrayPort,
		UUID:        cfg.UUID,
		Protocol:    statusProtocolLabel,
	}

	if report.HasConnectionParameters() {
		endpoint := net.JoinHostPort(report.Domain, strconv.Itoa(report.XrayPort))
		report.ShareLink = fmt.Sprintf(
			"vless://%s@%s?flow=%s&security=%s&type=tcp#Xray-WARP",
			report.UUID,
			endpoint,
			internal.FlowXTLSVision,
			internal.ProtocolTLS,
		)
	}

	return report
}

// HasConnectionParameters reports whether the VLESS connection details can be rendered.
func (r StatusReport) HasConnectionParameters() bool {
	return r.UUID != "" && r.Domain != ""
}

// HasFailures reports whether status validation would fail.
func (r StatusReport) HasFailures() bool {
	return len(r.failures()) > 0
}

// ValidationError returns an error for validation contexts such as post-install checks.
func (r StatusReport) ValidationError() error {
	failures := r.failures()
	if len(failures) == 0 {
		return nil
	}

	return fmt.Errorf("status validation failed: %s", strings.Join(failures, ", "))
}

func (r StatusReport) failures() []string {
	failures := make([]string, 0, 4)

	if r.NginxStatus != internal.StatusActive {
		failures = append(failures, fmt.Sprintf("nginx=%s", statusValue(r.NginxStatus)))
	}

	if r.XrayStatus != internal.StatusActive {
		failures = append(failures, fmt.Sprintf("xray=%s", statusValue(r.XrayStatus)))
	}

	if r.WarpStatus != internal.StatusActive {
		failures = append(failures, fmt.Sprintf("warp-svc=%s", statusValue(r.WarpStatus)))
	}

	if r.WarpIPError != nil {
		failures = append(failures, fmt.Sprintf("warp ip probe: %v", r.WarpIPError))
	}

	return failures
}

func statusValue(status string) string {
	if status == "" {
		return "unknown"
	}

	return status
}

// PrintStatusReport renders a collected status report with the existing Chinese CLI output.
func PrintStatusReport(report StatusReport) {
	internal.PrintYellow("\n>>> 系统健康检查 <<<")

	printStatusLine("Nginx: ", report.NginxStatus, "运行中", "未运行")
	printStatusLine("Xray:  ", report.XrayStatus, "运行中", "未运行")
	printStatusLine("WARP:  ", report.WarpStatus, "已连接", "未连接")

	internal.PrintYellow("\n>>> 物理链路测试 <<<")

	if report.WarpIPError == nil {
		internal.PrintGreen("WARP 出口正常 (IP: %s)", report.WarpIP)
	} else {
		internal.PrintRed("WARP 链路中断，请检查 warp-cli 状态")
	}

	if report.HasConnectionParameters() {
		internal.PrintYellow("\n>>> 连接参数 <<<")
		internal.PrintGreen("域名: %s", report.Domain)
		internal.PrintGreen("端口: %d", report.XrayPort)
		internal.PrintGreen("UUID: %s", report.UUID)
		internal.PrintGreen("协议: %s", report.Protocol)
		internal.PrintGreen("分享链接: %s", report.ShareLink)
	}
}

func printStatusLine(prefix, status, activeText, inactiveText string) {
	fmt.Print(prefix)

	if status == internal.StatusActive {
		internal.PrintGreen(activeText)
	} else {
		internal.PrintRed(inactiveText)
	}
}

// CheckStatus 检查所有服务状态.
func CheckStatus(cfg *config.Config) {
	PrintStatusReport(CollectStatus(cfg))
}
