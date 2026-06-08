package internal

import (
	"regexp"
)

// errPattern maps a compiled regex to a user-facing Chinese explanation.
type errPattern struct {
	re   *regexp.Regexp
	text string
}

var patterns = []errPattern{
	{
		re:   regexp.MustCompile(`(Unable to locate package|No package .* available|Unable to find a match)`),
		text: "软件包未找到 — 请确认操作系统版本受支持（Debian 11+ / Ubuntu 20.04+ / CentOS Stream 8+），然后运行 apt update 后重试",
	},
	{
		re:   regexp.MustCompile(`bind: address already in use`),
		text: "端口 80 已被占用 — 可能有其他 Web 服务器（Nginx/Apache）正在运行。请先停止占用端口的服务后重试",
	},
	{
		re:   regexp.MustCompile(`Verify error`),
		text: "证书申请失败 — 请确认域名已正确解析到此服务器的 IP 地址。DNS 记录生效可能需要几分钟",
	},
}

// TranslateError maps a raw system error to a plain-language Chinese message.
// If no pattern matches, the original error string is returned unchanged.
func TranslateError(err error) string {
	msg := err.Error()
	for _, p := range patterns {
		if p.re.MatchString(msg) {
			return p.text
		}
	}
	return msg
}
