# xrayctl UX Polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Polish xrayctl's onboarding and first-run UX for non-expert users: rewritten README, guided setup wizard, progress indicators, plain-language error messages, and `--version` flag.

**Architecture:** New file `cli/wizard.go` for guided wizard (separate from `cli/menu.go` loop). New file `internal/errtrans.go` for centralized error translation with regex catalog. Progress indicators added at `InstallAll` level (5 PrintGreen calls). First-run detection via `shouldShowWizard()` extracted for testability. Partial-install detection via `.install-complete` sentinel file.

**Tech Stack:** Go 1.21, `gopkg.in/yaml.v3`, existing `internal.Print*` color output functions, `text/template` for nginx config.

**Source documents:**
- Design doc: `~/.gstack/projects/NoTalkTech-xrayctl/biyu.huang-master-design-20260608-170246.md`
- Test plan: `~/.gstack/projects/NoTalkTech-xrayctl/biyu.huang-master-eng-review-test-plan-20260608-181540.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/constants.go` | Modify | Add `Version`, `InstallCompleteMarker` constants |
| `internal/errtrans.go` | Create | Error translation: regex catalog + `TranslateError()` |
| `internal/errtrans_test.go` | Create | Tests: 3 error patterns × 2 distros = 6 cases |
| `cli/wizard.go` | Create | Guided setup wizard: `ShowGuidedSetup()`, `shouldShowWizard()`, `detectNginxUser()` |
| `cli/wizard_test.go` | Create | Tests: wizard routing, DNS check, input validation |
| `cli/menu.go` | Modify | Simplified TUI header (lines 42-43) |
| `cli/flags.go` | Modify | Add `--version` flag parsing + output |
| `cmd/main.go` | Modify | First-run routing: wizard vs menu |
| `service/install.go` | Modify | Progress indicators `[1/5]`...`[5/5]` |
| `service/health.go` | Modify | Post-install "What now?" guidance |
| `README.md` | Modify | Non-expert-first rewrite: value prop, quick start, FAQ |

---

### Task 1: Add Version and Install-Complete Constants

**Files:**
- Modify: `internal/constants.go:32-33`

- [ ] **Step 1: Add constants to internal/constants.go**

Append after the existing `AcmeRootPath` block (line 31):

```go
// InstallCompleteMarker is touched after a successful full install; its
// absence signals a partial or never-completed installation to the wizard.
const InstallCompleteMarker = "/etc/xrayctl/.install-complete"

// Version is set at build time via ldflags. The default is shown when
// built with go build (no -ldflags).
var Version = "dev"
```

- [ ] **Step 2: Verify compilation**

Run: `go build -o /dev/null ./...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/constants.go
git commit -m "feat: add Version var and InstallCompleteMarker constant"
```

---

### Task 2: Add --version Flag

**Files:**
- Modify: `cli/flags.go:18-33`

- [ ] **Step 1: Add version flag to ParseFlags**

In `cli/flags.go`, add to the var block (around line 20, before `flag.Parse()`):

```go
version = flag.Bool("version", false, "显示版本信息")
```

- [ ] **Step 2: Add --version to the actions list and mutual-exclusion check**

Add to the `actions` slice (after line 59, before the closing `}`):

```go
{*version, "--version"},
```

- [ ] **Step 3: Add version dispatch in executeFlagAction**

In `executeFlagAction`, add a case before the `install` case (after `switch {`):

```go
case *version:
	fmt.Printf("xrayctl %s\n", internal.Version)
```

- [ ] **Step 4: Verify compilation and test**

Run: `go build -o /dev/null ./...`
Expected: exit 0.

Run: `go vet ./cli/...`
Expected: exit 0.

- [ ] **Step 5: Test --version with ldflags**

```bash
go build -ldflags "-X xrayctl/internal.Version=v1.0.0-$(git rev-parse --short HEAD)" -o /tmp/xrayctl-test ./cmd
/tmp/xrayctl-test --version
```

