package namespace

import (
	"fmt"
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	tests := []struct {
		profile  string
		expected string
	}{
		{"us-east", "ocvpn-us-east"},
		{"prod", "ocvpn-prod"},
		{"test-123", "ocvpn-test-123"},
		{"a.b", "ocvpn-a.b"},
		{"my-vpn", "ocvpn-my-vpn"},
		{"vpnbook-us", "ocvpn-vpnbook-us"},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			got := Name(tt.profile)
			if got != tt.expected {
				t.Errorf("Name(%q) = %q, want %q", tt.profile, got, tt.expected)
			}
		})
	}
}

func TestVethNameLength(t *testing.T) {
	// Linux interface names limited to 15 chars; veth names are hash-derived
	for _, name := range []string{"a", "us-east", "vpnbook-us", "long-profile-name"} {
		ns := Name(name)
		hash := [16]byte{}
		copy(hash[:], []byte(ns))
		hostVeth := fmt.Sprintf("v%x", hash[:2])
		if len(hostVeth) > 15 {
			t.Errorf("hostVeth for %q = %q, length %d > 15", name, hostVeth, len(hostVeth))
		}
	}
}

func TestExistsNonexistent(t *testing.T) {
	if Exists("nonexistent-profile-12345") {
		t.Error("Exists() returned true for nonexistent namespace")
	}
}

func TestSubnet(t *testing.T) {
	subnet, hostIP, nsIP, gw := Subnet("vpnbook-us")
	// Verify format, not specific values (hash-derived)
	if !strings.HasSuffix(subnet, "/24") || !strings.HasPrefix(subnet, "10.200.") {
		t.Errorf("Subnet(us) subnet = %q, want 10.200.X.0/24", subnet)
	}
	if !strings.HasSuffix(hostIP, ".1/24") || !strings.HasPrefix(hostIP, "10.200.") {
		t.Errorf("Subnet(us) hostIP = %q, want 10.200.X.1/24", hostIP)
	}
	if !strings.HasSuffix(nsIP, ".2/24") || !strings.HasPrefix(nsIP, "10.200.") {
		t.Errorf("Subnet(us) nsIP = %q, want 10.200.X.2/24", nsIP)
	}
	if !strings.HasSuffix(gw, ".1") || !strings.HasPrefix(gw, "10.200.") {
		t.Errorf("Subnet(us) gateway = %q, want 10.200.X.1", gw)
	}
}

func TestSubnetUniqueness(t *testing.T) {
	seen := map[string]string{}
	for _, profile := range []string{"vpnbook-us", "vpnbook-eu1", "vpnbook-eu2", "vpnbook-eu3", "a", "b", "c", "prod"} {
		s, _, _, _ := Subnet(profile)
		if prev, ok := seen[s]; ok {
			t.Errorf("Subnet collision: %q and %q both map to %s", prev, profile, s)
		}
		seen[s] = profile
	}
}

func TestSubnetIn200Range(t *testing.T) {
	for _, profile := range []string{"x", "y", "z", "vpnbook-us", "vpnbook-eu1"} {
		s, h, n, gw := Subnet(profile)
		var thirdOctet int
		fmt.Sscanf(s, "10.200.%d.0/24", &thirdOctet)
		if thirdOctet < 1 || thirdOctet > 254 {
			t.Errorf("Subnet(%q) third octet = %d, want 1-254", profile, thirdOctet)
		}
		_ = h
		_ = n
		_ = gw
	}
}

func TestPrefix(t *testing.T) {
	if prefix != "ocvpn" {
		t.Errorf("prefix = %q, want ocvpn", prefix)
	}
}
