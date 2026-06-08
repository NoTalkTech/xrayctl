# xrayctl

**一条命令，把普通 VPS 变成专属 VPN。ChatGPT、Claude、Grok 等 AI 站点自动走 Cloudflare WARP 出口，无需复杂配置。**

## 这是什么？

xrayctl 是一个单文件 Go 程序（约 8MB）。上传到你的 VPS，运行一条命令，自动完成：
- 安装和配置 VLESS + XTLS-Vision 加密协议（目前最快的 VLESS 方案）
- 部署 Nginx 作为回落站点（让 VPN 流量看起来像普通网页访问）
- 接入 Cloudflare WARP 代理出口（访问 ChatGPT/Claude 等 AI 站点时自动切换线路）
- 申请和管理免费 SSL 证书（自动续签，永久有效）

**适合谁用：** 有一台 Linux VPS，想自建 VPN 来访问 AI 工具或其他受限站点的人。你不需要懂 Nginx 配置、证书管理、WARP 代理 —— xrayctl 帮你搞定全部。

## 你需要准备

在开始之前，请确认你已有：
- ✅ 一台 Linux VPS（Debian 11+/Ubuntu 20.04+/CentOS Stream 8+，1核512MB 即可）
- ✅ 一个域名，且已添加 A 记录指向 VPS 的 IP 地址
- ✅ 以 root 用户登录 VPS（或可以使用 sudo）
- ✅ 10 分钟时间

## 快速开始

### 1. 下载

```bash
wget https://github.com/notalktech/xrayctl/releases/latest/download/xrayctl-linux-amd64 -O xrayctl
chmod +x xrayctl
```

### 2. 运行安装向导

```bash
sudo ./xrayctl
```

首次运行会自动进入安装向导，按提示输入域名和邮箱即可。向导会：
1. 检查你的系统环境
2. 帮你安装所有依赖
3. 自动申请 SSL 证书
4. 配置 Nginx 和 Xray
5. 启动所有服务

### 3. 完成！

安装完成后，屏幕会显示连接参数和分享链接。将分享链接导入你的客户端（如 v2rayN、Shadowrocket、v2rayNG），即可开始使用。

### 非交互式安装（自动化/CI）

```bash
sudo ./xrayctl --install --domain your-domain.com --email you@example.com
```

## 常见问题

### 安装失败：证书申请报错
**原因：** 域名 DNS 记录还未生效，或域名未指向当前 VPS IP。
**解决：** 在域名 DNS 管理后台添加 A 记录，等待 2-5 分钟后重试。可以用 `ping your-domain.com` 验证是否已解析到正确 IP。

### 安装失败：端口被占用
**原因：** 80 或 443 端口已被其他程序（如已安装的 Nginx/Apache）占用。
**解决：** 停止占用端口的服务：`systemctl stop nginx apache2`，然后重试安装。

### 安装失败：软件包未找到
**原因：** 操作系统版本不受支持，或软件源未更新。
**解决：** 确认系统为 Debian 11+ / Ubuntu 20.04+ / CentOS Stream 8+。运行 `apt update`（Debian/Ubuntu）或 `yum update`（CentOS）后重试。

### 如何修改分流域名？
编辑 `/etc/xrayctl/config.yaml` 中的 `route_domains` 列表，添加你需要走 WARP 出口的域名，然后重启 Xray：
```bash
sudo systemctl restart xray
```

### 如何更新 xrayctl？
下载最新版本替换旧二进制文件即可：
```bash
wget https://github.com/notalktech/xrayctl/releases/latest/download/xrayctl-linux-amd64 -O xrayctl
chmod +x xrayctl
```

### 如何卸载？
```bash
sudo ./xrayctl --uninstall
```

## 命令行参数

```text
--install               完整安装所有组件
--domain string         指定域名
--email string          指定证书申请邮箱
--uuid string           指定 UUID（可选，默认自动生成）

--check                 只读预检，不安装、不修改系统
--status                查看运行状态与连接参数
--restart-warp          重启 WARP 代理
--update-xray           更新 Xray 核心
--renew-cert            重新申请/续签证书
--backup                备份所有配置与证书
--restore string        从指定备份文件恢复
--uninstall             彻底卸载所有组件
--version               显示版本信息

一次只能指定一个操作 flag。
```

## 分流规则

默认以下 AI 站点自动走 WARP 出口：
ChatGPT/OpenAI, X/Grok, Anthropic/Claude, Bing

编辑 `/etc/xrayctl/config.yaml` 的 `route_domains` 列表可自定义。

## 📁 项目结构

```
xrayctl/
├── cmd/main.go         程序主入口
├── cli/
│   ├── flags.go        非交互 CLI（flag 解析 + 配置覆盖 + 动作分发）
│   ├── menu.go         交互式 TUI 菜单
│   └── wizard.go       首次运行安装向导
├── config/             YAML 配置 + 原子持久化
├── service/            子系统编排
│   ├── base.go           基础环境 + BBR
│   ├── cert.go           SSL 证书（acme.sh）
│   ├── nginx.go          Nginx 配置
│   ├── warp.go           Cloudflare WARP
│   ├── xray.go           Xray 核心
│   ├── install.go        安装流水线
│   ├── health.go         状态检查
│   ├── backup.go         备份恢复
│   └── uninstall.go      卸载
└── internal/           共享工具
    ├── cmdexec.go         命令执行
    ├── svcmanager.go      systemd 管理
    ├── utils.go           颜色输出 + UUID 生成
    ├── netutil.go         网络工具
    ├── constants.go       常量定义
    └── errtrans.go        错误信息翻译
```

## 致谢

- [Xray-core](https://github.com/XTLS/Xray-core)
- [acme.sh](https://github.com/acmesh-official/acme.sh)
- [Cloudflare WARP](https://developers.cloudflare.com/warp-client/)
