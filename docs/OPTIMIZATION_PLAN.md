# xrayctl Optimization Plan

- **Status:** refreshed against the latest codebase on 2026-04-29 (first-principles revisit)
- **Branch:** `master`
- **Baseline commit:** `3756759`

**Validation baseline:**
- `GOCACHE=/tmp/xrayctl-go-cache go test ./...` passes
- `GOCACHE=/tmp/xrayctl-go-cache GOLANGCI_LINT_CACHE=/tmp/xrayctl-golangci-cache golangci-lint run --timeout 5m ./...` passes

## Goal

Make the root-level installer fail safely and consistently. `xrayctl` runs as root and mutates a live network stack — the objective is not "clean architecture" in the abstract, but predictable, recoverable, and testable behavior for a dangerous operator tool.

Keep the small, direct Go CLI shape. This is not a framework rewrite. The target shape stays:

```text
cmd -> cli -> {config, service, internal}
service -> {config, internal}
```

## Current Baseline

The codebase is already in better shape than the earlier plan assumed:

- `internal/cmdexec.go` already defines `CommandRunner`, `DefaultRunner`, and package-level `Exec*` wrappers. Tests can replace `internal.DefaultRunner` today.
- `go test ./...` and `golangci-lint run --timeout 5m ./...` pass with cache paths redirected inside the sandbox.
- The install sequence is still duplicated in `cli/flags.go` and `cli/menu.go`.
- `service/cert.go` still calls `os.Exit(1)` through `promptValue` when stdin is closed and no valid value exists.
- `service/health.go:CheckStatus` and `service/base.go:CheckSystemEnvironment` still return nothing.
- `service/xray.go` still falls back to a deterministic MD5-derived UUID from email before using a random UUID.
- `config/manager.go` still uses reflection-based default filling.

## Optimization Principles

- Keep service code procedural and explicit.
- Let the CLI layer own process exit, prompts, and output formatting.
- Let service functions return data and errors.
- Use the existing `internal.DefaultRunner` seam before introducing broader dependency injection.
- Avoid a `Runtime` or dependency-bag abstraction unless multiple concrete dependencies need to move together.
- Add tests around behavior that is currently risky, not around every shell command for its own sake.
- **Status errors should be context-sensitive.** A down service is useful status output; it only becomes a hard error in validation contexts like post-install or `--check`, not on every `--status` call.

## Phase 1: Core Fixes

These are the changes worth doing first. They are small enough to ship together and directly address current code risks.

### 1. Remove Process Exit From `service/`

**Current:** `service/cert.go:promptValue` calls `os.Exit(1)` when stdin is closed and no usable persisted value exists. This makes `service` code impossible to call safely from tests or automation.

**Target:**

Change `promptValue` to return a `(value, error)` pair instead of terminating the process on closed stdin. Introduce a sentinel error for the input-closed case so callers can distinguish EOF from validation failure. `SetupCert` and `SetupNginxVlessConf` propagate wrapped errors upward instead of exiting.

**Scope:**
- `service/cert.go`
- `service/nginx.go`
- callers in `cli/flags.go` and `cli/menu.go` already handle returned errors
- update `service/cert_test.go` for the new return shape

**Done when:**
- No `os.Exit` or `log.Fatal` calls remain in `service/` package code
- non-interactive install with valid persisted domain/email still accepts those values on EOF
- non-interactive install without required values returns a clear error

### 2. Use Random UUID For New Installs

**Current:** `SetupXray` chooses UUID in this order:

```text
cfg.UUID -> existing Xray config UUID -> MD5(email) -> random only when email is empty
```

This makes a fresh UUID predictable from email.

**Target:**

```text
cfg.UUID -> existing Xray config UUID -> random UUID
```

Do not add a `--uuid-from-email` flag unless compatibility demand appears. Keeping a deterministic fallback around is extra surface area for a weak default that new installs should not use.

**Scope:**
- `service/xray.go`
- `service/xray_test.go`
- README wording if it implies email-derived UUID behavior

**Done when:**
- MD5 import and `generateUUIDFromEmail` are gone
- tests assert random fallback shape without requiring deterministic equality
- existing installs still preserve UUID from config or on-disk Xray config

### 3. Extract One Install Orchestrator

**Current:** `--install` and menu option `1` both repeat:

```text
InstallBase -> SetupCert -> SetupNginx -> SetupWarp -> SetupXray -> CheckStatus
```

**Target:**

Introduce `func InstallAll(cfg *config.Config) error` in `service/install.go`. It runs the fixed sequence once and wraps each step with a contextual error message. Keep it simple for Phase 1. Do not add `context.Context` or an explicit runner parameter here yet. The current command path already goes through `internal.DefaultRunner`, and adding context only at the orchestrator layer would not cancel existing package-level shell-outs.

