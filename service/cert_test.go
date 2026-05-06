package service

import (
	"io"
	"os"
	"strings"
	"testing"
)

func withClosedStdin(t *testing.T) *os.File {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		if err := r.Close(); err != nil {
			t.Errorf("close pipe reader: %v", err)
		}
	})

	return r
}

// TestPromptValueAcceptsPersistedOnClosedStdin pins the non-interactive
// contract that `xrayctl --install --domain x --email y < /dev/null`
// depends on: once a flag pre-populates the value, promptValue must
// accept it without blocking for input that will never come.
func TestPromptValueAcceptsPersistedOnClosedStdin(t *testing.T) {
	// Replace os.Stdin with an already-closed pipe so ReadString immediately
	// returns io.EOF with an empty line — exactly what the runtime sees for
	// `cmd < /dev/null`.
	r := withClosedStdin(t)

	// current is already valid, so the EOF-on-confirm branch should
	// validate and return it, not spin or exit.
	got, err := promptValue("域名", "example.com", validateDomain)
	if err != nil {
		t.Fatalf("promptValue returned error: %v", err)
	}
	if got != "example.com" {
		t.Errorf("promptValue returned %q, want %q", got, "example.com")
	}

	got, err = promptValue("邮箱", "admin@example.com", validateEmail)
	if err != nil {
		t.Fatalf("promptValue returned error: %v", err)
	}
	if got != "admin@example.com" {
		t.Errorf("promptValue returned %q, want %q", got, "admin@example.com")
	}

	// Sanity: confirm we really drained the pipe to EOF and aren't just
	// lucky — a second read must still report EOF immediately.
	buf := make([]byte, 1)
	if _, err := r.Read(buf); err != io.EOF {
		t.Errorf("expected stdin at EOF, got err=%v", err)
	}
}

func TestPromptValueReturnsErrorOnClosedStdinWithoutPersistedValue(t *testing.T) {
	withClosedStdin(t)

	got, err := promptValue("域名", "", validateDomain)
	if err == nil {
		t.Fatalf("promptValue returned nil error with value %q, want missing-input error", got)
	}
	if got != "" {
		t.Errorf("promptValue returned value %q, want empty value", got)
	}

	errText := err.Error()
	for _, want := range []string{"域名", "stdin 已关闭", "没有有效的已保存值"} {
		if !strings.Contains(errText, want) {
			t.Errorf("promptValue error %q does not contain %q", errText, want)
		}
	}
}
