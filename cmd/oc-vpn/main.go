package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/lvluu/oc-vpn/internal/doctor"
	"github.com/lvluu/oc-vpn/internal/namespace"
	"github.com/lvluu/oc-vpn/internal/profiles"
	"github.com/lvluu/oc-vpn/internal/tui"
	"github.com/lvluu/oc-vpn/internal/wireguard"
)

var exitCode int

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Ensure root for commands that need it
	needRoot := map[string]bool{
		"up": true, "down": true, "shell": true, "import": true, "export": true, "remove": true,
	}
	if needRoot[cmd] && os.Getuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: command '%s' requires root privileges. Run with sudo.\n", cmd)
		os.Exit(1)
	}

	switch cmd {
	case "import", "add":
		cmdImport(args)
	case "list", "ls", "profiles":
		cmdList(args)
	case "up", "connect":
		cmdUp(args)
	case "down", "disconnect":
		cmdDown(args)
	case "run":
		cmdRun(args)
	case "shell":
		cmdShell(args)
	case "status":
		cmdStatus(args)
	case "cleanup":
		cmdCleanup(args)
	case "ip":
		cmdIP(args)
	case "export":
		cmdExport(args)
	case "remove", "rm", "delete":
		cmdRemove(args)
	case "doctor":
		cmdDoctor()
	case "version", "-v", "--version":
		fmt.Printf("oc-vpn %s %s (%s)\n%s/%s\n", version, commit, date, runtime.GOOS, runtime.GOARCH)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func printUsage() {
	fmt.Println(`oc-vpn — Run opencode through isolated WireGuard tunnels

Usage:
  oc-vpn <command> [options]

Commands:
  import <config.conf> -n <name>   Import a WireGuard config file
  list | ls                       List all profiles
  up <name>                       Connect to a profile
  down <name>                     Disconnect a profile
  run <name> [cmd...]             Run a command in a tunnel
  shell <name>                    Open an interactive shell in a tunnel
  status                          Show status of all profiles
  ip <name>                       Check public IP for a profile
  export <name>                   Export a profile config to stdout
  remove <name>                   Delete a profile
  doctor                          Check system requirements
  version                         Show version

Flags:
  -d, --dns <server>              Override DNS server (import)
  --keepalive <seconds>           Persistent keepalive (import)
  --mtu <size>                    Override MTU (import, default 1420)
  --fwmark <number>               Override firewall mark (import)
  --endpoint <host:port>          Override endpoint (import)
  --public-key <key>              Override public key (import)
  --allowed-ips <cidr>            Override allowed IPs (import)
  --private-key <key>             Override private key (import)
  --address <cidr>                Override interface address (import)
  -y, --yes                       Skip confirmation prompts

Examples:
  oc-vpn import config.conf -n us-east
  oc-vpn up us-east
  oc-vpn run us-east curl ifconfig.me
  oc-vpn shell us-east
  oc-vpn status
  oc-vpn ip us-east
  oc-vpn export us-east | wg-quick up -`)
}

func requireRoot() {
	if os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, "Error: this command requires root. Run with sudo.")
		os.Exit(1)
	}
}

func cmdImport(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: oc-vpn import <config.conf> -n <name>")
		os.Exit(1)
	}

	conf := args[0]
	name := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-n", "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		}
	}

	if name == "" {
		// Infer from filename
		base := conf
		if idx := strings.LastIndex(base, "/"); idx != -1 {
			base = base[idx+1:]
		}
		base = strings.TrimSuffix(base, ".conf")
		base = strings.TrimSuffix(base, ".wg")
		name = base
	}

	if name == "" {
		fmt.Fprintln(os.Stderr, "Error: profile name required (-n)")
		os.Exit(1)
	}

	if err := profiles.Import(conf, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Profile '%s' imported successfully\n", name)
}

func cmdList(args []string) {
	forceFlag := false
	for _, a := range args {
		if a == "-f" || a == "--force" {
			forceFlag = true
		}
	}

	names := profiles.List()
	if len(names) == 0 {
		if forceFlag {
			fmt.Println("[]")
		} else {
			fmt.Println("No profiles found. Import one: oc-vpn import <config.conf> -n <name>")
		}
		return
	}

	if forceFlag {
		for _, name := range names {
			fmt.Println(name)
		}
		return
	}

	tui.ListProfiles()
}

func cmdUp(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: oc-vpn up <name> | oc-vpn up (interactive picker)")
		os.Exit(1)
	}

	name := args[0]
	if name == "" {
		var err error
		name, err = tui.PickProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Connecting to %s...\n", name)
	if err := wireguard.Up(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if wireguard.IsUp(name) {
		fmt.Printf("✓ Connected to %s\n", name)
	} else {
		fmt.Println("Note: handshake may take a few seconds")
	}
}

func cmdDown(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: oc-vpn down <name> | oc-vpn down (interactive picker)")
		os.Exit(1)
	}

	name := args[0]
	if name == "" {
		var err error
		name, err = tui.PickProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	wireguard.Down(name)
	fmt.Printf("✓ Disconnected from %s\n", name)
}

func cmdRun(args []string) {
	requireRoot()

	name := ""
	var cmd []string
	keepAlive := false

	// Parse flags
loop:
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--keep", "-k":
			keepAlive = true
		default:
			if name == "" {
				name = args[i]
			} else {
				cmd = args[i:]
				break loop
			}
		}
	}

	if name == "" {
		var err error
		name, err = tui.PickProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(cmd) == 0 {
			cmd = []string{"opencode"}
		}
	} else if len(cmd) == 0 {
		cmd = []string{"opencode"}
	}

	// Auto-teardown on exit unless --keep
	defer func() {
		if !keepAlive {
			fmt.Fprintf(os.Stderr, "\nTearing down %s...\n", name)
			wireguard.Down(name)
		} else {
			fmt.Fprintf(os.Stderr, "\nTunnel %s still running. Use: oc-vpn down %s\n", name, name)
		}
	}()

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nInterrupted. Tearing down %s...\n", name)
		wireguard.Down(name)
		os.Exit(0)
	}()

	if !wireguard.IsUp(name) {
		if err := wireguard.Up(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
			return
		}
	}

	if err := namespace.ExecAsUser(name, cmd...); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitCode = 1
	}
}

