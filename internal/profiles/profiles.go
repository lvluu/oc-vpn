// Package profiles handles WireGuard profile CRUD, config parsing, and validation.
package profiles

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Profile represents a stored WireGuard profile.
type Profile struct {
	Name     string
	Dir      string
	ConfPath string
	MetaPath string
}

// Meta is the metadata stored alongside a profile.
type Meta struct {
	Name       string `json:"name"`
	ImportedAt string `json:"imported_at"`
	Source     string `json:"source"`
	Endpoint   string `json:"endpoint"`
	Address    string `json:"address,omitempty"`
}

// WGConfig holds parsed WireGuard configuration fields.
type WGConfig struct {
	PrivateKey string
	Address    string
	DNS        string
	MTU        string
	FwMark     string
	Endpoint   string
	PublicKey  string
	AllowedIPs string
	Keepalive  string
}

// Dir returns the profiles directory, preferring $OC_VPN_PROFILES_DIR,
// falling back to the real user's ~/.config/oc-vpn/profiles.
func Dir() string {
	if d := os.Getenv("OC_VPN_PROFILES_DIR"); d != "" {
		return d
	}

	// When running via sudo, $HOME points to root's home.
	// Use SUDO_USER's home instead.
	if realUser := os.Getenv("SUDO_USER"); realUser != "" {
		if u, err := user.Lookup(realUser); err == nil {
			return filepath.Join(u.HomeDir, ".config", "oc-vpn", "profiles")
		}
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "oc-vpn", "profiles")
}

// ValidateName checks that a profile name contains only safe characters.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("profile name cannot contain '..'")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("profile name cannot contain '/'")
	}
	if strings.ContainsAny(name, " \t\n") {
		return fmt.Errorf("profile name cannot contain whitespace")
	}
	if !validName.MatchString(name) {
		return fmt.Errorf("profile name can only contain letters, numbers, dots, hyphens, underscores")
	}
	return nil
}

// Get returns a Profile by name. Returns nil if not found.
func Get(name string) *Profile {
	dir := filepath.Join(Dir(), name)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return &Profile{
			Name:     name,
			Dir:      dir,
			ConfPath: filepath.Join(dir, "wg.conf"),
			MetaPath: filepath.Join(dir, "meta.json"),
		}
	}
	return nil
}

// Exists checks if a profile directory exists.
func Exists(name string) bool {
	return Get(name) != nil
}

// List returns all profile names.
func List() []string {
	base := Dir()
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// Count returns the number of profiles.
func Count() int {
	return len(List())
}

// Import copies a WireGuard config into a profile, stripping wg-quick-only
// fields and creating metadata.
func Import(confPath, name string, extraFlags ...string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", confPath)
	}

	dir := filepath.Join(Dir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating profile dir: %w", err)
	}

	confDest := filepath.Join(dir, "wg.conf")
	if err := copyFile(confPath, confDest); err != nil {
		return fmt.Errorf("copying config: %w", err)
	}

	// Parse address BEFORE stripping (Address is needed for wg0 IP assignment)
	preCfg, err := ParseConfig(confDest)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	// Strip wg-quick-only fields
	if stripErr := stripWGQuickFields(confDest); stripErr != nil {
		return fmt.Errorf("stripping config: %w", stripErr)
	}

	// Parse endpoint from cleaned config
	cfg, err := ParseConfig(confDest)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	// Write metadata
	src, _ := filepath.Abs(confPath)
	meta := Meta{
		Name:       name,
		ImportedAt: time.Now().Format(time.RFC3339),
		Source:     src,
		Endpoint:   cfg.Endpoint,
		Address:    preCfg.Address,
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaBytes, 0o644); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}

// Remove deletes a profile directory.
func Remove(name string) error {
	p := Get(name)
	if p == nil {
		return fmt.Errorf("profile '%s' not found", name)
	}
	return os.RemoveAll(p.Dir)
}

// Endpoint reads the Endpoint from a profile's config.
func Endpoint(name string) (string, error) {
	p := Get(name)
	if p == nil {
		return "", fmt.Errorf("profile '%s' not found", name)
	}
	cfg, err := ParseConfig(p.ConfPath)
	if err != nil {
		return "-", err
	}
	if cfg.Endpoint == "" {
		return "-", nil
	}
	return cfg.Endpoint, nil
}

// ReadMeta reads the meta.json for a profile.
func ReadMeta(name string) (*Meta, error) {
	p := Get(name)
	if p == nil {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}
	data, err := os.ReadFile(p.MetaPath)
	if err != nil {
		return nil, err
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// ParseConfig parses a WireGuard config file into WGConfig.
// It handles both [Interface] and [Peer] sections.
func ParseConfig(path string) (*WGConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseConfigBytes(data), nil
}

func parseConfigBytes(data []byte) *WGConfig {
	cfg := &WGConfig{}
	lines := strings.Split(string(data), "\n")
	inPeer := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.EqualFold(line, "[Peer]") {
			inPeer = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inPeer = false
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if inPeer {
			switch key {
			case "PublicKey":
				if cfg.PublicKey == "" {
					cfg.PublicKey = val
				}
			case "AllowedIPs":
				if cfg.AllowedIPs == "" {
					cfg.AllowedIPs = val
				}
			case "Endpoint":
				if cfg.Endpoint == "" {
					cfg.Endpoint = val
				}
			case "PersistentKeepalive":
				if cfg.Keepalive == "" {
					cfg.Keepalive = val
				}
			}
		} else {
			switch key {
			case "PrivateKey":
				cfg.PrivateKey = val
			case "Address":
				cfg.Address = val
			case "DNS":
				cfg.DNS = val
			case "MTU":
				cfg.MTU = val
			case "FwMark":
				cfg.FwMark = val
			}
		}
	}
	return cfg
}

// stripWGQuickFields removes wg-quick-only fields from a config file.
func stripWGQuickFields(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fields := []string{
		"Address", "DNS", "MTU", "Table", "SaveConfig",
		"PreUp", "PreDown", "PostUp", "PostDown",
	}

	lines := strings.Split(string(data), "\n")
	var kept []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		skip := false
		for _, f := range fields {
			if strings.HasPrefix(trimmed, f+"=") || strings.HasPrefix(trimmed, f+" =") {
				skip = true
				break
			}
		}
		if !skip {
			kept = append(kept, line)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
