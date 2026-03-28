package internal

// 通用常量定义
const (
	// IP查询服务
	IPCheckURL = "https://ifconfig.me"

	// 系统服务名
	ServiceNginx = "nginx"
	ServiceXray  = "xray"
	ServiceWarp  = "warp-svc"

	// 服务状态
	StatusActive   = "active"
	StatusInactive = "inactive"
	StatusConnected = "Connected"

	// 协议常量
	ProtocolVLESS   = "VLESS"
	ProtocolTLS     = "tls"
	ProtocolSOCKS   = "socks"
	ProtocolFreedom = "freedom"
	NetworkRaw      = "raw"
	FlowXTLSVision  = "xtls-rprx-vision"

	// 第三方工具路径
	AcmeShPath = "/root/.acme.sh/acme.sh"
)
