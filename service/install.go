package service

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
