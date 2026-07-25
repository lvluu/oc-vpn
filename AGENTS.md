# AGENTS.md

## Overview

oc-vpn is a Bash CLI tool that runs opencode through isolated WireGuard VPN tunnels using Linux network namespaces. It never modifies the host routing table.

## Architecture

- `bin/oc-vpn` — Main entry point, CLI parsing, command dispatch
- `lib/namespace.sh` — Namespace lifecycle (create/destroy/exists), veth pair setup, iptables NAT, UFW route rules
- `lib/wireguard.sh` — WireGuard operations using `wg set` (not `wg-quick`), DNS setup, IP/geo lookups with caching
- `lib/profiles.sh` — Profile CRUD under `~/.config/oc-vpn/profiles/`, config import with wg-quick field stripping
- `lib/doctor.sh` — System requirement checks (wg, gum, namespaces, curl, root)
- `lib/tui.sh` — gum-based terminal UI: profile picker, status dashboard, IP check, import wizard

## Key Design Decisions

- **`wg set` over `wg-quick`**: wg-quick auto-manages routes which conflicts with namespace networking. We parse configs manually and strip wg-quick-only fields (Address, DNS, Table, SaveConfig, PostUp/Down, etc.)
- **Network namespaces**: Each profile gets `ocvpn-<name>` namespace. Traffic flows: namespace → veth → host NAT → physical NIC → WireGuard endpoint
- **veth naming**: Uses `md5sum` hash of namespace name for short veth names (Linux 15-char limit on interface names)
- **DNS**: Written directly to `/etc/resolv.conf` inside the namespace (not via resolvconf)
- **Endpoint routing**: The WireGuard endpoint IP gets a specific host route via the veth so the tunnel handshake works, then the default route goes through `wg0`

## Conventions

- All scripts use `set -euo pipefail`
- Helper functions: `die()` (red error + exit), `ok()` (green check), `info()` (blue ::), `warn()` (yellow !)
- Root required for namespace/WireGuard operations; checked via `require_root`
- Profiles stored at `$OC_VPN_PROFILES_DIR` or `~/.config/oc-vpn/profiles/`
- Colored output using ANSI escape codes

## Testing

Sample WireGuard configs in `test/` (gitignored). No automated test suite currently.

## Dependencies

- wireguard-tools (`wg`)
- `gum` (terminal UI)
- `curl` (IP checks)
- Linux kernel with `ip netns` support, iptables, sysctl
- Root/sudo access for namespace operations
