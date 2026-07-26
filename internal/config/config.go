// Package config manages global oc-vpn settings (default profile, preferences).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

const (
	CurrentVersion = 1
	Filename       = "config.json"
)

var configDirEnv = "OC_VPN_CONFIG_DIR"

type Preferences struct {
	CheckIPOnConnect *bool `json:"check_ip_on_connect,omitempty"`
	AutoConnect      *bool `json:"auto_connect,omitempty"`
	KeepaliveDefault *int  `json:"keepalive_default,omitempty"`
	MTUDefault       *int  `json:"mtu_default,omitempty"`
}

type Config struct {
	Preferences    Preferences `json:"preferences,omitempty"`
	DefaultProfile string      `json:"default_profile"`
	Version        int         `json:"version"`
}

func Dir() string {
	if d := os.Getenv(configDirEnv); d != "" {
		return d
	}
	if realUser := os.Getenv("SUDO_USER"); realUser != "" {
		if u, err := user.Lookup(realUser); err == nil {
			return filepath.Join(u.HomeDir, ".config", "oc-vpn")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "oc-vpn")
}

func Path() string {
	return filepath.Join(Dir(), Filename)
}

func ProfilesDir() string {
	if d := os.Getenv("OC_VPN_PROFILES_DIR"); d != "" {
		return d
	}
	return filepath.Join(Dir(), "profiles")
}

func Load() (*Config, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func Default() *Config {
	return &Config{
		Version: CurrentVersion,
	}
}

func (c *Config) Validate() error {
	if c.Version < 1 || c.Version > CurrentVersion {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	return nil
}

func (c *Config) Save() error {
	if err := c.Validate(); err != nil {
		return err
	}
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(Path(), data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func (c *Config) SetDefault(name string) error {
	if name == "" {
		c.DefaultProfile = ""
		return nil
	}
	c.DefaultProfile = name
	return c.Save()
}

func (c *Config) ClearDefault() {
	c.DefaultProfile = ""
}
