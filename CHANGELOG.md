# Changelog

## [v0.0.2] - 2026-06-08

### Added
- 首次运行安装向导：自动检测新用户，引导输入域名和邮箱，安装前环境预检 + 确认步骤
- `--version` 查看版本信息（支持 ldflags 注入构建信息）
- 安装过程进度提示 `[1/5]...[5/5]`
- 常见错误信息中文翻译：软件包未找到、端口被占用、证书申请失败
- 安装完成标记文件（`.install-complete`），用于区分首次安装和部分安装失败

### Changed
- TUI 菜单标题简化：替换 VLESS 技术术语为通俗中文
- README 重写：面向非技术用户，添加准备清单、快速开始、常见问题

## [2026-06-08]

- Add `--email` flag for non-interactive cert setup
- Add gstack skill routing to CLAUDE.md

## [2026-04-21]

### Fixed
- Nginx empty-domain prompt now reuses promptValue (avoids post-install manual edit)
- Panic on crypto/rand failure in GenerateUUID (internal error resilience)
- Backup tarball permissions tightened to 0o600 (TLS key exposure)
- RunSilent returns stdout on non-zero exit (restore from backup works)
- Cert flow: let closed stdin accept a valid persisted value
- Xray config written 0o600 to keep UUID off world-readable bits
- WARP: fail loudly on RPM install when arch is not amd64 (avoids silent 404)
- Config save is now atomic (tmp file + rename, no truncated yaml on crash)
- CLI exits cleanly when stdin closes mid-prompt

### Changed
- Refactored config: dropped unused fields and --warp-license flag
- Tightened /etc/xrayctl perms to 0700/0600 (root-only)

## [2026-04-19]

### Changed
- CLI/menu: extracted renderServiceStatus; added StatusFailed constant
- All service operations routed through internal layer (systemd wrappers)
- Config cert_dir tag aligned with const; fixed route_domains yaml key

### Fixed
- WARP: RPM url + apt update + sources.list; added fake-runner test

## [2026-04-18]

### Changed
- Backup execs tar directly instead of via bash -c
- SetupXray uses ServiceXray constant; logs SaveConfig errors

## [2026-04-17]

### Added
- CommandRunner interface with context.Context support
- Xray config rendered from embedded text/template files
- Typed XrayConfig struct with JSON round-trip tests
- CLAUDE.md for AI assistant guidance

### Changed
- CLI/flags: reject ambiguous action combos; save config only when dirty
- Nginx configs: all rendered from text/templates
- WARP status reported via systemd, not warp-cli text

### Removed
- Unused internal.GetHostIP

## [2026-03-29 — 2026-03-31]

- Initial project scaffold and core feature development
- Install base, cert, nginx, WARP, and Xray setup pipelines
- Interactive menu and non-interactive CLI modes
