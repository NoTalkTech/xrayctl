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

	if !ShouldShowWizard() {
		t.Error("ShouldShowWizard() = false when no config exists, want true")
	}
}

func TestShouldShowWizard_ConfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	config.ConfigPath = configPath
	if err := os.WriteFile(configPath, []byte("domain: example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Create the install-complete marker so the wizard is NOT shown.
	oldMarker := installCompleteMarker
	installCompleteMarker = filepath.Join(tmpDir, ".install-complete")
	defer func() { installCompleteMarker = oldMarker }()
	if err := os.WriteFile(installCompleteMarker, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if ShouldShowWizard() {
		t.Error("ShouldShowWizard() = true when config and marker exist, want false")
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
	// Override the marker path to use tmpDir.
	oldMarker := installCompleteMarker
	installCompleteMarker = filepath.Join(tmpDir, ".install-complete")
	defer func() { installCompleteMarker = oldMarker }()

	if !ShouldShowWizard() {
		t.Error("ShouldShowWizard() = false when config exists but install incomplete, want true")
	}
}

func TestDetectNginxUser_Debian(t *testing.T) {
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