**Scope:**
- `service/install.go`
- `cli/flags.go`
- `cli/menu.go`
- focused test for step order and first-error behavior if it can be done without broad monkey-patching; otherwise defer orchestration unit tests until service functions accept narrower seams

**Done when:**
- the install order exists in one place
- flag and menu install paths call the same function
- user-facing Chinese error messages remain in the CLI layer

### 4. Return Structured Status

**Current:** `CheckStatus(cfg)` prints directly and returns nothing. It mixes status collection, WARP link probing, connection parameter formatting, and terminal output.

**Target:**

Separate status data collection from rendering. Introduce a report type to hold the collected state (service status strings, WARP IP, connection parameters, share link). Expose a collector function that returns the report and a printer function that renders it. Callers choose context:

- For `--status`: collect and render; a down service is useful output, not a hard error.
- For post-install or `--check`: collect, render, then evaluate the report for hard failures.

Keep the old combined function temporarily as a compatibility wrapper that delegates to collector + printer and returns an error for callers that still need it.

**Scope:**
- `service/health.go`
- call sites in `cli/flags.go`, `cli/menu.go`, and `service/install.go`
- tests for share-link generation and failed WARP probe behavior

**Done when:**
- callers can tell whether health verification failed without parsing stdout
- status rendering remains Chinese and visually equivalent for users

### 5. Make Environment Check Return Errors

**Current:** `CheckSystemEnvironment(cfg)` prompts, may call `InstallBase`, prints status, and returns nothing. If dependency installation fails, it prints "依赖安装完成，请重试", which is misleading for an error path.

**Target:**

Separate environment inspection from error reporting. Introduce an environment report type to hold the inspection results (missing commands, config/cert/Xray/nginx file presence). Add a function that checks the environment and returns an error when dependency installation fails. The existing interactive auto-install prompt can stay for now, but a failed `InstallBase` must return an observable error and print an accurate message.

**Scope:**
- `service/base.go`
- menu option `6`
- first-run menu startup

**Done when:**
- dependency install failure is observable by the caller
- output no longer claims dependency installation completed after an error

## Phase 1 Execution Order

```text
1. Remove service-layer os.Exit from promptValue.         (security: process control)
2. Replace email-derived UUID fallback with random UUID.   (security: credential hygiene)
3. Extract InstallAll and point menu/flag install paths at it.  (consistency)
4. Split status collection from status printing.           (observability)
5. Make environment check failures observable.              (safety)
6. Run go test and golangci-lint after each step.
```

## Phase 2: Deferred Work

These are useful but should not block Phase 1.

| Item | Why deferred |
|---|---|
| Explicit `CommandRunner` parameters on service functions | Existing `internal.DefaultRunner` already gives tests a seam. Add explicit parameters only when a specific function needs cancellation or isolated concurrent tests. |
| `context.Context` propagation | Useful for long-running shell-outs, but only after service functions stop using package-level `Exec*` helpers directly. |
| Production `FakeRunner` | Current test fakes are small. Add shared test infrastructure only after duplication becomes meaningful. |
| `ApplyDefaults(*Config)` | Reflection defaults are not a current runtime bug. Replace when config evolution needs explicit migration semantics. |
| `--check` preflight mode | Needs stable structured environment and status reports first. |
| `--dry-run` mode | Would be misleading until file writes, package installs, and service restarts also have seams. |
| Makefile | Low effort, but documentation already lists commands. Add once the validation commands settle. |
| CONTRIBUTING.md | Useful after Phase 1 establishes stable extension patterns. |
| BBR sysctl idempotency | Current code can append duplicate sysctl lines. Important, but lower risk than service exits and UUID fallback. |
| Backup restore hardening | Inspect tar paths before extracting to `/`; reject `..`, absolute paths, and symlink escapes. |
| `bash -c` audit | Keep shell only where pipes or compound commands are required; replace simple calls with argv or Go file APIs over time. |
| Uninstall error model | `Uninstall()` still returns nothing. Convert after the higher-traffic install/status paths are cleaned up. |

## Non-Goals

- No large runtime framework.
- No generic dependency bag.
- No rollback transaction model for apt, acme.sh, nginx, WARP, and Xray.
- No dry-run mode that claims safety while real code still mutates the filesystem or system services.
- No broad rewrite of Chinese user-facing output.

## Review Checklist

Before considering this plan complete:

- `go test ./...` passes.
- `golangci-lint run --timeout 5m ./...` passes.
- `rg "os\\.Exit|log\\.Fatal" service` returns no matches.
- new installs generate random UUIDs unless an existing UUID is available.
- `--install` and menu install share one orchestration path.
- service-layer checks return errors or structured reports.
