# xrayctl Optimization Plan

**Status:** Reviewed via `/plan-devex-review` — 2026-04-28  
**Branch:** claude/non-interactive-install  
**Target persona:** Go developer / contributor who clones, builds, extends, and tests the codebase

## Goal

Keep `xrayctl` small and pragmatic while improving maintainability, testability, and operational safety for contributors. The project should remain a single-binary Go CLI for VPS setup, not become a framework-heavy system.

## Design Constraint

Do not over-engineer. Keep the codebase as a small procedural Go CLI with clear package boundaries:

```
cmd -> cli -> {config, service, internal}
service -> {config, internal}
```

## Scope: Two-Phase Delivery

Codex outside voice found the original 8-priority plan too broad for one refactor. Split into **Phase 1** (core architectural fixes, ship now) and **Phase 2** (nice-to-have improvements, defer).

---

## Phase 1 — Core (ship first)

### Priority 1: Remove `os.Exit` From Service Layer

**Current:** `service/cert.go:62` calls `os.Exit(1)` when stdin is closed. Kills the process — no recovery, no retry, no useful error for CI/automation.

**Target:** Return `error` from every code path. The CLI layer (menu or flags) decides whether to exit.

**Changes:**
- `service/cert.go`: replace `exitNoInput()` → return `fmt.Errorf("reading input failed: stdin closed")`
- Audit all service files for `os.Exit`, `log.Fatal`, `panic` — zero tolerance in `service/` package

### Priority 2: All Service Functions Return `error`

**Current:** Two void functions hide failures:
- `CheckSystemEnvironment(cfg)` — prints status but returns nothing
- `CheckStatus(cfg)` — prints health but returns nothing

**Target:** Every `service/*` function returns `error`. The caller (menu or flags) owns all output.

```go
// Before
func CheckStatus(cfg *config.Config)

// After — returns structured status
type StatusReport struct {
    XrayRunning bool
    NginxRunning bool
    WarpRunning bool
    CertExists bool
    XrayPort int
    NginxPort int
    WarpPort int
    Domain string
}
func CheckStatus(cfg *config.Config) (*StatusReport, error)
```

**Why:** Programmatic callers (health monitoring, CI validation, watch mode) need structured data, not stringly-typed stdout to parse.

### Priority 3: Unify Install Orchestration

**Current:** The install pipeline is duplicated between `cli/flags.go` and `cli/menu.go`:

```
InstallBase → SetupCert → SetupNginx → SetupWarp → SetupXray → CheckStatus
```

**Target:** Single orchestration entry point:

```go
type RunContext struct {
    Cfg    *config.Config
    Runner internal.CommandRunner
    Ctx    context.Context
}

// InstallAll runs the full install pipeline, returning on first error.
// Steps are executed in order; each step is idempotent if the service
// function is well-behaved.
func InstallAll(ctx context.Context, cfg *config.Config, runner internal.CommandRunner) error
```

Both `cli/flags.go` and `cli/menu.go` call the same function.

**Location:** `service/install.go` (new file, orchestration only — no prompting, no printing policy).

### Priority 4: Inject CommandRunner Into Service Functions

**Current:** The `CommandRunner` interface exists (`internal/cmdexec.go:16`) but 30+ shell-out calls in `service/` use package-level globals (`internal.ExecCommand`, `internal.ExecCommandWithSudo`, `internal.ExecCommandSilent`).

**Target:** Start narrow — inject `CommandRunner` only into the functions that need it most:

```go
// Phase 1 scope: cert.go and warp.go (most call sites)
func SetupCert(ctx context.Context, cfg *config.Config, runner internal.CommandRunner) error
func SetupWarp(ctx context.Context, cfg *config.Config, runner internal.CommandRunner) error

// Other services follow once the pattern is proven.
// No Runtime struct abstraction yet — just inject what's needed.
```

**Why:** Codex found that `Runtime{Runner}` risks being "both too small and too abstract" — service code also touches filesystem, env, network, systemd paths. Start with command execution only; add narrow file/network seams only where tests need them.

### Priority 5: Add `FakeRunner` For Tests

**Current:** Service tests can't verify shell commands — they either skip on non-Linux or run `go test` without race.

**Target:**

```go
// internal/fakerunner.go
type FakeRunner struct {
    // RecordedCommands stores every command invocation for assertion.
    RecordedCommands []CommandCall
    // Stubs maps "name arg1 arg2" → canned output + error.
    Stubs map[string]FakeResult
}

type CommandCall struct {
    Name string
    Args []string
}

type FakeResult struct {
    Stdout string
    Err    error
}
```

Test pattern:

```go
func TestSetupCertCallsAcme(t *testing.T) {
    runner := internal.NewFakeRunner()
    runner.Stub("systemctl stop nginx", internal.FakeResult{Stdout: "", Err: nil})
    runner.Stub("acme.sh --issue ...", internal.FakeResult{Stdout: "cert issued", Err: nil})

    err := service.SetupCert(context.Background(), testCfg, runner)

    assert.NoError(t, err)
    assert.Len(t, runner.RecordedCommands, 3)
    assert.Equal(t, "acme.sh", runner.RecordedCommands[1].Name)
}
```

### Priority 6: Use Random UUID By Default

**Current:** UUID may be derived deterministically from email (MD5). If someone knows the email, they can predict the client's UUID.

**Target:**
- Reuse existing UUID from on-disk config if present
- Reuse configured UUID if set in YAML
- Otherwise generate random UUID via `crypto/rand`
- Only derive from email if explicitly requested via `--uuid-from-email`

**Why:** Better credential hygiene. Avoids predictable identity material. The "reuse existing" path preserves compatibility for installed clients.

---

## Phase 1 Execution Order