func cmdShell(args []string) {
	requireRoot()

	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		var err error
		name, err = tui.PickProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	if !wireguard.IsUp(name) {
		if err := wireguard.Up(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Entering shell in %s namespace. Type 'exit' to leave.\n", name)

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\nExiting %s shell.\n", name)
		wireguard.Down(name)
		os.Exit(0)
	}()

	shell := "/bin/bash"
	if s := os.Getenv("SHELL"); s != "" {
		shell = s
	}

	if err := namespace.ExecAsUser(name, shell); err != nil {
		fmt.Fprintf(os.Stderr, "Shell error: %v\n", err)
		os.Exit(1)
	}

	wireguard.Down(name)
}

func cmdStatus(_ []string) {
	tui.StatusDashboard()
}

func cmdIP(args []string) {
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		var err error
		name, err = tui.PickProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	tui.IPCheck(name)
}

func cmdExport(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: oc-vpn export <name>")
		os.Exit(1)
	}

	name := args[0]
	p := profiles.Get(name)
	if p == nil {
		fmt.Fprintf(os.Stderr, "Error: profile '%s' not found\n", name)
		os.Exit(1)
	}

	cfg, err := profiles.ParseConfig(p.ConfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Private key: %s\n", cfg.PrivateKey)
	fmt.Printf("Public key:  %s\n", cfg.PublicKey)
	fmt.Printf("Endpoint:    %s\n", cfg.Endpoint)
	fmt.Printf("Allowed IPs: %s\n", cfg.AllowedIPs)
	if cfg.DNS != "" {
		fmt.Printf("DNS:         %s\n", cfg.DNS)
	}
	if cfg.Address != "" {
		fmt.Printf("Address:     %s\n", cfg.Address)
	}
}

func cmdRemove(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: oc-vpn remove <name>")
		os.Exit(1)
	}

	name := args[0]
	p := profiles.Get(name)
	if p == nil {
		fmt.Fprintf(os.Stderr, "Error: profile '%s' not found\n", name)
		os.Exit(1)
	}

	if wireguard.IsUp(name) {
		wireguard.Down(name)
	}

	if err := profiles.Remove(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Profile '%s' removed\n", name)
}

func cmdDoctor() {
	checks := doctor.Run()

	fmt.Println()
	fmt.Println("System checks:")
	fmt.Println()

	hasIssue := false
	for _, c := range checks {
		var icon, color string
		switch c.Status {
		case "ok":
			icon = "✓"
			color = "\033[32m" // green
		case "warn":
			icon = "!"
			color = "\033[33m" // yellow
			hasIssue = true
		case "fail":
			icon = "✗"
			color = "\033[31m" // red
			hasIssue = true
		}
		reset := "\033[0m"
		fmt.Printf("  %s%s %s%s — %s\n", color, icon, reset, c.Name, c.Message)
	}

	fmt.Println()
	if hasIssue {
		fmt.Println("Some issues found. Fix them above for full functionality.")
	} else {
		fmt.Println("✓ All checks passed.")
	}
}

func cmdCleanup(args []string) {
	requireRoot()

	force := false
	for _, a := range args {
		if a == "-f" || a == "--force" {
			force = true
		}
	}

	names := profiles.List()
	profileSet := make(map[string]bool)
	for _, n := range names {
		profileSet[n] = true
	}

	// Find all ocvpn-* namespaces
	out, err := exec.Command(namespace.IPPath(), "netns", "list").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing namespaces: %v\n", err)
		os.Exit(1)
	}

	var orphans []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		ns := fields[0]
		if !strings.HasPrefix(ns, "ocvpn-") {
			continue
		}
		profile := strings.TrimPrefix(ns, "ocvpn-")
		if !profileSet[profile] {
			orphans = append(orphans, ns)
		}
	}

	if len(orphans) == 0 {
		fmt.Println("✓ No orphaned namespaces found.")
		return
	}

	fmt.Printf("Found %d orphaned namespace(s):\n", len(orphans))
	for _, ns := range orphans {
		fmt.Printf("  - %s\n", ns)
	}

	if !force {
		fmt.Print("\nTear them all down? [y/N]: ")
		var input string
		_, _ = fmt.Scanln(&input)
		if input != "y" && input != "Y" {
			fmt.Println("Aborted.")
			return
		}
	}

	for _, ns := range orphans {
		profile := strings.TrimPrefix(ns, "ocvpn-")
		fmt.Printf("  Destroying %s...\n", ns)
		wireguard.Down(profile)
	}

	fmt.Printf("✓ Cleaned up %d orphaned namespace(s).\n", len(orphans))
}
