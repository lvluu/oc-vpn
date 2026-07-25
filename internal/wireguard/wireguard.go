// Package wireguard manages WireGuard interfaces inside network namespaces.
package wireguard

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lvluu/oc-vpn/internal/namespace"
	"github.com/lvluu/oc-vpn/internal/profiles"
)

// Up brings up a WireGuard tunnel in a namespace.
func Up(name string) error {
	p := profiles.Get(name)
	if p == nil {
		return fmt.Errorf("profile '%s' not found", name)
	}

	cfg, err := profiles.ParseConfig(p.ConfPath)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	if cfg.PrivateKey == "" || cfg.Endpoint == "" || cfg.PublicKey == "" {
		return fmt.Errorf("config missing required fields (PrivateKey, Endpoint, PublicKey)")
	}

	// Create namespace if needed
	if !namespace.Exists(name) {
		if createErr := namespace.Create(name); createErr != nil {
			return fmt.Errorf("creating namespace: %w", createErr)
		}
	}

	// Check if interface already exists
	if out, _ := namespace.Exec(name, "wg", "show", "interfaces"); strings.Contains(out, "wg0") {
		return nil
	}

	// Create WireGuard interface
	if _, execErr := namespace.Exec(name, "ip", "link", "add", "dev", "wg0", "type", "wireguard"); execErr != nil {
		return fmt.Errorf("creating wg0: %w", execErr)
	}

	// Build clean config for wg setconf
	tmpconf, err := os.CreateTemp("", "ocvpn-wg-*.conf")
	if err != nil {
		return fmt.Errorf("creating temp config: %w", err)
	}
	defer func() { _ = os.Remove(tmpconf.Name()) }()

	var sb strings.Builder
	fmt.Fprintf(&sb, "[Interface]\nPrivateKey = %s\n", cfg.PrivateKey)
	if cfg.MTU != "" {
		fmt.Fprintf(&sb, "MTU = %s\n", cfg.MTU)
	}
	if cfg.FwMark != "" {
		fmt.Fprintf(&sb, "FwMark = %s\n", cfg.FwMark)
	}
	fmt.Fprintf(&sb, "[Peer]\nPublicKey = %s\n", cfg.PublicKey)
	if cfg.Endpoint != "" {
		fmt.Fprintf(&sb, "Endpoint = %s\n", cfg.Endpoint)
	}
	if cfg.AllowedIPs != "" {
		fmt.Fprintf(&sb, "AllowedIPs = %s\n", cfg.AllowedIPs)
	}
	if cfg.Keepalive != "" {
		fmt.Fprintf(&sb, "PersistentKeepalive = %s\n", cfg.Keepalive)
	}

	if _, err := tmpconf.WriteString(sb.String()); err != nil {
		_ = tmpconf.Close()
		return err
	}
	_ = tmpconf.Close()

	// Apply config
	if _, err := namespace.Exec(name, "wg", "setconf", "wg0", tmpconf.Name()); err != nil {
		return fmt.Errorf("setting wg config: %w", err)
	}

	// Assign IP address
	// Address may have been stripped from wg.conf during import;
	// fall back to meta.json where it's saved separately.
	address := cfg.Address
	if address == "" {
		if meta, metaErr := profiles.ReadMeta(name); metaErr == nil {
			address = meta.Address
		}
	}
	if address != "" {
		_, _ = namespace.Exec(name, "ip", "addr", "add", address, "dev", "wg0")
	}

	mtu := "1420"
	if cfg.MTU != "" {
		mtu = cfg.MTU
	}
	if _, err := namespace.Exec(name, "ip", "link", "set", "mtu", mtu, "up", "dev", "wg0"); err != nil {
		return fmt.Errorf("bringing up wg0: %w", err)
	}

	// Route management
	endpointHost := extractIPv4(cfg.Endpoint)
	if endpointHost != "" {
		_, _ = namespace.Exec(name, "ip", "route", "add", endpointHost+"/32", "via", namespace.Gateway(name))
	}

	// Replace default route through wg0
	_, _ = namespace.Exec(name, "ip", "route", "replace", "default", "dev", "wg0")

	// DNS setup — write directly inside the namespace
	dnsServers := cfg.DNS
	if dnsServers == "" {
		dnsServers = "1.1.1.1, 8.8.8.8"
	}
	var dnsContent strings.Builder
	for _, ns := range strings.Split(dnsServers, ",") {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			fmt.Fprintf(&dnsContent, "nameserver %s\n", ns)
		}
	}

	// Write resolv.conf via /etc/netns/<ns>/resolv.conf (kernel applies it)
	nsDir := "/etc/netns/" + namespace.Name(name)
	_ = os.MkdirAll(nsDir, 0o755)
	_ = os.WriteFile(nsDir+"/resolv.conf", []byte(dnsContent.String()), 0o644)

	// Also write directly into the namespace's /etc/resolv.conf
	_, _ = namespace.Exec(name, "bash", "-c",
		fmt.Sprintf("echo '%s' > /etc/resolv.conf", strings.TrimSpace(dnsContent.String())))

	return nil
}

