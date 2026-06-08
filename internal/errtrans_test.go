package internal

import (
	"errors"
	"testing"
)

func TestTranslateError_PackageNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "Debian apt",
			err:  errors.New("E: Unable to locate package nginx"),
			want: "软件包未找到 — 请确认操作系统版本受支持（Debian 11+ / Ubuntu 20.04+ / CentOS Stream 8+），然后运行 apt update 后重试",
		},
		{
			name: "CentOS yum",
			err:  errors.New("Error: Unable to find a match: nginx"),
			want: "软件包未找到 — 请确认操作系统版本受支持（Debian 11+ / Ubuntu 20.04+ / CentOS Stream 8+），然后运行 apt update 后重试",
		},
		{
			name: "Ubuntu apt",
			err:  errors.New("No package nginx available"),
			want: "软件包未找到 — 请确认操作系统版本受支持（Debian 11+ / Ubuntu 20.04+ / CentOS Stream 8+），然后运行 apt update 后重试",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateError(tt.err)
			if got != tt.want {
				t.Errorf("TranslateError(%q) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestTranslateError_PortConflict(t *testing.T) {
	err := errors.New("listen tcp :80: bind: address already in use")
	want := "端口 80 已被占用 — 可能有其他 Web 服务器（Nginx/Apache）正在运行。请先停止占用端口的服务后重试"
	got := TranslateError(err)
	if got != want {
		t.Errorf("TranslateError(%q) = %q, want %q", err, got, want)
	}
}

func TestTranslateError_DNSFailure(t *testing.T) {
	err := errors.New("Verify error: Invalid response from https://acme-v02.api.letsencrypt.org")
	want := "证书申请失败 — 请确认域名已正确解析到此服务器的 IP 地址。DNS 记录生效可能需要几分钟"
	got := TranslateError(err)
	if got != want {
		t.Errorf("TranslateError(%q) = %q, want %q", err, got, want)
	}
}

func TestTranslateError_UnknownPassthrough(t *testing.T) {
	err := errors.New("some unrecognized error message")
	got := TranslateError(err)
	if got != err.Error() {
		t.Errorf("TranslateError(%q) = %q, want passthrough of original %q", err, got, err.Error())
	}
}