Expected output: `xrayctl v1.0.0-<commit-hash>`

- [ ] **Step 6: Commit**

```bash
git add cli/flags.go
git commit -m "feat: add --version flag with ldflags-injected version"
```

---

### Task 3: Add Progress Indicators to Install Pipeline

**Files:**
- Modify: `service/install.go:10-39`

- [ ] **Step 1: Write the failing test**

Create test in `service/install_test.go` (or create if not exists — check first):

```bash
test -f service/install_test.go && echo "EXISTS" || echo "NOT_FOUND"
```

If `NOT_FOUND`, create `service/install_test.go`. Add test:

```go
package service

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// captureOutput captures stdout during a function call.
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestInstallAllPrintsProgressSteps(t *testing.T) {
	// This test verifies the progress output format without actually running
	// the install. It tests the helper that prints progress lines.
	steps := []string{
		"Installing base dependencies...",
		"Issuing SSL certificate...",
		"Configuring Nginx...",
		"Setting up WARP proxy...",
		"Installing Xray core...",
	}
	output := captureOutput(func() {
		for i, step := range steps {
			printProgressStep(i+1, len(steps), step)
		}
	})

	for i, step := range steps {
		expected := formatProgressStep(i+1, len(steps), step)
		if !strings.Contains(output, expected) {
			t.Errorf("output missing step %d: want %q in output\nGot: %s", i+1, expected, output)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./service -run TestInstallAllPrintsProgressSteps -v`
Expected: FAIL — `printProgressStep` / `formatProgressStep` not defined.

- [ ] **Step 3: Add progress helpers and wire into InstallAll**

In `service/install.go`, add helper functions and modify `InstallAll`:

```go
import (
	"fmt"

	"xrayctl/config"
	"xrayctl/internal"
)

// printProgressStep prints a colorized progress line.
func printProgressStep(current, total int, step string) {
	internal.PrintGreenRaw("[%d/%d] %s\n", current, total, step)
}

// formatProgressStep returns the expected formatted string for a progress step.
// Exported for testability.
func formatProgressStep(current, total int, step string) string {
	return fmt.Sprintf("[%d/%d] %s", current, total, step)
}

// InstallAll runs the full installation sequence.
func InstallAll(cfg *config.Config) error {
	totalSteps := 5

	printProgressStep(1, totalSteps, "Installing base dependencies...")
	if err := InstallBase(); err != nil {
		return fmt.Errorf("install base: %w", err)
	}

	printProgressStep(2, totalSteps, "Issuing SSL certificate...")
	if err := SetupCert(cfg); err != nil {
		return fmt.Errorf("setup cert: %w", err)
	}

	printProgressStep(3, totalSteps, "Configuring Nginx...")
	if err := SetupNginx(cfg); err != nil {
		return fmt.Errorf("setup nginx: %w", err)
	}

	printProgressStep(4, totalSteps, "Setting up WARP proxy...")
	if err := SetupWarp(cfg); err != nil {
		return fmt.Errorf("setup warp: %w", err)
	}

	printProgressStep(5, totalSteps, "Installing Xray core...")
	if err := SetupXray(cfg); err != nil {
		return fmt.Errorf("setup xray: %w", err)
	}

	report := CollectStatus(cfg)
	PrintStatusReport(report)

	if err := report.ValidationError(); err != nil {
		return fmt.Errorf("validate status: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./service -run TestInstallAllPrintsProgressSteps -v`
Expected: PASS.

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -v -race`
Expected: all existing tests PASS.

- [ ] **Step 6: Commit**

```bash
git add service/install.go service/install_test.go
git commit -m "feat: add [1/5]...[5/5] progress indicators to install pipeline"
```

---

### Task 4: Error Translation Layer

**Files:**
- Create: `internal/errtrans.go`
- Create: `internal/errtrans_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/errtrans_test.go`:

```go
package internal

