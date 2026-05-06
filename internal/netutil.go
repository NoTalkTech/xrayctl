package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// 全局复用HTTP客户端.
var (
	defaultClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	portCheckClient = &http.Client{
		Timeout: 2 * time.Second,
	}
)

// fetchIP 公共IP获取函数.
func fetchIP(ctx context.Context, client *http.Client) (string, error) {
	ctx = contextOrBackground(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, IPCheckURL, http.NoBody)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close() //nolint:errcheck // closing HTTP response body

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(ip), nil
}

// GetPublicIP 获取公网IP.
func GetPublicIP() (string, error) {
	return fetchIP(context.Background(), defaultClient)
}

// GetWarpIP 通过WARP代理获取出口IP.
func GetWarpIP(port int) (string, error) {
	return GetWarpIPContext(context.Background(), port)
}

// GetWarpIPContext 通过WARP代理获取出口IP，并允许调用方取消HTTP探测.
func GetWarpIPContext(ctx context.Context, port int) (string, error) {
	ctx = contextOrBackground(ctx)

	proxyURL, err := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	return fetchIP(ctx, client)
}

// PortOpen 检测端口是否开放.
func PortOpen(port int) bool {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:%d", port), http.NoBody)
	if err != nil {
		return false
	}

	resp, err := portCheckClient.Do(req)
	if err != nil {
		return false
	}

	_ = resp.Body.Close() //nolint:errcheck // best-effort body close in port check

	return true
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}

	return ctx
}
