# xrayctl

**一台 VPS、一个 Go 二进制，自动把 ChatGPT / Claude / Grok 这类 AI 站点的流量切到 Cloudflare WARP 出口，其余流量原路直连。**

VPS 部署的代理工具里，bash 脚本占了绝大多数。xrayctl 走另一条路：所有逻辑都在一个**约 8 MB 的 Go 单二进制**里，源代码可读、可 review、可单元测试，没有几百行的 `curl | bash` 黑盒。

## ✨ 它做什么

- 🌍 **AI 站点精准分流**：ChatGPT / OpenAI / Claude / X / Grok / Bing 等域名走 Cloudflare WARP SOCKS 出口，其它流量直连。规则是 YAML 列表，随时改。
- 📦 **单 Go 二进制**：扔到 VPS 上 `./xrayctl` 就能跑，不依赖外部 bash 脚本、不依赖 Python 运行时、不依赖第三方包管理。
- 🤖 **真正的非交互部署**：`./xrayctl --install --domain x --email y < /dev/null` 在脚本/CI 里能跑通，不会因为缺少 stdin 卡住。
- 🎫 **证书 + 服务全自动**：acme.sh 申请 / 续签 Let's Encrypt 证书，nginx 回落，warp-cli 注册并设代理，xray 安装并起来。
- 💾 **备份恢复**：tar 打包配置 + 证书（自动 0o600），换机器解压后服务可继续。
- 🖥️ **双 UI 同源**：交互式菜单和 `--flags` 命令行共用同一套 service 层，不会出现"菜单里能干、CLI 里不能干"的割裂。
- 📜 **配置持久化**：`/etc/xrayctl/config.yaml`，原子写入，崩溃不会留半成品。

底层栈是 **VLESS + XTLS-Vision + 真 TLS + Nginx fallback** —— 这是 2022 年起就稳定的方案，足够日常使用，但**不是抗主动探测最强的选择**。如果你的威胁模型是高强度审查，建议看 Reality / Hysteria2 类的方案（见下方"它不做什么"）。

## 🚫 它不做什么

为了把"在一台 VPS 上跑稳"这件事做好，xrayctl 主动放弃了几类功能：

- ❌ **不是多用户面板**。一台机器一个 VLESS client。需要发卡、按用户限流、Web 管理界面，请用 [3x-ui](https://github.com/MHSanaei/3x-ui) / [Marzban](https://github.com/Gozargah/Marzban) / [Hiddify](https://github.com/hiddify/hiddify-config)。
- ❌ **不是多协议管理器**。当前只支持 VLESS + XTLS-Vision 一种 inbound。需要 Reality / Hysteria2 / TUIC / NaiveProxy 共存的，请用 [v2ray-agent](https://github.com/mack-a/v2ray-agent) / [vless-all-in-one](https://github.com/Chil30/vless-all-in-one)。
- ❌ **不替换 Xray / Nginx / acme.sh / warp-cli**。它是这几个工具的**编排器**，不是替代品。

## 🚀 快速开始

### 1. 下载二进制
```bash
# Release 页选对应架构（linux-amd64 / linux-arm64），重命名后赋权
wget https://github.com/notalktech/xrayctl/releases/latest/download/xrayctl-linux-amd64 -O xrayctl
chmod +x xrayctl
```

### 2. 运行（需 root）
**交互式（推荐第一次用）**
```bash
sudo ./xrayctl
```
菜单选 `1. 完整安装`，跟着提示输入域名 + 邮箱。

**非交互式（CI / 自动化部署）**
```bash
sudo ./xrayctl --install --domain your-domain.com --email you@example.com < /dev/null
```
配置写入 `/etc/xrayctl/config.yaml`，后续 `--install` / `--renew-cert` 之类的动作可以省略 `--domain` / `--email`，xrayctl 会复用持久化的值。

## 📖 命令行参数

```text
# 一键安装
--install               完整安装所有组件
--domain string         指定域名
--email string          指定证书申请邮箱（acme.sh 注册用）
--uuid string           指定 UUID（可选，默认自动生成或从已有配置恢复）

# 运维操作
--status                查看运行状态与连接参数
--restart-warp          重启 WARP 代理并验证连通性
--update-xray           更新 Xray 核心（保留配置与 UUID）
--renew-cert            重新申请 / 续签 SSL 证书

# 备份恢复
--backup                备份所有配置与证书（输出 xrayctl-backup-<ts>.tar.gz, 0o600）
--restore string        从指定备份文件恢复

# 其他
--uninstall             彻底卸载所有组件
```

## 🔌 客户端连接

安装完成后 `--status` 会打印 VLESS 分享链接和分项参数：

| 字段 | 值 |
|---|---|
| 协议 | `VLESS` |
| 地址 | `<你的域名>` |
| 端口 | `443` |
| UUID | 自动生成 / 你指定的值 |
| 流控 | `xtls-rprx-vision` |
| 传输层安全 | `TLS` |
| 传输协议 | `TCP` |

## 🔧 分流规则

默认走 WARP 出口的 AI 站点（`config.yaml.route_domains`）：

- ChatGPT / OpenAI: `chatgpt.com`, `openai.com`, `oaistatic.com`, `oaiusercontent.com`
- X / Grok: `x.ai`, `grok.com`, `x.com`
- Anthropic / Claude: `anthropic.com`, `claude.ai`
- Bing: `bing.com`, `edgeservices.bing.com`

加新规则直接编辑 `/etc/xrayctl/config.yaml` 的 `route_domains` 列表，重启 Xray 即可生效。

## 📁 项目结构

```
xrayctl/
├── cmd/main.go         程序主入口（保持轻量）
├── cli/
│   ├── flags.go        非交互 CLI（flag 解析 + 配置覆盖 + 动作分发）
│   └── menu.go         交互式 TUI
├── config/             YAML 配置 + 原子持久化
├── service/            子系统编排（每个文件对应一个组件）
│   ├── base.go           基础环境 + BBR
│   ├── cert.go           acme.sh + Let's Encrypt
│   ├── nginx.go           Nginx fallback（embed.FS 模板）
│   ├── warp.go           Cloudflare warp-cli
│   ├── xray.go           Xray + VLESS 配置生成
│   ├── health.go         健康检查
│   └── backup.go         备份 / 恢复
└── internal/           shell-out / 文件 / 颜色 / 网络等通用工具
```

依赖图严格单向：`cmd → cli → {config, service, internal}`，`service → {config, internal}`。详见 [`CLAUDE.md`](CLAUDE.md)。

## ⚠️ 运行前提

1. **域名已解析到服务器公网 IP** —— Let's Encrypt `--standalone` 验证用
2. **80 / 443 端口可入站** —— acme.sh 占 80 验证证书，xray 占 443 提供 VLESS
3. **以 root 运行** —— 写 `/etc/nginx/`、`/usr/local/etc/xray/`、调 systemctl
4. **支持的系统**：
   - ✅ Debian 11+ / Ubuntu 20.04+ — amd64 + arm64
   - ✅ CentOS Stream 8+ / RHEL 衍生 — **仅 amd64**（Cloudflare 不为 RHEL 家族发布 arm64 的 WARP RPM，xrayctl 在 arm64 RHEL 上会显式报错而不是静默失败）

## 📄 License

MIT
