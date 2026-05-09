# xrayctl

**一台 VPS、一个 Go 二进制，自动把 ChatGPT / Claude / Grok 这类 AI 站点的流量切到 Cloudflare WARP 出口，其余流量原路直连。**

## ✨ 功能特性
- 🚀 **单二进制无依赖**：仅8MB大小，无需预装任何依赖，扔到服务器直接运行
- 🔒 **安全协议**：VLESS + XTLS-Vision 加密协议，性能强、安全性高
- 🌍 **智能分流**：ChatGPT/OpenAI/Claude/X/Grok等AI站点自动走WARP出口，其余流量直连
- 📜 **配置持久化**：所有配置保存在YAML文件，修改无需改代码
- 🎫 **证书自动管理**：acme.sh自动申请/续签SSL证书，无需人工干预
- 💾 **备份恢复**：一键备份/恢复所有配置、证书、密钥，重装系统快速迁移
- 🖥️ **双模式支持**：交互式菜单 + 非交互命令行，适合手动操作和自动化部署
- 🌐 **多系统兼容**：支持Debian 11+/Ubuntu 20.04+/CentOS Stream 8+

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
--uuid string           指定 UUID（可选；默认优先从已有配置恢复，否则随机生成）

# 运维操作
--check                 只读预检环境与服务状态，不安装、不写配置、不重启服务
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

### Dry-run 状态

`--dry-run` 暂不开放。可信的 dry-run 需要先为三类副作用补齐可测试 seam：文件写入（例如 `/etc/xrayctl/`、Nginx/Xray 配置和证书目录）、包安装（apt/yum/dnf 与外部安装脚本）、service 操作（restart/enable/stop）。在这些 seam 存在之前，xrayctl 只提供 `--check` 作为只读预检，避免把仍可能改系统状态的路径包装成“安全演练”。

## 🔌 客户端连接

安装完成后 `--status` 会打印 VLESS 分享链接和分项参数：

| 字段 | 值 |
|---|---|
| 协议 | `VLESS` |
| 地址 | `<你的域名>` |
| 端口 | `443` |
| UUID | 你指定的值 / 既有配置中的值 / 随机生成 |
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
