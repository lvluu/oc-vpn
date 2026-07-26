package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDir(t *testing.T) {
	t.Setenv("SUDO_USER", "")
	t.Setenv("OC_VPN_CONFIG_DIR", "")

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "oc-vpn")
	if got := Dir(); got != expected {
		t.Errorf("Dir() = %q, want %q", got, expected)
	}

	t.Setenv("OC_VPN_CONFIG_DIR", "/tmp/test-ocvpn")
	if got := Dir(); got != "/tmp/test-ocvpn" {
		t.Errorf("Dir() with env override = %q, want /tmp/test-ocvpn", got)
	}
}

func TestProfilesDir(t *testing.T) {
	t.Setenv("OC_VPN_CONFIG_DIR", "/tmp/test-ocvpn")
	t.Setenv("OC_VPN_PROFILES_DIR", "")

	expected := "/tmp/test-ocvpn/profiles"
	if got := ProfilesDir(); got != expected {
		t.Errorf("ProfilesDir() = %q, want %q", got, expected)
	}

	t.Setenv("OC_VPN_PROFILES_DIR", "/custom/profiles")
	if got := ProfilesDir(); got != "/custom/profiles" {
		t.Errorf("ProfilesDir() with env override = %q, want /custom/profiles", got)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, CurrentVersion)
	}
	if cfg.DefaultProfile != "" {
		t.Errorf("DefaultProfile = %q, want empty", cfg.DefaultProfile)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_CONFIG_DIR", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() on missing config should return defaults: %v", err)
	}
	if cfg.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, CurrentVersion)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_CONFIG_DIR", tmpDir)

	cfg := Default()
	if err := cfg.SetDefault("us-east"); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.DefaultProfile != "us-east" {
		t.Errorf("DefaultProfile = %q, want us-east", loaded.DefaultProfile)
	}
}

func TestValidateBadVersion(t *testing.T) {
	cfg := Config{Version: 99}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for bad version")
	}
}

func TestSetDefault(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_CONFIG_DIR", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetDefault("us-east"); err != nil {
		t.Fatalf("SetDefault error = %v", err)
	}
	if cfg.DefaultProfile != "us-east" {
		t.Errorf("DefaultProfile = %q, want us-east", cfg.DefaultProfile)
	}
}

func TestClearDefault(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_CONFIG_DIR", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetDefault("us-east"); err != nil {
		t.Fatal(err)
	}
	if got := cfg.DefaultProfile; got != "us-east" {
		t.Fatalf("DefaultProfile = %q, want us-east", got)
	}
	cfg.ClearDefault()
	if cfg.DefaultProfile != "" {
		t.Errorf("DefaultProfile should be empty after ClearDefault")
	}
}

func TestDirPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_CONFIG_DIR", tmpDir)

	cfgPath := Path()
	expected := filepath.Join(tmpDir, "config.json")
	if cfgPath != expected {
		t.Errorf("Path() = %q, want %q", cfgPath, expected)
	}
}

func TestSaveCreatesDir(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "deep", "nested")
	t.Setenv("OC_VPN_CONFIG_DIR", tmpDir)

	cfg := Default()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(Path()); os.IsNotExist(err) {
		t.Error("config.json should exist after Save")
	}
}
