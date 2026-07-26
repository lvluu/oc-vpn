// Package tui provides terminal UI helpers for oc-vpn.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/lvluu/oc-vpn/internal/profiles"
	"github.com/lvluu/oc-vpn/internal/wireguard"
)

var reader = bufio.NewReader(os.Stdin)

var debug = os.Getenv("OCVPN_DEBUG") != ""

func dbg(msg string, args ...any) {
	if debug {
		fmt.Fprintf(os.Stderr, "[debug] "+msg+"\n", args...)
	}
}

// PickProfile shows an interactive profile picker using bubbletea.
func PickProfile() (string, error) {
	names := profiles.List()
	dbg("PickProfile: found %d profiles", len(names))
	if len(names) == 0 {
		return "", fmt.Errorf("no profiles — import one: oc-vpn import <file> -n <name>")
	}

	if len(names) == 1 {
		fmt.Printf("Selected profile: %s\n", names[0])
		return names[0], nil
	}

	return pickProfileBubbletea(names)
}

// Confirm prompts the user for yes/no.
func Confirm(msg string) bool {
	fmt.Printf("%s (y/N): ", msg)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// Input prompts for text input.
func Input(placeholder string) string {
	fmt.Printf("%s: ", placeholder)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// ListProfiles prints all profiles with status.
func ListProfiles() {
	names := profiles.List()

	if len(names) == 0 {
		fmt.Println("No profiles. Import one: oc-vpn import <config.conf> -n <name>")
		return
	}

	type row struct {
		name, status, endpoint, latency, transfer string
	}
	var rows []row
	for _, name := range names {
		status := "DOWN"
		endpoint, _ := profiles.Endpoint(name)
		latency := "-"
		transfer := "-"
		rows = append(rows, row{name, status, endpoint, latency, transfer})
	}

	w := []int{len("PROFILE"), len("STATUS"), len("ENDPOINT"), len("LATENCY"), len("TRANSFER")}
	for _, r := range rows {
		if len(r.name) > w[0] {
			w[0] = len(r.name)
		}
		if len(r.endpoint) > w[2] {
			w[2] = len(r.endpoint)
		}
		if len(r.latency) > w[3] {
			w[3] = len(r.latency)
		}
		if len(r.transfer) > w[4] {
			w[4] = len(r.transfer)
		}
	}
	sep := func(n int) string {
		s := ""
		for i := 0; i < n; i++ {
			s += "─"
		}
		return s
	}
	fmt.Printf("┌─%s─┬─%s─┬─%s─┬─%s─┬─%s─┐\n", sep(w[0]), sep(w[1]), sep(w[2]), sep(w[3]), sep(w[4]))
	fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │ %-*s │\n", w[0], "PROFILE", w[1], "STATUS", w[2], "ENDPOINT", w[3], "LATENCY", w[4], "TRANSFER")
	fmt.Printf("├─%s─┼─%s─┼─%s─┼─%s─┼─%s─┤\n", sep(w[0]), sep(w[1]), sep(w[2]), sep(w[3]), sep(w[4]))
	for _, r := range rows {
		fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │ %-*s │\n", w[0], r.name, w[1], r.status, w[2], r.endpoint, w[3], r.latency, w[4], r.transfer)
	}
	fmt.Printf("└─%s─┴─%s─┴─%s─┴─%s─┴─%s─┘\n", sep(w[0]), sep(w[1]), sep(w[2]), sep(w[3]), sep(w[4]))
}

// StatusDashboard shows detailed status for all profiles.
func StatusDashboard() {
	names := profiles.List()

	fmt.Print("\033[2J\033[H")
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║ oc-vpn status                                                   ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	if len(names) == 0 {
		fmt.Println("  No profiles configured.")
		return
	}

	for _, name := range names {
		statusStr := "DOWN"
		endpoint, _ := profiles.Endpoint(name)
		region := "-"
		handshake := "-"
		latency := "-"
		transfer := "-"

		if wireguard.IsUp(name) {
			statusStr = "UP"
			endpoint = wireguard.Endpoint(name)
			handshake = wireguard.HandshakeAgo(name)
			latency = wireguard.Latency(name)
			transfer = wireguard.Transfer(name)
		}

		fmt.Printf("  ┌─ %s [%s]\n", name, statusStr)
		fmt.Printf("  │ Endpoint:  %s\n", endpoint)
		fmt.Printf("  │ Region:    %s\n", region)
		fmt.Printf("  │ Handshake: %s\n", handshake)
		fmt.Printf("  │ Latency:   %s\n", latency)
		fmt.Printf("  │ Transfer:  %s\n", transfer)
		fmt.Println("  └──")
		fmt.Println()
	}
}

// IPCheck displays public IP info for a profile.
func IPCheck(name string) {
	ip := wireguard.PublicIP(name)
	if ip == "-" || ip == "" {
		fmt.Println("  Failed to get IP. Is the tunnel up?")
		return
	}

	info := wireguard.IPInfo(name)
	country := "-"
	city := "-"
	isp := "-"
	if v, ok := info["country"]; ok {
		country = v
	}
	if v, ok := info["city"]; ok {
		city = v
	}
	if v, ok := info["isp"]; ok {
		isp = v
	}

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║ Public IP (via %s)\n", name)
	fmt.Println("╠═══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ IP:       %-54s║\n", ip)
	fmt.Printf("║ Location: %-54s║\n", fmt.Sprintf("%s, %s", city, country))
	fmt.Printf("║ ISP:      %-54s║\n", isp)
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
}

// ImportWizard runs an interactive import flow.
func ImportWizard() error {
	fmt.Println()
	fmt.Println("Import WireGuard Config")
	fmt.Println()

	conf := Input("Config file path")
	if conf == "" {
		return nil
	}

	name := Input("Profile name (e.g. us-east)")
	if name == "" {
		return nil
	}

	dns := Input("DNS server (optional, Enter to skip)")

	var err error
	if dns != "" {
		err = profiles.Import(conf, name, dns)
	} else {
		err = profiles.Import(conf, name)
	}

	if err != nil {
		fmt.Printf("  ✗ Import failed: %v\n", err)
		return err
	}

	fmt.Printf("  ✓ Profile '%s' imported\n", name)
	return nil
}
