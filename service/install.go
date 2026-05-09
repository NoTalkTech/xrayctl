package service

import (
	"fmt"

	"xrayctl/config"
)

// InstallAll runs the full installation sequence.
func InstallAll(cfg *config.Config) error {
	if err := InstallBase(); err != nil {
		return fmt.Errorf("install base: %w", err)
	}

	if err := SetupCert(cfg); err != nil {
		return fmt.Errorf("setup cert: %w", err)
	}

	if err := SetupNginx(cfg); err != nil {
		return fmt.Errorf("setup nginx: %w", err)
	}

	if err := SetupWarp(cfg); err != nil {
		return fmt.Errorf("setup warp: %w", err)
	}

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
