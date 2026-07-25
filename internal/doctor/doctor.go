// Package doctor checks system requirements for oc-vpn.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/lvluu/oc-vpn/internal/profiles"
)

// Check represents a single system check result.
type Check struct {
	Name    string
	Status  string // "ok", "warn", "fail"
	Message string
}

// Run performs all system checks and returns results.
func Run() []Check {
	var checks []Check

	// WireGuard tools
	if path, err := exec.LookPath("wg"); err == nil {
		out, _ := exec.Command(path, "--version").CombinedOutput()
		checks = append(checks, Check{"wg", "ok", strings.TrimSpace(string(out))})
	} else {
		checks = append(checks, Check{"wg", "fail", "not found — install wireguard-tools"})
	}

	// WireGuard kernel module
	if runtime.GOOS == "linux" {
		if out, err := exec.Command("modprobe", "wireguard").CombinedOutput(); err == nil || strings.Contains(string(out), "") {
			checks = append(checks, Check{"wireguard module", "ok", "loaded"})
		} else if lsmod, _ := exec.Command("lsmod").CombinedOutput(); strings.Contains(string(lsmod), "wireguard") {
			checks = append(checks, Check{"wireguard module", "ok", "loaded"})
		} else {
			checks = append(checks, Check{"wireguard module", "warn", "not loaded (may load on demand)"})
		}
	}

	// Network namespaces
	if os.Getuid() == 0 {
		if err := exec.Command("ip", "netns", "add", "_ocvpn_test").Run(); err == nil {
			_ = exec.Command("ip", "netns", "del", "_ocvpn_test").Run()
			checks = append(checks, Check{"network namespaces", "ok", "supported"})
		} else {
			checks = append(checks, Check{"network namespaces", "fail", "not available"})
		}
	} else {
		if _, err := exec.LookPath("ip"); err == nil {
			checks = append(checks, Check{"network namespaces", "ok", "available (needs sudo)"})
		} else {
			checks = append(checks, Check{"network namespaces", "fail", "ip command not found"})
		}
	}

	// resolvconf
	if _, err := exec.LookPath("resolvconf"); err == nil {
		checks = append(checks, Check{"resolvconf", "ok", "available"})
	} else {
		checks = append(checks, Check{"resolvconf", "warn", "not found — DNS uses /etc/resolv.conf fallback"})
	}

	// gum
	if _, err := exec.LookPath("gum"); err == nil {
		out, _ := exec.Command("gum", "--version").CombinedOutput()
		checks = append(checks, Check{"gum", "ok", strings.TrimSpace(string(out))})
	} else {
		checks = append(checks, Check{"gum", "fail", "not found — install from github.com/charmbracelet/gum"})
	}

	// curl
	if _, err := exec.LookPath("curl"); err == nil {
		checks = append(checks, Check{"curl", "ok", "available"})
	} else {
		checks = append(checks, Check{"curl", "warn", "not found — IP checks will not work"})
	}

	// Root
	if os.Getuid() == 0 {
		checks = append(checks, Check{"root privileges", "ok", "yes"})
	} else {
		checks = append(checks, Check{"root privileges", "warn", "no — run with sudo"})
	}

	// Profiles dir
	dir := profiles.Dir()
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		count := profiles.Count()
		checks = append(checks, Check{"profiles dir", "ok", fmt.Sprintf("%s (%d profiles)", dir, count)})
	} else {
		checks = append(checks, Check{"profiles dir", "warn", fmt.Sprintf("%s (will be created on first import)", dir)})
	}

	return checks
}

// Errors returns the number of failed checks.
func Errors(checks []Check) int {
	n := 0
	for _, c := range checks {
		if c.Status == "fail" {
			n++
		}
	}
	return n
}
