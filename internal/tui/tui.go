// Package tui provides terminal UI helpers for oc-vpn.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/lvluu/oc-vpn/internal/profiles"
	"github.com/lvluu/oc-vpn/internal/wireguard"
	"golang.org/x/term"
)

var reader = bufio.NewReader(os.Stdin)

var debug = os.Getenv("OCVPN_DEBUG") != ""

func dbg(msg string, args ...any) {
	if debug {
		fmt.Fprintf(os.Stderr, "[debug] "+msg+"\n", args...)
	}
}

// PickProfile shows an interactive profile picker.
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

	return pickProfileArrow(names)
}

func pickProfileArrow(names []string) (string, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return pickProfileNumbered(names)
	}
	defer func() {
		// Flush the kernel input queue so stale escape sequences
		// don't leak into the calling TUI (e.g. opencode).
		const TIOCFLUSH = 0x5410
		const TCIFLUSH = 0
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), TIOCFLUSH, TCIFLUSH)
		term.Restore(fd, oldState)
		fmt.Print("\033[?25h") // ensure cursor is visible
	}()

	selected := 0

	for {
		fmt.Print("\033[2J\033[H")
		fmt.Println()
		fmt.Println("  \033[1mSelect profile:\033[0m")
		fmt.Println()

		for i, name := range names {
			if i == selected {
				fmt.Printf("  \033[36m▸ %s\033[0m\n", name)
			} else {
				fmt.Printf("    %s\n", name)
			}
		}

		fmt.Println()
		fmt.Println("  \033[90m↑/↓ navigate   ↵ select   q cancel\033[0m")

		buf := make([]byte, 16)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", fmt.Errorf("reading key: %w", err)
		}

		if n == 1 {
			switch buf[0] {
			case 13, 10:
				fmt.Print("\033[2J\033[H")
				fmt.Printf("  \033[32m✓\033[0m %s\n", names[selected])
				return names[selected], nil
			case 3, 113: // Ctrl+C, q
				fmt.Print("\033[2J\033[H")
				return "", fmt.Errorf("selection cancelled")
			case 27: // bare ESC — could be start of arrow sequence, try reading more
				if n2, err2 := os.Stdin.Read(buf[1:]); err2 == nil && n2 >= 2 && buf[1] == 91 {
					// it's an escape sequence — fall through to sequence handler below
					n = 1 + n2
					goto sequence
				}
				// bare ESC pressed
				fmt.Print("\033[2J\033[H")
				return "", fmt.Errorf("selection cancelled")
			case 106: // j
				if selected < len(names)-1 {
					selected++
				}
			case 107: // k
				if selected > 0 {
					selected--
				}
			}
		}

	sequence:
		if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65:
				if selected > 0 {
					selected--
				}
			case 66:
				if selected < len(names)-1 {
					selected++
				}
			}
		}
	}
}

func pickProfileNumbered(names []string) (string, error) {
	fmt.Println()
	fmt.Println("Select profile:")
	fmt.Println()

	for i, name := range names {
		fmt.Printf("  \033[36m%d\033[0m) %s\n", i+1, name)
	}

	fmt.Println()
	fmt.Print("  \033[90m>\033[0m ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return "", fmt.Errorf("no selection")
	}

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(names) {
		return "", fmt.Errorf("invalid selection: %s", input)
	}

	fmt.Printf("  \033[32m✓\033[0m %s\n", names[idx-1])
	return names[idx-1], nil
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