// Down tears down a WireGuard tunnel.
func Down(name string) {
	if !namespace.Exists(name) {
		return
	}
	namespace.Destroy(name)
}

// IsUp checks if WireGuard is running in a namespace.
func IsUp(name string) bool {
	if !namespace.Exists(name) {
		return false
	}
	out, _ := namespace.Exec(name, "wg", "show", "interfaces")
	return strings.Contains(out, "wg0")
}

// Status returns the tunnel status: "up", "stale", "no-handshake", or "down".
func Status(name string) string {
	if !IsUp(name) {
		return "down"
	}

	out, _ := namespace.Exec(name, "wg", "show", "wg0", "latest-handshakes")
	handshake := parseHandshake(out)
	now := time.Now().Unix()
	diff := now - handshake

	if diff > 180 {
		return "stale"
	}
	if handshake == 0 {
		return "no-handshake"
	}
	return "up"
}

// HandshakeAgo returns a human-readable string of time since last handshake.
func HandshakeAgo(name string) string {
	if !IsUp(name) {
		return "-"
	}

	out, _ := namespace.Exec(name, "wg", "show", "wg0", "latest-handshakes")
	handshake := parseHandshake(out)
	if handshake == 0 {
		return "never"
	}

	diff := time.Now().Unix() - handshake
	switch {
	case diff < 60:
		return fmt.Sprintf("%ds ago", diff)
	case diff < 3600:
		return fmt.Sprintf("%dm ago", diff/60)
	default:
		return fmt.Sprintf("%dh ago", diff/3600)
	}
}

// Transfer returns a human-readable RX/TX string.
func Transfer(name string) string {
	if !IsUp(name) {
		return "- / -"
	}

	rx, _ := namespace.Exec(name, "cat", "/sys/class/net/wg0/statistics/rx_bytes")
	tx, _ := namespace.Exec(name, "cat", "/sys/class/net/wg0/statistics/tx_bytes")

	return fmt.Sprintf("%s ↑ / %s ↓", formatBytes(rx), formatBytes(tx))
}

// Endpoint returns the WireGuard endpoint.
func Endpoint(name string) string {
	if !IsUp(name) {
		return "-"
	}
	out, _ := namespace.Exec(name, "wg", "show", "wg0", "endpoints")
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return fields[1]
		}
	}
	return "-"
}

// Latency pings the endpoint and returns RTT.
func Latency(name string) string {
	if !IsUp(name) {
		return "-"
	}
	ep := Endpoint(name)
	host := extractIPv4(ep)
	if host == "" {
		return "-"
	}

	out, err := namespace.Exec(name, "ping", "-c", "1", "-W", "3", host)
	if err != nil {
		return "?ms"
	}

	for _, line := range strings.Split(out, "\n") {
		if idx := strings.Index(line, "time="); idx != -1 {
			timeStr := line[idx+5:]
			if end := strings.Index(timeStr, " "); end != -1 {
				timeStr = timeStr[:end]
			}
			return timeStr + "ms"
		}
	}
	return "?ms"
}

// PublicIP queries the public IP through the tunnel.
func PublicIP(name string) string {
	if !IsUp(name) {
		return "-"
	}
	out, err := namespace.Exec(name, "curl", "-s", "--max-time", "5", "ifconfig.me")
	if err != nil {
		return "-"
	}
	return strings.TrimSpace(out)
}

// IPInfo queries location info through the tunnel.
func IPInfo(name string) map[string]string {
	if !IsUp(name) {
		return nil
	}
	out, _ := namespace.Exec(name, "curl", "-s", "--max-time", "5", "http://ip-api.com/json")
	result := make(map[string]string)

	// Simple JSON parsing without external deps
	for _, key := range []string{"country", "city", "isp"} {
		search := fmt.Sprintf(`%q:`, key)
		if idx := strings.Index(out, search); idx != -1 {
			val := out[idx+len(search):]
			if end := strings.Index(val, `"`); end != -1 {
				result[key] = val[:end]
			}
		}
	}
	return result
}

func extractIPv4(endpoint string) string {
	host := endpoint
	if idx := strings.LastIndex(endpoint, ":"); idx != -1 {
		host = endpoint[:idx]
	}
	host = strings.Trim(host, "[]")

	parts := strings.Split(host, ".")
	if len(parts) != 4 {
		return ""
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return ""
		}
	}
	return host
}

func parseHandshake(output string) int64 {
	fields := strings.Fields(output)
	for _, f := range fields {
		if v, err := strconv.ParseInt(f, 10, 64); err == nil && v > 0 {
			return v
		}
	}
	return 0
}

func formatBytes(s string) string {
	b, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return "0B"
	}
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fGiB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fMiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fKiB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
