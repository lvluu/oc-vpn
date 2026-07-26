// Package namespace manages Linux network namespaces for isolated WireGuard tunnels.
package namespace

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/lvluu/oc-vpn/internal/config"
)

func Prefix() string {
	if config.IsDev() {
		return "ocvpn-dev"
	}
	return "ocvpn"
}

// Subnet returns the unique /24 subnet details for a profile.
// Host (veth) gets .1, namespace gets .2, gateway for namespace is host.
func Subnet(profile string) (subnet, hostIP, nsIP, gateway string) {
	ns := Name(profile)
	hash := md5.Sum([]byte(ns))
	n := int(hash[0])%254 + 1 // 1–254
	subnet = fmt.Sprintf("10.200.%d.0/24", n)
	hostIP = fmt.Sprintf("10.200.%d.1/24", n)
	nsIP = fmt.Sprintf("10.200.%d.2/24", n)
	gateway = fmt.Sprintf("10.200.%d.1", n)
	return
}

// SubnetCIDR returns just the CIDR for iptables rules.
func SubnetCIDR(profile string) string {
	s, _, _, _ := Subnet(profile)
	return s
}

// Gateway returns the host gateway IP for a profile.
func Gateway(profile string) string {
	_, _, _, gw := Subnet(profile)
	return gw
}

// HostIP returns the host veth IP with CIDR for a profile.
func HostIP(profile string) string {
	_, h, _, _ := Subnet(profile)
	return h
}

// NamespaceIP returns the namespace-side IP with CIDR for a profile.
func NamespaceIP(profile string) string {
	_, _, n, _ := Subnet(profile)
	return n
}

// Port returns a deterministic port (51820 + profile hash mod 1000) for future use.
func Port(profile string) int {
	hash := md5.Sum([]byte(Name(profile)))
	return 51820 + int(hash[0])%1000
}

// FormatPort returns port as string.
func FormatPort(profile string) string {
	return strconv.Itoa(Port(profile))
}

// Name returns the namespace name for a profile.
func Name(profile string) string {
	return fmt.Sprintf("%s-%s", Prefix(), profile)
}

// Exists checks if a namespace exists.
func Exists(profile string) bool {
	ns := Name(profile)
	out, err := exec.Command(IPPath(), "netns", "list").CombinedOutput()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == ns {
			return true
		}
	}
	return false
}

// Create sets up a network namespace with a veth pair and NAT.
func Create(profile string) error {
	ns := Name(profile)
	if Exists(profile) {
		return fmt.Errorf("namespace %s already exists", ns)
	}

	_, hostIP, nsIP, gw := Subnet(profile)
	cidr := SubnetCIDR(profile)

	cmds := [][]string{
		{"ip", "netns", "add", ns},
		{"ip", "netns", "exec", ns, "ip", "link", "set", "lo", "up"},
	}

	for _, c := range cmds {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s (%w)", strings.Join(c, " "), strings.TrimSpace(string(out)), err)
		}
	}

	// Create veth pair with short names (Linux 15-char limit)
	hash := md5.Sum([]byte(ns))
	hostVeth := fmt.Sprintf("v%x", hash[:2])
	nsVeth := fmt.Sprintf("v%xn", hash[:2])

	// Clean up stale veth (best-effort)
	_, _ = exec.Command(IPPath(), "link", "del", hostVeth).CombinedOutput()

	vethCmds := [][]string{
		{"ip", "link", "add", hostVeth, "type", "veth", "peer", "name", nsVeth},
		{"ip", "link", "set", nsVeth, "netns", ns},
		{"ip", "addr", "flush", "dev", hostVeth},
		{"ip", "addr", "add", hostIP, "dev", hostVeth},
		{"ip", "link", "set", hostVeth, "up"},
		{"ip", "netns", "exec", ns, "ip", "addr", "flush", "dev", nsVeth},
		{"ip", "netns", "exec", ns, "ip", "addr", "add", nsIP, "dev", nsVeth},
		{"ip", "netns", "exec", ns, "ip", "link", "set", nsVeth, "up"},
		{"ip", "netns", "exec", ns, "ip", "route", "add", "default", "via", gw},
	}

	for _, c := range vethCmds {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s (%w)", strings.Join(c, " "), strings.TrimSpace(string(out)), err)
		}
	}

	// Enable IP forwarding (best-effort)
	_, _ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").CombinedOutput()

	// Get default route device
	defaultDev, _ := getDefaultDev()
	if defaultDev != "" {
		// NAT so namespace traffic can reach the physical network
		iptArgs := []string{
			"-t", "nat", "-A", "POSTROUTING",
			"-s", cidr, "-o", defaultDev,
			"-j", "MASQUERADE",
			"-m", "comment", "--comment", "oc-vpn-" + ns,
		}
		_, _ = exec.Command("iptables", iptArgs...).CombinedOutput()
	}

	// Allow forwarding through UFW (best-effort)
	_, _ = exec.Command("ufw", "route", "allow", "from", cidr).CombinedOutput()
	_, _ = exec.Command("ufw", "route", "allow", "to", cidr).CombinedOutput()

	return nil
}