```
1. Add FakeRunner to internal/ package           (test infrastructure)
2. Remove os.Exit from service/cert.go           (safety, no design)
3. Convert void functions to return error         (API consistency)
4. Extract InstallAll — new service/install.go    (orchestration)
5. Inject Runner into cert.go + warp.go           (inject, not globals)
6. Write tests using FakeRunner for cert + warp   (verification)
7. Change UUID fallback to random                 (security)
```

---

## Phase 2 — Deferred

These are explicitly deferred. Do not work on them until Phase 1 is shipped and validated.

| # | Item | Why Deferred |
|---|------|-------------|
| 8 | `--check` preflight mode | Separate from `--dry-run`. Needs its own result model. Scope risk. |
| 9 | `--dry-run` mode | Much harder than FakeRunner — service functions write files, edit sysctl, download packages. Would lie without injected filesystem mutators. |
| 10 | `ApplyDefaults(*Config)` | Replace reflection-based `fillDefaults` with explicit default logic. Codex: "adding config versioning immediately may be unnecessary unless there is an actual incompatible schema migration." Start with `ApplyDefaults`. |
| 11 | Resumable install with state file | Uninstall + restart is already viable recovery. True rollback for apt/cert/nginx/warp/xray is strategically overambitious without a transaction model. |
| 12 | CONTRIBUTING.md | Phase 1 establishes the patterns. Document them when they're stable. |
| 13 | Makefile | `make build`/`make test`/`make lint` shorthand. Low effort but blocked on establishing the test command pattern first. |
| 14 | BBR sysctl idempotency | Low risk, easy fix. Not a priority until someone reports a duplicate-bbr-line bug. |
| 15 | Tar restore hardening | Inspect contents before extracting to `/`. Reject `..`, absolute paths, symlink escapes. |
| 16 | bash -c audit | Which calls use shell features vs. pure argv. Replace eligible ones with Go file APIs. |

---

## Design Decisions (from /plan-devex-review)

### Dependency Injection
- **Chosen:** Pass `internal.CommandRunner` as a parameter to service functions. Start narrow (cert.go, warp.go), add seams where tests need them.
- **Rejected:** `Runtime` struct abstraction. Risk of "generic dependency bag" — premature abstraction is worse than none.
- **Rejected:** Continue using globals. Untestable shell-outs are the main pain point being fixed.

### Error Handling
- **All service functions return `error`.** No exceptions for "informational" functions like `CheckStatus`.
- **No `os.Exit` from `service/`.** The CLI layer owns process exit.
- **Prefer error wrapping.** `fmt.Errorf("setup cert: %w", err)` provides context up the call stack.

### CheckStatus
- **Returns `*StatusReport` struct** with systemd states, port availability, cert presence, xray config validity.
- **Not void.** Not just `error`. Callers need structured data for monitoring and automation.

### InstallAll Contract

```go
// InstallAll runs the full install pipeline.
//
// Steps are executed in sequence:
//   1. InstallBase — detect distro, install deps, enable BBR
//   2. SetupCert — issue Let's Encrypt cert via acme.sh
//   3. SetupNginx — write nginx configs, test, restart
//   4. SetupWarp — install warp-cli, configure SOCKS proxy
//   5. SetupXray — install xray, write VLESS config, restart
//   6. CheckStatus — verify all services
//
// On first error the pipeline stops. There is no step-level rollback
// (see Phase 2 item 11). Each step is designed to be idempotent.
//
// InstallAll does not prompt for input and does not print to stdout
// beyond what the runner logs. The caller owns all user interaction
// and all output formatting.
func InstallAll(ctx context.Context, cfg *config.Config, runner internal.CommandRunner) error
```

### Config Migration
- **Not adding a config version field yet.** Phase 2 may add `ApplyDefaults(*Config)`.
- The reflection-based `fillDefaults` stays for now. Replace only when there's an actual incompatible schema change.

---

## Developer Persona

```
TARGET DEVELOPER PERSONA
========================
Who:       Go developer / DevOps engineer contributing to xrayctl
Context:   Clones the repo, reads CLAUDE.md for conventions, runs
           `go build`, `go test ./...`, `golangci-lint run`.
           May need to debug shell-outs to systemctl, warp-cli, acme.sh.
Tolerance: Will tolerate moderate setup if code is well-structured.
           Will abandon if the build-lint-test cycle is broken or confusing.
Expects:   make test or equivalent one-command validation. Clear error
           returns from service functions. No reflection magic. Testable
           interfaces for command execution. CI that catches regressions.
```

## DX Review Scorecard

```
+====================================================================+
| Dimension            | Score  | Why                                |
|----------------------|--------|-------------------------------------|
| Getting Started      | 7/10   | Strong CLAUDE.md, needs Makefile    |
| API/CLI/SDK Design   | 5/10   | CommandRunner exists but unused      |
| Error Messages       | 4/10   | os.Exit + void functions             |
| Documentation        | 6/10   | Missing CONTRIBUTING.md              |
| Upgrade Path         | 3/10   | No config versioning                 |
| Dev Environment      | 7/10   | CI + lint strong, needs Makefile     |
| Community            | 5/10   | Issues templates exist               |
| DX Measurement       | 2/10   | No before/after benchmarks           |
+--------------------------------------------------------------------+
| Overall DX           | 6/10   | Phase 1 addresses the 3 worst gaps   |
+--------------------------------------------------------------------+
```

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| DX Review | `/plan-devex-review` | Developer experience gaps in optimization plan | 1 | issues_found | score: 4/10 → 6/10, narrow scope to Phase 1 |
| Outside Voice | Codex | Independent plan challenge | 1 | 18 issues found | Scope narrowed, rollback removed, CheckStatus restructured |
