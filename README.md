# oc-vpn

Run [opencode](https://opencode.ai) through isolated WireGuard tunnels.

WireGuard with `AllowedIPs=0.0.0.0/0` captures **all** traffic. This tool isolates each tunnel in a **Linux network namespace** so multiple tunnels can coexist without route conflicts. The host's routing table is never modified.

## Requirements

| Tool | Purpose |
|------|---------|
| `wg` (wireguard-tools) | WireGuard interface management |
| `gum` | Terminal UI |
| `curl` | Public IP checks |
| Linux with `ip netns` support | Network namespace isolation |
| `sudo` / root | Namespace and iptables operations |

## Install

```bash
sudo ./install.sh
```

This symlinks `bin/oc-vpn` to `/usr/local/bin/oc-vpn`.

## Usage

```bash
# Check system requirements
oc-vpn doctor

# Import a WireGuard config
oc-vpn import ./us-east.conf -n us-east

# List profiles
oc-vpn list

# Launch opencode in a tunnel (interactive profile picker)
sudo oc-vpn run

# Launch directly in a named profile
sudo oc-vpn run us-east

# Bring up / tear down a tunnel
sudo oc-vpn up us-east
sudo oc-vpn down us-east

# Check public IP via tunnel
sudo oc-vpn ip us-east

# Live status dashboard
sudo oc-vpn status --live

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
├── bin/oc-vpn          # Main CLI entry point
├── lib/
│   ├── namespace.sh    # Network namespace management
│   ├── wireguard.sh    # WireGuard operations + IP/geo queries
│   ├── profiles.sh     # Profile CRUD
│   ├── doctor.sh       # System requirement checks
│   └── tui.sh          # gum-based terminal UI
├── test/               # Sample WireGuard configs
├── install.sh          # Symlink installer
└── README.md
```

## License

MIT
