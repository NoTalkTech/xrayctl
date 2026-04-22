package service

import (
	"io"
	"os"
	"testing"
)

// TestPromptValueAcceptsPersistedOnClosedStdin pins the non-interactive
// contract that `xrayctl --install --domain x --email y < /dev/null`
// depends on: once a flag pre-populates the value, promptValue must
// accept it without blocking for input that will never come.
func TestPromptValueAcceptsPersistedOnClosedStdin(t *testing.T) {
	// Replace os.Stdin with an already-closed pipe so ReadString immediately
	// returns io.EOF with an empty line — exactly what the runtime sees for
	// `cmd < /dev/null`.
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
		r.Close()
	})

	// current is already valid, so the EOF-on-confirm branch should
	// validate and return it, not spin or exit.
	got := promptValue("域名", "example.com", validateDomain)
	if got != "example.com" {
		t.Errorf("promptValue returned %q, want %q", got, "example.com")
	}

	// Sanity: confirm we really drained the pipe to EOF and aren't just
	// lucky — a second read must still report EOF immediately.
	buf := make([]byte, 1)
	if _, err := r.Read(buf); err != io.EOF {
		t.Errorf("expected stdin at EOF, got err=%v", err)
	}
}
