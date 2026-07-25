# oc-vpn

Run [opencode](https://opencode.ai) through isolated WireGuard tunnels.

WireGuard with `AllowedIPs=0.0.0.0/0` captures **all** traffic. This tool isolates each tunnel in a **Linux network namespace** so multiple tunnels can coexist without route conflicts. The host's routing table is never modified.

## Prerequisites

### Required

| Tool | Package | Purpose |
|------|---------|---------|
| `wg` | `wireguard-tools` | WireGuard interface management |
| `gum` | `gum` | Terminal UI (profile picker, status dashboard) |
| `curl` | `curl` | Public IP checks |
| `ip` | `iproute2` | Network namespace and interface management |
| `iptables` | `iptables` | NAT for namespace traffic |
| `sudo` / root | — | Namespace and iptables operations require root |

### Linux Kernel

- WireGuard kernel module (`wireguard`)
- Network namespaces (`CONFIG_NETNS`)
- veth device support

Most modern Linux distributions (kernel 5.6+) include WireGuard in-tree.

### Install by Distro

```bash
# Arch Linux
sudo pacman -S wireguard-tools gum curl iptables iproute2

# Ubuntu / Debian
sudo apt install wireguard-tools gum curl iptables iproute2

# Fedora / RHEL
sudo dnf install wireguard-tools gum curl iptables iproute2
```

> **Note**: `gum` may not be in your distro's default repos. See [github.com/charmbracelet/gum](https://github.com/charmbracelet/gum) for install instructions.

## Install

### From source

```bash
git clone https://github.com/lvluu/oc-vpn.git
cd oc-vpn
go build -o oc-vpn ./cmd/oc-vpn
sudo mv oc-vpn /usr/local/bin/
```

### Run tests

```bash
go test ./...
```

## Doctor

Run `oc-vpn doctor` to verify your system is ready. It checks each dependency and reports pass/warn/fail:

```bash
$ oc-vpn doctor

System checks:

  ✓ wg — wireguard-tools v1.0.20210914
  ✓ wireguard module — loaded
  ✓ network namespaces — available (needs sudo)
  ! resolvconf — not found — DNS uses /etc/resolv.conf fallback
  ✓ gum — v0.14.5
  ✓ curl — available
  ! root privileges — no — run with sudo
  ✓ profiles dir — /home/user/.config/oc-vpn/profiles (2 profiles)
```

### What doctor checks

| Check | What happens | Fix |
|-------|-------------|-----|
| **wg** | `command -v wg` | `sudo pacman -S wireguard-tools` |
| **wireguard module** | `modprobe wireguard` or `lsmod` | Usually loads on demand; verify with `modprobe wireguard` |
| **network namespaces** | Creates and deletes a test namespace | Ensure `ip` from iproute2 is installed; needs root |
| **resolvconf** | `command -v resolvconf` | Optional — DNS falls back to `/etc/netns/` without it |
| **gum** | `command -v gum` | [Install gum](https://github.com/charmbracelet/gum#installation) |
| **curl** | `command -v curl` | `sudo pacman -S curl` (or distro equivalent) |
| **root** | Check `$EUID` | Run with `sudo` |
| **profiles dir** | Checks `~/.config/oc-vpn/profiles/` | Created automatically on first import |

If a check fails, fix the issue and re-run `oc-vpn doctor` until all checks pass.

## Usage

```bash
# Import a WireGuard config
oc-vpn import ./us-east.conf -n us-east

# List profiles
oc-vpn list

# Launch directly in a named profile
sudo oc-vpn run us-east

# Bring up / tear down a tunnel
sudo oc-vpn up us-east
sudo oc-vpn down us-east

# Check public IP via tunnel
sudo oc-vpn ip us-east

# Live status dashboard
sudo oc-vpn status

# Drop into a shell inside the namespace
sudo oc-vpn shell us-east

# Show a profile's WireGuard config
oc-vpn export us-east

# Remove a profile
oc-vpn remove us-east
```

## How It Works

```
┌─────────────────────────────────────────────────┐
│  Host                                           │
│                                                 │
│  ┌───────────────┐     ┌───────────────┐        │
│  │ netns:        │     │ netns:        │        │
│  │ ocvpn-us-east │     │ ocvpn-eu-west │        │
│  │               │     │               │        │
│  │  wg0 ─────────┼──┐  │  wg0 ─────────┼──┐     │
│  │  veth ────────┼──┼──┼── veth ───────┼──┼─┐   │
│  └───────────────┘  │  └───────────────┘  │ │   │
│                     │                     │ │   │
│  ┌──────────────────┼─────────────────────┼─┴─┐ │
│  │  veth bridge + NAT (iptables)         │   │ │
│  └──────────────────┴─────────────────────┴───┘ │
│                    │                           │
│                    └──── physical NIC ──────────┘
└─────────────────────────────────────────────────┘
```

1. **Namespace creation** — Each profile gets its own network namespace (`ocvpn-<name>`) connected to the host via a veth pair
2. **WireGuard setup** — `wg set` (not `wg-quick`) configures the tunnel inside the namespace with no route manipulation
3. **Routing** — The endpoint IP gets a host route via the veth; the default route goes through `wg0`
4. **NAT** — iptables MASQUERADE forwards namespace traffic through the host's physical NIC
5. **DNS** — `/etc/resolv.conf` is written inside the namespace

## Profiles

Stored in `~/.config/oc-vpn/profiles/<name>/`:
- `wg.conf` — WireGuard config (stripped of wg-quick-only fields)
- `meta.json` — Import metadata (name, source, endpoint, timestamp)

## Project Structure

```
oc-vpn/
├── cmd/oc-vpn/main.go      # CLI entry point
├── internal/
│   ├── profiles/            # Profile CRUD, config parsing
│   ├── namespace/           # Network namespace management
│   ├── wireguard/           # WireGuard operations, IP/geo queries
│   ├── doctor/              # System requirement checks
│   └── tui/                 # Terminal UI (gum + fallback)
├── test/                    # Sample WireGuard configs
└── README.md
```

## License

MIT
