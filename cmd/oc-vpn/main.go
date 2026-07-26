package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lvluu/oc-vpn/internal/config"
	"github.com/lvluu/oc-vpn/internal/doctor"
	"github.com/lvluu/oc-vpn/internal/namespace"
	"github.com/lvluu/oc-vpn/internal/profiles"
	"github.com/lvluu/oc-vpn/internal/tui"
	"github.com/lvluu/oc-vpn/internal/wireguard"
	"github.com/lvluu/oc-vpn/internal/worktree"
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
		"up": true, "down": true, "shell": true, "import": true, "export": true, "remove": true, "worktree": true,
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
	case "config":
		cmdConfig(args)
	case "default":
		cmdDefault(args)
	case "worktree", "wt":
		cmdWorktree(args)
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
	fmt.Println(`oc-vpn — Isolated WireGuard VPN tunnels

Usage:
  oc-vpn <command> [options]

Commands:
  import <config.conf> -n <name>   Import a WireGuard config file
  list | ls                       List all profiles
  up [name]                       Connect to a profile (uses default if set)
  down [name]                     Disconnect a profile
  run [name] [cmd...]             Run a command in a tunnel
  shell [name]                    Open an interactive shell in a tunnel
  status                          Show status of all profiles
  ip [name]                       Check public IP for a profile
  export <name>                   Export a profile config to stdout
  remove <name>                   Delete a profile
  default [name]                  Show or set the default profile
  config [get|set <key> <val>]    View or modify global settings
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
  --down-on-exit                  Tear down tunnel when command/shell exits (run, shell)

Worktree commands:
  worktree list | ls              List profiles with linked worktrees
  worktree add <dir> -p <profile> Link a project directory to a profile
  worktree remove <dir>           Remove a project link

Examples:
  oc-vpn import config.conf -n us-east
  oc-vpn default us-east
  oc-vpn up
  oc-vpn run curl ifconfig.me
  oc-vpn run --down-on-exit us-east npm test
  oc-vpn shell
  oc-vpn shell --down-on-exit us-east
  oc-vpn status
  oc-vpn ip
  oc-vpn export us-east`)
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
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		cfg, err := config.Load()
		if err == nil && cfg.DefaultProfile != "" {
			if profiles.Exists(cfg.DefaultProfile) {
				name = cfg.DefaultProfile
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
	}

	fmt.Printf("Connecting to %s...", name)
	if err := wireguard.Up(name); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(" ✓")

	fmt.Print("Waiting for handshake...")
	if err := wireguard.WaitForHandshake(name, 15*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: %v\n", err)
	} else {
		fmt.Println(" ✓")
		fmt.Print("Checking connectivity...")
		publicIP, err := wireguard.CheckConnectivity(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nWarning: %v\n", err)
		} else {
			fmt.Printf(" ✓ (public IP: %s)\n", publicIP)
		}
	}
}

func cmdDown(args []string) {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		cfg, err := config.Load()
		if err == nil && cfg.DefaultProfile != "" {
			if profiles.Exists(cfg.DefaultProfile) {
				name = cfg.DefaultProfile
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
	}

	wireguard.Down(name)
	fmt.Printf("✓ Disconnected from %s\n", name)
}

func cmdRun(args []string) {
	requireRoot()

	name := ""
	var cmd []string
	downOnExit := false

loop:
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--down-on-exit":
			downOnExit = true
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
		cfg, err := config.Load()
		if err == nil && cfg.DefaultProfile != "" && profiles.Exists(cfg.DefaultProfile) {
			name = cfg.DefaultProfile
		}
	}
	if name == "" {
		groups, err := worktree.ByProfile()
		if err == nil && len(groups) > 0 {
			choices := worktree.ProfileChoices(groups)
			picked, pickErr := tui.PickGeneric("Select profile:", choices)
			if pickErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", pickErr)
				os.Exit(1)
			}
			// Extract profile name from display string (before the first space)
			name = picked
			if idx := strings.Index(name, " "); idx != -1 {
				name = name[:idx]
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
	}
	if len(cmd) == 0 {
		cmd = []string{"opencode"}
	}

	broughtUp := false

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nInterrupted.\n")
		if downOnExit && broughtUp {
			if promptConfirm(fmt.Sprintf("Tear down tunnel '%s'?", name)) {
				fmt.Fprintf(os.Stderr, "Tearing down %s...\n", name)
				wireguard.Down(name)
			} else {
				fmt.Fprintf(os.Stderr, "Tunnel %s left running. Use: oc-vpn down %s\n", name, name)
			}
		}
		os.Exit(0)
	}()

	if !wireguard.IsUp(name) {
		if err := wireguard.Up(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
			return
		}
		broughtUp = true
		if err := wireguard.WaitForHandshake(name, 15*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		} else if publicIP, err := wireguard.CheckConnectivity(name); err == nil {
			fmt.Fprintf(os.Stderr, "Tunnel is up (public IP: %s)\n", publicIP)
		}
	}

	if err := namespace.ExecAsUser(name, cmd...); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitCode = 1
	}

	if downOnExit && broughtUp {
		if promptConfirm(fmt.Sprintf("Tear down tunnel '%s'?", name)) {
			fmt.Fprintf(os.Stderr, "\nTearing down %s...\n", name)
			wireguard.Down(name)
		} else {
			fmt.Fprintf(os.Stderr, "\nTunnel %s left running. Use: oc-vpn down %s\n", name, name)
		}
	}
}

