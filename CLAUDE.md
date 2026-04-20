# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`xrayctl` is a single-binary Go CLI (Go 1.21, sole runtime dep `gopkg.in/yaml.v3`) that installs and manages a Xray + Nginx + Cloudflare WARP stack on Linux. It produces a VLESS + XTLS-Vision server with AI-site traffic selectively routed through a WARP SOCKS outbound.

Target runtime: Linux amd64/arm64, **as root**. Most `service/*` code shells out to `systemctl`, `apt`/`yum`/`dnf`, `warp-cli`, `nginx`, `acme.sh`, and the Xray installer — so tests and local dev on non-Linux hosts can only exercise pure-Go logic (config, helpers, JSON/YAML generation). Note: Cloudflare publishes WARP RPMs for `x86_64` only, so on RHEL-family arm64 hosts `installWarpRPM` returns an explicit error rather than 404'ing inside `rpm`. Debian/Ubuntu apt covers both arches.

## Common commands

```bash
# Build local binary (output: ./xrayctl — gitignored)
go build -o xrayctl ./cmd

# Cross-compile (matches CI)
GOOS=linux GOARCH=amd64 go build -o xrayctl-linux-amd64 ./cmd
GOOS=linux GOARCH=arm64 go build -o xrayctl-linux-arm64 ./cmd

# Run full test suite with race detector (what CI runs)
go test ./... -v -race -coverprofile=coverage.out

# Single package / single test
go test ./config -run TestSaveAndLoadConfig -v
go test ./service -run TestXrayConfigGeneration -v

# Lint (CI pins golangci-lint v1.60.0; config in .golangci.yml)
golangci-lint run --timeout 5m ./...

# Run interactively (menu) vs. non-interactive (flags)
sudo ./xrayctl
sudo ./xrayctl --install --domain example.com
sudo ./xrayctl --status
```

`cli.ParseFlags` returns `false` when `os.Args` has no flags; `main` then falls through to `cli.ShowMenu`. This is why adding a new flag requires updating `cli/flags.go` only — the menu is a separate code path in `cli/menu.go`.

## Architecture

Four-package layout, strict one-way dependency graph `cmd → cli → {config, service, internal}` and `service → {config, internal}`. Do not introduce cycles.

- **`cmd/main.go`** — Entry point. Keep trivial.
- **`cli/`** — Two UIs over the same `service` functions:
  - `flags.go` non-interactive CLI (also overrides persisted config from flag values, then calls `config.SaveConfig` before dispatch).
  - `menu.go` interactive TUI; polls `NginxStatus`/`XrayStatus`/`WarpStatus` each loop for the header.
- **`config/`** — YAML persistence at `/etc/xrayctl/config.yaml` (`DefaultConfigPath`). `LoadConfig` reads the file, then `fillDefaults` uses reflection to overlay `DefaultConfig()` onto any zero-valued field. `ConfigPath` is a package-level `var` so tests can point it at a tmp file. When adding a `Config` field, also set a default in `DefaultConfig()` if it needs one — the `default:"..."` struct tag is documentation only, not read by code.
- **`internal/`** — Shared utilities. `internal` here is a normal package name under module `xrayctl`, **not** Go's import-privacy `internal/` convention. Import as `xrayctl/internal`.
  - `cmdexec.go` — `ExecCommand` (logs `[CMD]`/`[STDOUT]`/`[STDERR]`), `ExecCommandSilent`, `ExecCommandWithSudo` (auto-skips `sudo` when already root).
  - `svcmanager.go` — systemd wrappers.
  - `utils.go` — ANSI color `Print{Red,Green,Yellow}` (used everywhere for user output — do not switch to `log`/`fmt.Println` for user-facing messages).
  - `netutil.go` — public IP and WARP-SOCKS IP probes with timeouts.
  - `constants.go` — service names, protocol tags, acme.sh paths. Prefer adding magic strings here.
- **`service/`** — One file per subsystem, each exposing an idempotent `Setup*` and a `*Status` query:
  - `base.go` — dep install (apt/yum/dnf auto-detect) + enables BBR.
  - `cert.go` — installs `acme.sh` (4 fallback install methods) and issues a Let's Encrypt cert via `--standalone`, which **stops nginx** for port 80 and restarts it on exit. Writes to `cfg.CertDir/{xray.crt,xray.key}`.
  - `nginx.go` — rewrites `/etc/nginx/nginx.conf` (main) and `cfg.NginxConfig` (vless fallback on `127.0.0.1:<NginxPort>` with `proxy_protocol`). Runs `nginx -t` before restart.
  - `warp.go` — adds Cloudflare apt repo, registers, sets `warp-cli mode proxy` on `cfg.WARPPort`, verifies by fetching public IP through the SOCKS proxy.
  - `xray.go` — installs via upstream `install-release.sh`; `buildXrayConfigJSON` emits the Xray config as a string (inbound on `cfg.XrayPort` with VLESS+XTLS-Vision, fallback to the nginx port, outbounds `direct`+`warp`, routing rules generated from `cfg.RouteDomains`). UUID precedence: `cfg.UUID` → extracted from existing on-disk config → MD5-of-email derived → random.
  - `health.go`, `backup.go`, `uninstall.go` — self-evident; `Backup` tars the config + cert paths into `xrayctl-backup-<ts>.tar.gz` in CWD.

The install pipeline (menu option 1 and `--install`) is a fixed sequence and order matters: `InstallBase → SetupCert → SetupNginx → SetupWarp → SetupXray → CheckStatus`. Cert issuance must precede nginx setup because acme.sh standalone binds :80 itself.

## Conventions

- **Language of user-facing strings is Chinese.** Preserve it when editing existing prints; new prints should match the surrounding file.
- Import grouping is enforced by `gci`/`goimports` with `local-prefixes: xrayctl` — stdlib, third-party, then `xrayctl/...`.
- `.golangci.yml` is strict (funlen 100 lines / 50 stmts, gocyclo 15, many linters enabled). Before pushing, run `golangci-lint run` — CI fails the whole pipeline (lint → test → build) on any lint error.
- `service/xray.go` exports a typed `XrayConfig` struct and emits configs via `json.MarshalIndent`; `service/xray_test.go` round-trips JSON back into the same `XrayConfig` type. Add new fields to the shared struct, not via a parallel definition.
- `config.SaveConfig` writes via tmp + `os.Rename` so a crash mid-save can never leave a truncated yaml on disk. Anything new that persists state should follow the same pattern.

## CI / branching

- CI: `.github/workflows/ci.yml` — triggers on push/PR to `main`/`master` and on `v*` tags. `lint` → `test` → `build`; tag pushes additionally run `release` (builds linux/windows/darwin amd64+arm64 and uploads SHA256 checksums via `softprops/action-gh-release`).
- Default branch is `master` (not `main`). Release tags are of the form `v<semver>`.
