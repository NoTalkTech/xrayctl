package service

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestFormatProgressStep(t *testing.T) {
	tests := []struct {
		current, total int
		step           string
		want           string
	}{
		{1, 5, "Installing base dependencies...", "[1/5] Installing base dependencies..."},
		{3, 5, "Configuring Nginx...", "[3/5] Configuring Nginx..."},
		{5, 5, "Installing Xray core...", "[5/5] Installing Xray core..."},
	}
	for _, tt := range tests {
		got := formatProgressStep(tt.current, tt.total, tt.step)
		if got != tt.want {
			t.Errorf("formatProgressStep(%d, %d, %q) = %q, want %q", tt.current, tt.total, tt.step, got, tt.want)
		}
	}
}

func TestPrintProgressStepOutput(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printProgressStep(1, 5, "Installing base dependencies...")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "[1/5] Installing base dependencies...") {
		t.Errorf("output = %q, want to contain progress step", output)
	}
}