// Destroy tears down a namespace and cleans up resources.
func Destroy(profile string) {
	ns := Name(profile)
	cidr := SubnetCIDR(profile)

	// Disconnect WireGuard (best-effort)
	_, _ = exec.Command(IPPath(), "netns", "exec", ns, "ip", "link", "del", "wg0").CombinedOutput()

	// Remove veth pair
	hash := md5.Sum([]byte(ns))
	hostVeth := fmt.Sprintf("v%x", hash[:2])
	_, _ = exec.Command(IPPath(), "link", "del", hostVeth).CombinedOutput()

	// Remove iptables rule
	defaultDev, _ := getDefaultDev()
	if defaultDev != "" {
		iptArgs := []string{
			"-t", "nat", "-D", "POSTROUTING",
			"-s", cidr, "-o", defaultDev,
			"-j", "MASQUERADE",
			"-m", "comment", "--comment", "oc-vpn-" + ns,
		}
		_, _ = exec.Command("iptables", iptArgs...).CombinedOutput()
	}

	// Delete namespace
	_, _ = exec.Command(IPPath(), "netns", "del", ns).CombinedOutput()

	// Clean up resolv.conf
	_, _ = exec.Command("rm", "-f", "/etc/netns/"+ns+"/resolv.conf").CombinedOutput()
	_, _ = exec.Command("rmdir", "/etc/netns/"+ns).CombinedOutput()
}

// ExecRaw runs an interactive command inside a namespace (stdin/stdout/stderr inherited).
func ExecRaw(profile string, cmd ...string) error {
	ns := Name(profile)
	args := append([]string{"netns", "exec", ns}, cmd...)
	c := exec.Command(IPPath(), args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// ExecAsUser runs a command inside a namespace as the real user (SUDO_USER),
// with their full environment and home directory.
func ExecAsUser(profile string, cmd ...string) error {
	ns := Name(profile)
	runUser := os.Getenv("SUDO_USER")
	if runUser == "" {
		return ExecRaw(profile, cmd...)
	}

	projectDir, _ := os.Getwd()
	cmdStr := "cd '" + projectDir + "' && exec " + strings.Join(cmd, " ")

	args := []string{"netns", "exec", ns, "su", "-", runUser, "-c", cmdStr}
	c := exec.Command(IPPath(), args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// Exec runs a command inside a namespace.
func Exec(profile string, cmd ...string) (string, error) {
	ns := Name(profile)
	args := append([]string{"netns", "exec", ns}, cmd...)
	out, err := exec.Command(IPPath(), args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// IPPath returns the path to the ip command.
func IPPath() string {
	if p, err := exec.LookPath("ip"); err == nil {
		return p
	}
	// Fallback for sudo with restricted PATH
	for _, dir := range []string{"/usr/sbin", "/sbin", "/usr/local/sbin"} {
		p := dir + "/ip"
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "ip"
}

// IsConnected checks if the namespace has internet connectivity.
func IsConnected(profile string) bool {
	if !Exists(profile) {
		return false
	}
	_, err := Exec(profile, "ping", "-c", "1", "-W", "2", "1.1.1.1")
	return err == nil
}

func getDefaultDev() (string, error) {
	out, err := exec.Command(IPPath(), "route", "show", "default").CombinedOutput()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	// default via X dev Y ...
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", nil
}