func promptConfirm(msg string) bool {
	fmt.Printf("%s [y/N]: ", msg)
	var input string
	_, _ = fmt.Scanln(&input)
	return input == "y" || input == "Y"
}

func cmdShell(args []string) {
	requireRoot()

	name := ""
	downOnExit := false

	var filtered []string
	for _, a := range args {
		if a == "--down-on-exit" {
			downOnExit = true
		} else {
			filtered = append(filtered, a)
		}
	}

	if len(filtered) > 0 {
		name = filtered[0]
	}
	if name == "" {
		cfg, err := config.Load()
		if err == nil && cfg.DefaultProfile != "" && profiles.Exists(cfg.DefaultProfile) {
			name = cfg.DefaultProfile
		}
	}
	if name == "" {
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

	// Handle Ctrl+C gracefully — only tear down if --down-on-exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\nExiting %s shell.\n", name)
		if downOnExit {
			wireguard.Down(name)
		}
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

	if downOnExit {
		wireguard.Down(name)
	}
}

func cmdStatus(_ []string) {
	tui.StatusDashboard()
}

func cmdIP(args []string) {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		cfg, err := config.Load()
		if err == nil && cfg.DefaultProfile != "" && profiles.Exists(cfg.DefaultProfile) {
			name = cfg.DefaultProfile
		}
	}
	if name == "" {
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

func cmdWorktree(args []string) {
	if len(args) == 0 {
		printWorktreeUsage()
		os.Exit(1)
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "list", "ls":
		cmdWorktreeList(subArgs)
	case "add":
		cmdWorktreeAdd(subArgs)
	case "remove", "rm":
		cmdWorktreeRemove(subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown worktree subcommand: %s\n", sub)
		printWorktreeUsage()
		os.Exit(1)
	}
}

func printWorktreeUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  worktree list                     List profiles with linked worktrees
  worktree add <dir> -p <profile>   Link a project directory to a profile
  worktree remove <dir>             Remove a project link`)
}

func cmdWorktreeList(_ []string) {
	groups, err := worktree.ByProfile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(groups) == 0 {
		fmt.Println("No profiles.")
		return
	}
	fmt.Println("Worktrees:")
	fmt.Print(worktree.FormatAll(groups))
}

func cmdWorktreeAdd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: oc-vpn worktree add <dir> -p <profile>")
		os.Exit(1)
	}

	projectDir := args[0]
	profile := ""

	for i := 1; i < len(args); i++ {
		if args[i] == "-p" || args[i] == "--profile" {
			if i+1 < len(args) {
				profile = args[i+1]
				i++
			}
		}
	}

	if profile == "" {
		fmt.Fprintln(os.Stderr, "Error: profile required (-p <name>)")
		os.Exit(1)
	}
	if !profiles.Exists(profile) {
		fmt.Fprintf(os.Stderr, "Error: profile '%s' not found\n", profile)
		os.Exit(1)
	}

	inv, err := worktree.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := inv.Add(projectDir, profile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := inv.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving worktrees: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Linked %s → %s\n", projectDir, profile)
}

func cmdWorktreeRemove(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: oc-vpn worktree remove <dir>")
		os.Exit(1)
	}

	projectDir := args[0]
	inv, err := worktree.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !inv.Remove(projectDir) {
		fmt.Fprintf(os.Stderr, "Error: no worktree found for %s\n", projectDir)
		os.Exit(1)
	}
	if err := inv.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving worktrees: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Removed worktree for %s\n", projectDir)
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

func cmdConfig(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 {
		fmt.Println("Settings:")
		fmt.Printf("  default_profile:   %s\n", orDash(cfg.DefaultProfile))
		fmt.Println("  preferences:")
		if cfg.Preferences.CheckIPOnConnect != nil {
			fmt.Printf("    check_ip_on_connect: %t\n", *cfg.Preferences.CheckIPOnConnect)
		}
		if cfg.Preferences.AutoConnect != nil {
			fmt.Printf("    auto_connect:        %t\n", *cfg.Preferences.AutoConnect)
		}
		if cfg.Preferences.KeepaliveDefault != nil {
			fmt.Printf("    keepalive_default:   %d\n", *cfg.Preferences.KeepaliveDefault)
		}
		if cfg.Preferences.MTUDefault != nil {
			fmt.Printf("    mtu_default:         %d\n", *cfg.Preferences.MTUDefault)
		}
		fmt.Printf("  config path: %s\n", config.Path())
		return
	}

	sub := args[0]
	switch sub {
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: oc-vpn config get <key>")
			os.Exit(1)
		}
		printConfigValue(cfg, args[1])
	case "set":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: oc-vpn config set <key> <value>")
			os.Exit(1)
		}
		setConfigValue(cfg, args[1], args[2])
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func printConfigValue(cfg *config.Config, key string) {
	switch key {
	case "default_profile":
		fmt.Println(cfg.DefaultProfile)
	case "check_ip_on_connect":
		if cfg.Preferences.CheckIPOnConnect != nil {
			fmt.Println(*cfg.Preferences.CheckIPOnConnect)
		} else {
			fmt.Println("true")
		}
	case "auto_connect":
		if cfg.Preferences.AutoConnect != nil {
			fmt.Println(*cfg.Preferences.AutoConnect)
		} else {
			fmt.Println("true")
		}
	case "keepalive_default":
		if cfg.Preferences.KeepaliveDefault != nil {
			fmt.Println(*cfg.Preferences.KeepaliveDefault)
		} else {
			fmt.Println("25")
		}
	case "mtu_default":
		if cfg.Preferences.MTUDefault != nil {
			fmt.Println(*cfg.Preferences.MTUDefault)
		} else {
			fmt.Println("1420")
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown config key: %s\n", key)
		os.Exit(1)
	}
}

func setConfigValue(cfg *config.Config, key, value string) {
	switch key {
	case "default_profile":
		if err := cfg.SetDefault(value); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "check_ip_on_connect":
		v, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid boolean: %s\n", value)
			os.Exit(1)
		}
		cfg.Preferences.CheckIPOnConnect = &v
	case "auto_connect":
		v, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid boolean: %s\n", value)
			os.Exit(1)
		}
		cfg.Preferences.AutoConnect = &v
	case "keepalive_default":
		v, err := strconv.Atoi(value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid integer: %s\n", value)
			os.Exit(1)
		}
		cfg.Preferences.KeepaliveDefault = &v
	case "mtu_default":
		v, err := strconv.Atoi(value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid integer: %s\n", value)
			os.Exit(1)
		}
		cfg.Preferences.MTUDefault = &v
	default:
		fmt.Fprintf(os.Stderr, "Unknown config key: %s\n", key)
		os.Exit(1)
	}
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ %s set to %s\n", key, value)
}

func cmdDefault(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 {
		if cfg.DefaultProfile == "" {
			fmt.Println("No default profile set.")
		} else {
			fmt.Printf("Default profile: %s\n", cfg.DefaultProfile)
		}
		return
	}

	name := args[0]
	if name == "--clear" {
		cfg.ClearDefault()
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Default profile cleared.")
		return
	}

	if !profiles.Exists(name) {
		fmt.Fprintf(os.Stderr, "Error: profile '%s' not found\n", name)
		os.Exit(1)
	}

	if err := cfg.SetDefault(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Default profile set to '%s'\n", name)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
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
