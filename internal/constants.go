package internal

// 通用常量定义
const (
	// IP查询服务
	IPCheckURL = "https://ifconfig.me/ip"
	// // 备用IP查询服务列表
	// IPCheckServices = "https://ifconfig.me/ip,https://api.ipify.org,https://checkip.amazonaws.com,https://ipinfo.io/ip"

	// 系统服务名
	ServiceNginx = "nginx"
	ServiceXray  = "xray"
	ServiceWarp  = "warp-svc"

	// 服务状态
	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusFailed    = "failed"
	StatusConnected = "Connected"

	// 协议常量
	ProtocolVLESS   = "VLESS"
	ProtocolTLS     = "tls"
	ProtocolSOCKS   = "socks"
	ProtocolFreedom = "freedom"
	NetworkRaw      = "raw"
	FlowXTLSVision  = "xtls-rprx-vision"

	// 第三方工具路径
	AcmeRootPath = "/root/.acme.sh"
	AcmeShPath   = "/root/.acme.sh/acme.sh"
)
