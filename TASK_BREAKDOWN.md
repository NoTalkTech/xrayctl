# xrayctl — Task Breakdown

Generated from the first-principles revisit of `OPTIMIZATION_PLAN.md`.
Phase 1 is framed as **fail-safe root installer cleanup**, not generic DX cleanup.

---

## Phase 1 — Core Fixes (execution order)

### Task 1. Remove Service-Layer Exit

**Files:** `service/cert.go`, `service/nginx.go`, `service/cert_test.go`

**Change:**
- `promptValue` returns `(string, error)` instead of calling `os.Exit(1)` on closed stdin with no persisted value.
- `SetupCert` and `SetupNginxVlessConf` propagate wrapped errors upward.
- Callers in `cli/flags.go` and `cli/menu.go` already handle returned errors.

**Acceptance criteria:**
- `service/` 包不再负责进程退出。
- stdin 关闭且没有可用持久化值时，调用方能收到明确错误。
- 非交互安装在已有合法 domain/email 时仍能正常接受这些值。
- 相关测试覆盖 EOF 成功路径和 EOF 缺失输入失败路径。

**Dependencies:** none

---

### Task 2. Use Random UUID For New Installs

**Files:** `service/xray.go`, `service/xray_test.go`, `CLAUDE.md`, `README.md` (if needed)

**Change:**
- Remove MD5 email-derived UUID fallback (`generateUUIDFromEmail`).
- New UUID resolution: `cfg.UUID` → existing on-disk Xray config UUID → random UUID.
- Tests assert UUID shape/preservation, not deterministic email behavior.

**Why early:** Small, low-coupling, security-relevant (predictable UUID from email).

**Acceptance criteria:**
- 新安装不再从 email 派生 UUID。
- 显式配置 UUID 优先级保持最高。
- 已安装 Xray 配置中的 UUID 仍能被复用。
- 只有没有显式/既有 UUID 时才生成随机 UUID。
- 测试不再依赖 email 派生的确定性 UUID。
- `CLAUDE.md`/`README.md` 中相关 UUID 描述与新行为一致。

**Dependencies:** none

---

### Task 3. Extract Shared Install Orchestrator

**Files:** `service/install.go`, `cli/flags.go`, `cli/menu.go`

**Change:**
- Add `service.InstallAll(cfg)` for the fixed install sequence:
  `InstallBase → SetupCert → SetupNginx → SetupWarp → SetupXray → CheckStatus`
- Each step wraps its error with context: `fmt.Errorf("setup cert: %w", err)`.
- Replace the duplicated install logic in `cli/flags.go` (flag path) and `cli/menu.go` (menu option 1) with calls to `InstallAll`.

**Acceptance criteria:**
- flag 安装路径和 menu 安装路径共用同一个安装编排入口。
- 安装步骤顺序只在一个地方维护。
- 任一步骤失败时，后续步骤不会继续执行。
- 错误带有足够上下文，CLI 仍负责中文用户输出。
- 不引入 context、runner 参数、rollback、dry-run 或 runtime abstraction。

**Dependencies:** Task 1

---

### Task 4. Split Status Collection From Rendering

**Files:** `service/health.go`, `cli/flags.go`, `cli/menu.go`, `service/install.go`

**Change:**
- Separate status data collection from rendering: introduce a report type (service status strings, WARP IP, connection parameters, share link), a collector function, and a printer function.
- Keep the old combined function temporarily as a compatibility wrapper for callers that still expect it.
- **Key nuance:** A down service is useful status output; it should only be a hard error in validation contexts (post-install, `--check`), not on every `--status` call. Callers choose context — render-only for `--status`, render-and-evaluate for validation.

**Acceptance criteria:**
- 状态采集和终端渲染分离。
- 调用方可以不解析 stdout 就获得状态结果。
- `--status` 只负责展示状态，不把服务异常默认视为命令失败。
- post-install 或未来 `--check` 可以选择把异常状态视为验证失败。
- 现有中文状态输出体验保持基本一致。

**Dependencies:** Task 3

---

### Task 5. Make Environment Check Observable

**Files:** `service/base.go`, `cli/menu.go`

**Change:**
- Introduce `EnvironmentReport` / collection path.
- Make `CheckSystemEnvironment` return `error`.
- Fix the misleading "依赖安装完成，请重试" message that appears when dependency installation actually failed.

