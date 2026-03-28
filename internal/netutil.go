package internal

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// 全局复用HTTP客户端
var (
	defaultClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	portCheckClient = &http.Client{
		Timeout: 2 * time.Second,
	}
)

// fetchIP 公共IP获取函数
func fetchIP(client *http.Client) (string, error) {
	resp, err := client.Get(IPCheckURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(ip), nil
}

// GetPublicIP 获取公网IP
func GetPublicIP() (string, error) {
	return fetchIP(defaultClient)
}

// GetWarpIP 通过WARP代理获取出口IP
func GetWarpIP(port int) (string, error) {
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
	return fetchIP(client)
}

// PortOpen 检测端口是否开放
func PortOpen(port int) bool {
	_, err := portCheckClient.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	return err == nil
}