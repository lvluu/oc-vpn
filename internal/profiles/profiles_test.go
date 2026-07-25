package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"us-east", false},
		{"us.east", false},
		{"us_east", false},
		{"test123", false},
		{"a", false},
		{"", true},
		{"us/east", true},
		{"us east", true},
		{"us\teast", true},
		{"..", true},
		{"us..east", true},
		{"us/east/../etc", true},
		{"special!chars", true},
		{"spaces at end", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestDir(t *testing.T) {
	// Clear SUDO_USER so Dir() falls back to os.UserHomeDir()
	t.Setenv("SUDO_USER", "")

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "oc-vpn", "profiles")
	if got := Dir(); got != expected {
		t.Errorf("Dir() = %q, want %q", got, expected)
	}

	// Test env override
	t.Setenv("OC_VPN_PROFILES_DIR", "/tmp/test-profiles")
	if got := Dir(); got != "/tmp/test-profiles" {
		t.Errorf("Dir() with OC_VPN_PROFILES_DIR = %q, want /tmp/test-profiles", got)
	}
}

func TestImport(t *testing.T) {
	// Create temp dir with a test config
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")
	config := `[Interface]
PrivateKey = abc123=
Address = 10.0.0.2/24
DNS = 1.1.1.1
MTU = 1420

[Peer]
PublicKey = def456=
Endpoint = 1.2.3.4:51820
AllowedIPs = 0.0.0.0/0
`
	if err := os.WriteFile(confPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set up temp profiles dir
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	// Import
	if err := Import(confPath, "test-profile"); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// Verify profile exists
	p := Get("test-profile")
	if p == nil {
		t.Fatal("Get() returned nil for imported profile")
	}

	// Verify config was copied
	if _, err := os.Stat(p.ConfPath); os.IsNotExist(err) {
		t.Error("Config file not created")
	}

	// Verify metadata was created
	if _, err := os.Stat(p.MetaPath); os.IsNotExist(err) {
		t.Error("Metadata file not created")
	}

	// Verify wg-quick fields were stripped
	cfg, err := ParseConfig(p.ConfPath)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if cfg.Address != "" {
		t.Errorf("Address should be stripped, got %q", cfg.Address)
	}
	if cfg.DNS != "" {
		t.Errorf("DNS should be stripped, got %q", cfg.DNS)
	}
	if cfg.MTU != "" {
		t.Errorf("MTU should be stripped, got %q", cfg.MTU)
	}

	// Verify peer fields are kept
	if cfg.Endpoint != "1.2.3.4:51820" {
		t.Errorf("Endpoint = %q, want 1.2.3.4:51820", cfg.Endpoint)
	}
	if cfg.PublicKey != "def456=" {
		t.Errorf("PublicKey = %q, want def456=", cfg.PublicKey)
	}
	if cfg.AllowedIPs != "0.0.0.0/0" {
		t.Errorf("AllowedIPs = %q, want 0.0.0.0/0", cfg.AllowedIPs)
	}
}

func TestImportInvalidName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	confPath := filepath.Join(tmpDir, "test.conf")
	os.WriteFile(confPath, []byte("[Interface]\n"), 0o644)

	if err := Import(confPath, "../escape"); err == nil {
		t.Error("Import with path traversal name should fail")
	}
}

func TestImportMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	if err := Import("/nonexistent/file.conf", "test"); err == nil {
		t.Error("Import with missing file should fail")
	}
}

func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	// Create a profile
	profileDir := filepath.Join(tmpDir, "to-delete")
	os.MkdirAll(profileDir, 0o755)
	os.WriteFile(filepath.Join(profileDir, "wg.conf"), []byte("[Interface]\n"), 0o644)

	if err := Remove("to-delete"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if Get("to-delete") != nil {
		t.Error("Profile should be deleted")
	}
}

func TestRemoveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	if err := Remove("nonexistent"); err == nil {
		t.Error("Remove() should fail for nonexistent profile")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	// Create some profiles
	for _, name := range []string{"alpha", "beta", "gamma"} {
		dir := filepath.Join(tmpDir, name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "wg.conf"), []byte("[Interface]\n"), 0o644)
	}

	names := List()
	if len(names) != 3 {
		t.Fatalf("List() returned %d items, want 3", len(names))
	}

	// Check all expected names are present
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"alpha", "beta", "gamma"} {
		if !nameSet[expected] {
			t.Errorf("List() missing profile %q", expected)
		}
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	names := List()
	if len(names) != 0 {
		t.Errorf("List() returned %d items on empty dir, want 0", len(names))
	}
}

func TestCount(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	if got := Count(); got != 0 {
		t.Errorf("Count() = %d, want 0", got)
	}

	os.MkdirAll(filepath.Join(tmpDir, "a"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, "b"), 0o755)

	if got := Count(); got != 2 {
		t.Errorf("Count() = %d, want 2", got)
	}
}

func TestParseConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	config := `[Interface]
PrivateKey = testkey=
Address = 10.0.0.2/24
DNS = 1.1.1.1
MTU = 1420
FwMark = 42

[Peer]
PublicKey = peerkey=
Endpoint = vpn.example.com:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`
	os.WriteFile(confPath, []byte(config), 0o644)

	cfg, err := ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if cfg.PrivateKey != "testkey=" {
		t.Errorf("PrivateKey = %q, want testkey=", cfg.PrivateKey)
	}
	if cfg.Address != "10.0.0.2/24" {
		t.Errorf("Address = %q, want 10.0.0.2/24", cfg.Address)
	}
	if cfg.DNS != "1.1.1.1" {
		t.Errorf("DNS = %q, want 1.1.1.1", cfg.DNS)
	}
	if cfg.MTU != "1420" {
		t.Errorf("MTU = %q, want 1420", cfg.MTU)
	}
	if cfg.FwMark != "42" {
		t.Errorf("FwMark = %q, want 42", cfg.FwMark)
	}
	if cfg.PublicKey != "peerkey=" {
		t.Errorf("PublicKey = %q, want peerkey=", cfg.PublicKey)
	}
	if cfg.Endpoint != "vpn.example.com:51820" {
		t.Errorf("Endpoint = %q, want vpn.example.com:51820", cfg.Endpoint)
	}
	if cfg.AllowedIPs != "0.0.0.0/0, ::/0" {
		t.Errorf("AllowedIPs = %q, want 0.0.0.0/0, ::/0", cfg.AllowedIPs)
	}
	if cfg.Keepalive != "25" {
		t.Errorf("Keepalive = %q, want 25", cfg.Keepalive)
	}
}

func TestEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OC_VPN_PROFILES_DIR", tmpDir)

	// Create profile with endpoint
	dir := filepath.Join(tmpDir, "with-endpoint")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "wg.conf"), []byte(`[Interface]
PrivateKey = test=

[Peer]
PublicKey = peer=
Endpoint = 1.2.3.4:51820
AllowedIPs = 0.0.0.0/0
`), 0o644)

	ep, err := Endpoint("with-endpoint")
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if ep != "1.2.3.4:51820" {
		t.Errorf("Endpoint() = %q, want 1.2.3.4:51820", ep)
	}

	// Nonexistent profile
	if _, err := Endpoint("nonexistent"); err == nil {
		t.Error("Endpoint() should fail for nonexistent profile")
	}
}

func TestParseConfigMissingFile(t *testing.T) {
	if _, err := ParseConfig("/nonexistent/file.conf"); err == nil {
		t.Error("ParseConfig() should fail for missing file")
	}
}