**Acceptance criteria:**
- 环境检查结果可被调用方读取，而不是只打印。
- 依赖安装失败会返回明确错误。
- 不再在失败路径输出类似"安装完成"的误导信息。
- 交互式菜单现有行为保持熟悉，不扩展成复杂诊断框架。

**Dependencies:** Task 1

---

### Task 6. Phase 1 Validation Pass

**Files:** none (unless validation reveals fixes)

**Acceptance criteria:**
- 全量测试通过。
- lint 通过。
- `service/` 包不再包含进程退出调用。
- 新安装 UUID 策略符合"显式值 → 既有值 → 随机值"。
- flag/menu 安装路径共享同一编排。
- status/environment 检查具备调用方可消费的结果。
- 变更仍保持小型、过程式 CLI 设计。

**Dependencies:** Tasks 1–5

---

## Phase 2 — Deferred Work

### Task 7. Targeted CommandRunner Injection

**Scope boundary:** Service functions that have a concrete need for cancellation or isolated concurrent shell-out tests. Not a blanket interface injection.

**Change:**
- Add explicit `internal.CommandRunner` parameters to select service functions where testing or cancellation demands it.
- Avoid a broad runtime/dependency-bag abstraction.
- The existing `internal.DefaultRunner` package-level seam is sufficient until a specific need arises.

**Acceptance criteria:**
- 只对有明确测试或取消需求的 service 函数注入 runner。
- 没有引入 Runtime/dependency bag。
- 不需要 runner 的函数仍保持现有简单调用方式。
- 新增 runner seam 有对应测试收益，而不是为了抽象而抽象。

**Dependencies:** Task 6

---

### Task 8. Context Propagation

**Scope boundary:** Long-running shell-out paths within service functions that have already adopted explicit runner parameters.

**Change:**
- Thread `context.Context` through selected shell-out paths after they stop relying only on package-level `Exec*` helpers.
- Enables timeout and cancellation for operations like cert issuance, apt installs, and WARP registration.

**Acceptance criteria:**
- 至少一个长耗时 shell-out 路径支持取消或超时语义。
- context 只在已具备 runner seam 的路径中传播。
- 没有制造"看似可取消但底层仍不可取消"的假语义。
- 旧调用方仍有清晰默认行为。

**Dependencies:** Task 7

---

### Task 9. Preflight and Dry-Run Foundations

**Scope boundary:** CLI flags that validate system state without mutating it.

**Change:**
- `--check` mode: runs environment inspection and status collection (Tasks 4–5) and reports failures without installing.
- `--dry-run` mode: deferred until file writes, package installs, and service restarts have testable seams.

**Acceptance criteria:**
- `--check` 只读取系统状态，不执行安装或修改操作。
- `--check` 复用结构化 environment/status 结果。
- dry-run 不在缺少文件写入、包安装、service restart seam 前承诺可用。
- dry-run 的阻塞前提被明确记录，避免给用户虚假安全感。

**Dependencies:** Tasks 4, 5, 8

---

### Task 10. Maintenance Hardening

**Scope boundary:** Lower-risk improvements across config, base, backup, and docs that are safe to defer.

**Change:**
- `config/manager.go`: explicit `ApplyDefaults(*Config)` replacing reflection-based default filling.
- `service/base.go`: BBR sysctl idempotency — prevent duplicate sysctl lines on repeated install.
- `service/backup.go`: tar path validation — reject `..`, absolute paths, and symlink escapes before extracting.
- Service-wide: `bash -c` audit — replace simple calls with argv or Go file APIs; keep shell only where pipes or compound commands are required.
- `service/uninstall.go`: return errors instead of void, consistent with Phase 1 patterns.
- Docs: Makefile, CONTRIBUTING.md (added after Phase 1 patterns settle).

**Acceptance criteria:**
- 每个 hardening 项独立拆分、独立验证，不混成大重构。
- config default、BBR 幂等、backup restore 安全、shell 调用审计、uninstall error model 都有明确处理或明确延期理由。
- 不破坏现有单向依赖图。
- Makefile/CONTRIBUTING 只在 Phase 1 模式稳定后补充。

**Dependencies:** Task 6
