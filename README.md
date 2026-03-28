# Xray-WARP 精准分流管理系统

基于Go语言开发的Xray + Nginx + Cloudflare WARP 三位一体管理工具，一键部署VLESS+XTLS-Vision代理，AI站点自动分流走WARP出口，其余流量直连。

## ✨ 功能特性
- 🚀 **单二进制无依赖**：仅8MB大小，无需预装任何依赖，扔到服务器直接运行
- 🔒 **安全协议**：VLESS + XTLS-Vision 加密协议，性能强、安全性高
- 🌍 **智能分流**：ChatGPT/OpenAI/Claude/X/Grok等AI站点自动走WARP出口，其余流量直连
- 📜 **配置持久化**：所有配置保存在YAML文件，修改无需改代码
- 🎫 **证书自动管理**：acme.sh自动申请/续签SSL证书，无需人工干预
- 💾 **备份恢复**：一键备份/恢复所有配置、证书、密钥，重装系统快速迁移
- 🖥️ **双模式支持**：交互式菜单 + 非交互命令行，适合手动操作和自动化部署
- 🐳 **Docker测试环境**：一键启动测试容器，不污染本机环境
- 🌐 **多系统兼容**：支持Debian 11+/Ubuntu 20.04+/CentOS Stream 8+

## 🚀 快速开始
### 1. 下载二进制
```bash
# 下载最新版本（替换为实际下载地址）
wget https://github.com/notalktech/xrayctl/releases/latest/download/xrayctl
chmod +x xrayctl
```

### 2. 运行（需root权限）
#### 交互式模式（推荐新手）
```bash
./xrayctl
```
按照菜单提示选择`1. 完整安装`，输入域名即可一键部署全链路。

#### 非交互式模式（适合自动化部署）
```bash
./xrayctl --install --domain your-domain.com
```

## 📖 使用说明
### 命令行参数
```bash
# 一键安装
--install               完整安装所有组件
--domain string         指定域名
--uuid string           指定UUID（可选，自动生成）

# 运维操作
--status                查看运行状态与连接参数
--restart-warp          重启WARP代理并验证连通性
--update-xray           更新Xray核心（保留配置与UUID）
--renew-cert            重新申请/续签SSL证书

# 备份恢复
--backup                备份所有配置与证书
--restore string        从指定备份文件恢复

# 其他
--uninstall             彻底卸载所有组件
```

### 连接配置
安装完成后会自动生成VLESS分享链接，也可以运行`./xrayctl --status`查看连接参数：
- 协议：`VLESS`
- 地址：`你的域名`
- 端口：`443`
- UUID：`自动生成的UUID`
- 流控：`xtls-rprx-vision`
- 传输层安全：`TLS`
- 传输协议：`TCP`

## 📁 项目结构
```
xrayctl/
├── cmd/main.go          # 程序主入口
├── config/              # 配置管理模块（YAML读写、持久化）
├── service/             # 业务功能模块
│   ├── base.go          # 基础环境与BBR加速
│   ├── cert.go          # SSL证书管理
│   ├── nginx.go         # Nginx回落配置
│   ├── warp.go          # WARP代理管理
│   ├── xray.go          # Xray核心管理
│   ├── health.go        # 健康检查与状态显示
│   └── backup.go        # 备份恢复功能
├── internal/            # 通用工具库
├── cli/                 # 交互层
│   ├── menu.go          # 交互式菜单
│   └── flags.go         # 命令行参数解析
└── README.md
```

## 🔧 分流规则
默认走WARP出口的AI站点：
- ChatGPT/OpenAI系列：`chatgpt.com`, `openai.com`, `oaistatic.com`, `oaiusercontent.com`
- X/Grok系列：`x.ai`, `grok.com`, `x.com`
- Anthropic/Claude系列：`anthropic.com`, `claude.ai`
- Bing：`bing.com`, `edgeservices.bing.com`

如需添加自定义分流规则，修改`/etc/xrayctl/config.yaml`中的`route_domains`配置即可。

## ⚠️ 注意事项
1. 运行前请确保域名已经解析到服务器公网IP
2. 服务器需要开放80和443端口
3. 仅支持AMD64架构Linux系统
4. 必须以root权限运行

## 📄 License
MIT License