import (
	"errors"
	"testing"
)

func TestTranslateError_PackageNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "Debian apt",
			err:  errors.New("E: Unable to locate package nginx"),
			want: "软件包未找到 — 请确认操作系统版本受支持（Debian 11+ / Ubuntu 20.04+ / CentOS Stream 8+），然后运行 apt update 后重试",
		},
		{
			name: "CentOS yum",
			err:  errors.New("Error: Unable to find a match: nginx"),
			want: "软件包未找到 — 请确认操作系统版本受支持（Debian 11+ / Ubuntu 20.04+ / CentOS Stream 8+），然后运行 yum update 后重试",
		},
		{
			name: "Ubuntu apt",
			err:  errors.New("No package nginx available"),
			want: "软件包未找到 — 请确认操作系统版本受支持（Debian 11+ / Ubuntu 20.04+ / CentOS Stream 8+），然后运行 apt update 后重试",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateError(tt.err)
			if got != tt.want {
				t.Errorf("TranslateError(%q) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestTranslateError_PortConflict(t *testing.T) {
	err := errors.New("listen tcp :80: bind: address already in use")
	want := "端口 80 已被占用 — 可能有其他 Web 服务器（Nginx/Apache）正在运行。请先停止占用端口的服务后重试"
	got := TranslateError(err)
	if got != want {
		t.Errorf("TranslateError(%q) = %q, want %q", err, got, want)
	}
}

func TestTranslateError_DNSFailure(t *testing.T) {
	err := errors.New("Verify error: Invalid response from https://acme-v02.api.letsencrypt.org")
	want := "证书申请失败 — 请确认域名已正确解析到此服务器的 IP 地址。DNS 记录生效可能需要几分钟"
	got := TranslateError(err)
	if got != want {
		t.Errorf("TranslateError(%q) = %q, want %q", err, got, want)
	}
}

func TestTranslateError_UnknownPassthrough(t *testing.T) {
	err := errors.New("some unrecognized error message")
	got := TranslateError(err)
	if got != err.Error() {
		t.Errorf("TranslateError(%q) = %q, want passthrough of original %q", err, got, err.Error())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal -run TestTranslateError -v`
Expected: FAIL — `TranslateError` not defined.

- [ ] **Step 3: Implement error translation**

Create `internal/errtrans.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal -run TestTranslateError -v`
Expected: 7 subtests PASS (4 PackageNotFound + 1 PortConflict + 1 DNSFailure + 1 UnknownPassthrough).

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -v -race`
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/errtrans.go internal/errtrans_test.go
git commit -m "feat: add plain-language error translation for top 3 failure modes"
```

---

### Task 5: Simplify TUI Header

**Files:**
- Modify: `cli/menu.go:41-44`

- [ ] **Step 1: Update the header text**

In `cli/menu.go`, replace lines 41-44:

Old:
```go
		internal.PrintGreen("==========================================")
		internal.PrintGreen("|      Xray-WARP 精准分流管理系统        |")
		internal.PrintGreen("|      (VLESS + XTLS + Nginx Fallback)   |")
		internal.PrintGreen("==========================================")
```

New:
```go
		internal.PrintGreen("==========================================")
		internal.PrintGreen("|           自建 VPN 管理工具            |")
		internal.PrintGreen("|     一键安装 · 智能分流 · 自动续签     |")
		internal.PrintGreen("==========================================")
```

- [ ] **Step 2: Verify compilation**

Run: `go build -o /dev/null ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add cli/menu.go
git commit -m "feat: simplify TUI header — replace VLESS jargon with plain Chinese"
```

---

### Task 6: Guided Setup Wizard

**Files:**
- Create: `cli/wizard.go`
- Create: `cli/wizard_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cli/wizard_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"xrayctl/config"
)

func TestShouldShowWizard_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	config.ConfigPath = filepath.Join(tmpDir, "nonexistent.yaml")

	if !shouldShowWizard() {
		t.Error("shouldShowWizard() = false when no config exists, want true")
	}
}

func TestShouldShowWizard_ConfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	config.ConfigPath = configPath
	if err := os.WriteFile(configPath, []byte("domain: example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if shouldShowWizard() {
		t.Error("shouldShowWizard() = true when config exists, want false")
	}
}

func TestShouldShowWizard_PartialInstall(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	config.ConfigPath = configPath
	// Config exists but .install-complete marker does not.
	if err := os.WriteFile(configPath, []byte("domain: example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Reset the marker path to use tmpDir.
	oldMarker := installCompleteMarker
	installCompleteMarker = filepath.Join(tmpDir, ".install-complete")
	defer func() { installCompleteMarker = oldMarker }()

	if !shouldShowWizard() {
		t.Error("shouldShowWizard() = false when config exists but install incomplete, want true")
	}
}

func TestDetectNginxUser_Debian(t *testing.T) {
	// Unit test: detectNginxUser checks /etc/passwd via a passed-in reader.
	// We test the logic by providing mock passwd content.
	passwdContent := "root:x:0:0:root:/root:/bin/bash\nwww-data:x:33:33:www-data:/var/www:/usr/sbin/nologin\n"
	got := detectNginxUserFromPasswd(passwdContent)
	if got != "www-data" {
		t.Errorf("detectNginxUserFromPasswd() = %q, want %q", got, "www-data")
	}
}

func TestDetectNginxUser_RHEL(t *testing.T) {
	passwdContent := "root:x:0:0:root:/root:/bin/bash\nnginx:x:996:994:nginx user:/var/cache/nginx:/sbin/nologin\n"
	got := detectNginxUserFromPasswd(passwdContent)
	if got != "nginx" {
		t.Errorf("detectNginxUserFromPasswd() = %q, want %q", got, "nginx")
	}
}

func TestDetectNginxUser_Default(t *testing.T) {
	passwdContent := "root:x:0:0:root:/root:/bin/bash\n"
	got := detectNginxUserFromPasswd(passwdContent)
	if got != "nginx" {
		t.Errorf("detectNginxUserFromPasswd() = %q, want default %q", got, "nginx")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli -run "TestShouldShowWizard|TestDetectNginxUser" -v`
Expected: FAIL — functions not defined.

- [ ] **Step 3: Implement cli/wizard.go**

Create `cli/wizard.go`:

```go
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"xrayctl/config"
	"xrayctl/internal"
	"xrayctl/service"
)

// installCompleteMarker is the sentinel file touched after a successful install.
// Set as a var so tests can override it.
var installCompleteMarker = internal.InstallCompleteMarker

// shouldShowWizard returns true when the guided wizard should run:
// config file doesn't exist, can't be loaded, or install was never completed.
func shouldShowWizard() bool {
	if _, err := os.Stat(config.ConfigPath); os.IsNotExist(err) {
		return true
	}
	if _, err := config.LoadConfigReadOnly(); err != nil {
		return true
	}
	if _, err := os.Stat(installCompleteMarker); os.IsNotExist(err) {
		return true
	}
	return false
}

// ShowGuidedSetup runs the guided first-run wizard.
func ShowGuidedSetup() {
	if !internal.IsRoot() {
		internal.PrintRed("错误: 请使用 root 运行！")
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)

	internal.PrintGreen("==========================================")
	internal.PrintGreen("|         欢迎使用 xrayctl 安装向导       |")
	internal.PrintGreen("|      让我们一步步完成 VPN 服务部署      |")
	internal.PrintGreen("==========================================")
	fmt.Println()

	// Step 1: Domain
	domain := promptDomain(scanner)

	// Step 2: Email
	email := promptEmail(scanner)

	// Pre-flight checks
	fmt.Println()
	internal.PrintYellow(">>> 环境预检 <<<")

	showPreflightWarnings(domain)

	// Confirmation
	fmt.Println()
	internal.PrintGreen("==========================================")
	internal.PrintGreen("  安装确认")
	internal.PrintGreen("==========================================")
	fmt.Printf("  域名:   %s\n", domain)
	fmt.Printf("  邮箱:   %s\n", email)
	fmt.Println()
	fmt.Print("确认开始安装？(Y/n): ")

	if !scanner.Scan() {
		fmt.Println()
		internal.PrintYellow("安装已取消。")
		return
	}
	confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if confirm != "" && confirm != "y" && confirm != "yes" {
		internal.PrintYellow("安装已取消。")
		return
	}

	// Build config and install
	cfg := &config.Config{
		Domain: domain,
		Email:  email,
	}
	config.ApplyDefaults(cfg)

	fmt.Println()
	if err := service.InstallAll(cfg); err != nil {
		internal.PrintRed("安装失败: %v", internal.TranslateError(err))
		fmt.Println()
		internal.PrintYellow("常见问题排查：")
		internal.PrintYellow("  1. 确认域名已正确解析到此服务器 IP")
		internal.PrintYellow("  2. 确认 80/443 端口未被其他服务占用")
		internal.PrintYellow("  3. 运行 sudo ./xrayctl 重新开始安装")
		return
	}

	// Mark install as complete
	if err := touchInstallComplete(); err != nil {
		internal.PrintYellow("无法写入安装标记: %v", err)
	}

	// Post-install guidance
	fmt.Println()
	internal.PrintGreen("==========================================")
	internal.PrintGreen("|        ✅ 安装完成！                   |")
	internal.PrintGreen("==========================================")
	fmt.Println()
	internal.PrintGreen("🎉 VPN 服务已成功部署！")
	fmt.Println()
	fmt.Println("接下来你可以：")
	fmt.Println("  sudo ./xrayctl              — 打开管理菜单，查看服务状态")
	fmt.Println("  sudo ./xrayctl --status     — 查看连接参数和分享链接")
	fmt.Println("  sudo ./xrayctl --backup     — 备份所有配置和证书")
	fmt.Println()
	internal.PrintYellow("提示：如需修改分流域名，编辑 /etc/xrayctl/config.yaml 后重启 Xray 即可。")
}

func promptDomain(scanner *bufio.Scanner) string {
	for {
		fmt.Print("请输入你的域名 (如 vpn.example.com): ")
		if !scanner.Scan() {
			fmt.Println()
			internal.PrintYellow("输入已结束，退出向导。")
			os.Exit(0)
		}
		domain := strings.TrimSpace(scanner.Text())
		if domain == "" {
			internal.PrintRed("域名不能为空，请重新输入。")
			continue
		}
		return domain
	}
}

func promptEmail(scanner *bufio.Scanner) string {
	for {
		fmt.Print("请输入证书申请邮箱 (acme.sh 注册用): ")
		if !scanner.Scan() {
			fmt.Println()
			internal.PrintYellow("输入已结束，退出向导。")
			os.Exit(0)
		}
		email := strings.TrimSpace(scanner.Text())
		if email == "" {
			internal.PrintRed("邮箱不能为空，请重新输入。")
			continue
		}
		return email
	}
}

func showPreflightWarnings(domain string) {
	// Check if domain resolves (best-effort, via system resolver hint).
	internal.PrintYellow("域名解析检查：请在浏览器确认 %s 已指向本服务器 IP", domain)

	// Distro nginx user check.
	passwdBytes, err := os.ReadFile("/etc/passwd")
	if err == nil {
		detectedUser := detectNginxUserFromPasswd(string(passwdBytes))
		if detectedUser != "nginx" {
			internal.PrintYellow("检测到当前系统 Nginx 用户为 '%s'（非默认 'nginx'）", detectedUser)
		}
	}
}

// detectNginxUserFromPasswd scans passwd content and returns the appropriate
// nginx user: "www-data" if present (Debian/Ubuntu), "nginx" otherwise.
func detectNginxUserFromPasswd(passwdContent string) string {
	for _, line := range strings.Split(passwdContent, "\n") {
		if strings.HasPrefix(line, "www-data:") {
			return "www-data"
		}
	}
	return "nginx"
}

// touchInstallComplete creates the marker file indicating a successful install.
func touchInstallComplete() error {
	dir := config.DefaultConfigDir
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(installCompleteMarker)
	if err != nil {
		return fmt.Errorf("create marker: %w", err)
	}
	f.Close()
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cli -run "TestShouldShowWizard|TestDetectNginxUser" -v`
Expected: 6 subtests PASS.

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -v -race`
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/wizard.go cli/wizard_test.go
git commit -m "feat: add guided setup wizard for first-run experience"
```

---

### Task 7: Wire First-Run Routing in main.go

**Files:**
- Modify: `cmd/main.go:10-17`

- [ ] **Step 1: Update main.go routing**

Replace the entire `main()` function:

Old:
```go
func main() {
	switch cli.ParseFlags() {
	case -1:
		cli.ShowMenu()
	case 1:
		os.Exit(1)
	}
}
```

New:
```go
func main() {
	switch cli.ParseFlags() {
	case -1:
		// No flags provided — decide between wizard and menu.
		if cli.ShouldShowWizard() {
			cli.ShowGuidedSetup()
		} else {
			cli.ShowMenu()
		}
	case 1:
		os.Exit(1)
	}
}
```

Note: `shouldShowWizard` is unexported in `cli/wizard.go`. Export it by capitalizing the function name to `ShouldShowWizard`. Update `cli/wizard.go`:

Old (line in wizard.go):
```go
func shouldShowWizard() bool {
```

New:
```go
// ShouldShowWizard returns true when the guided wizard should run instead of
// the main menu. Exported so cmd/main.go can route without importing config.
func ShouldShowWizard() bool {
```

Update `cli/wizard_test.go` references from `shouldShowWizard` to `ShouldShowWizard`.

- [ ] **Step 2: Verify compilation**

Run: `go build -o /dev/null ./...`
Expected: exit 0.

- [ ] **Step 3: Run tests**

Run: `go test ./cli -run TestShouldShowWizard -v`
Expected: 3 subtests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/main.go cli/wizard.go cli/wizard_test.go
git commit -m "feat: wire first-run wizard routing into cmd/main.go"
```

---

### Task 8: README Rewrite

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Rewrite README with non-expert-first framing**

Replace the current README content (preserving the project structure and routing rules sections) with:

```markdown
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

## 项目结构

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
```

- [ ] **Step 2: Verify markdown renders correctly**

Open README.md in any markdown viewer and check: headings, code blocks, lists, links.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: rewrite README with non-expert-first framing and FAQ"
```

---

### Task 9: Final Test Suite Verification

**Files:**
- All files from Tasks 1-8.

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -v -race -coverprofile=coverage.out
```

Expected: all tests PASS, no race conditions.

- [ ] **Step 2: Run linter**

```bash
golangci-lint run --timeout 5m ./...
```

Expected: exit 0, no lint errors.

- [ ] **Step 3: Cross-compile verification**

```bash
GOOS=linux GOARCH=amd64 go build -ldflags "-X xrayctl/internal.Version=$(git describe --tags --always --dirty)" -o /tmp/xrayctl-linux-amd64 ./cmd
GOOS=linux GOARCH=arm64 go build -ldflags "-X xrayctl/internal.Version=$(git describe --tags --always --dirty)" -o /tmp/xrayctl-linux-arm64 ./cmd
```

Expected: exit 0 for both.

- [ ] **Step 4: Verify --version works**

```bash
/tmp/xrayctl-linux-amd64 --version
```

Expected: prints version string (e.g., `xrayctl 905af8b`).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: final test suite pass — lint, test, cross-compile verified"
```
