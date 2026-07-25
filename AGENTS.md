# AGENTS.md

## Overview

oc-vpn is a Go CLI tool that runs opencode through isolated WireGuard VPN tunnels using Linux network namespaces. It never modifies the host routing table.

## Architecture

- `cmd/oc-vpn/main.go` — CLI entry point, command dispatch
- `internal/profiles/` — Profile CRUD under `~/.config/oc-vpn/profiles/`, config parsing, wg-quick field stripping
- `internal/namespace/` — Network namespace lifecycle (create/destroy/exists), veth pair setup, iptables NAT, UFW rules
- `internal/wireguard/` — WireGuard operations using `wg set` (not `wg-quick`), DNS setup, IP/geo lookups
- `internal/doctor/` — System requirement checks (wg, gum, namespaces, curl, root)
- `internal/tui/` — Terminal UI: profile picker, status dashboard, IP check, import wizard

## Key Design Decisions

- **`wg set` over `wg-quick`**: wg-quick auto-manages routes which conflicts with namespace networking. We parse configs manually and strip wg-quick-only fields (Address, DNS, Table, SaveConfig, PostUp/Down, etc.)
- **Network namespaces**: Each profile gets `ocvpn-<name>` namespace. Traffic flows: namespace → veth → host NAT → physical NIC → WireGuard endpoint
- **veth naming**: Uses `crypto/md5` hash of namespace name for short veth names (Linux 15-char limit on interface names)
- **DNS**: Written directly to `/etc/resolv.conf` inside the namespace (not via resolvconf)
- **Endpoint routing**: The WireGuard endpoint IP gets a specific host route via the veth so the tunnel handshake works, then the default route goes through `wg0`

## Conventions

- Standard Go project layout with `cmd/` and `internal/` packages
- Root required for namespace/WireGuard operations; checked via `os.Getuid()`
- Profiles stored at `$OC_VPN_PROFILES_DIR` or `~/.config/oc-vpn/profiles/`
- Colored terminal output using ANSI escape codes
- Terminal UI tries `gum` first, falls back to stdin prompts

## Testing

- Unit tests in `*_test.go` files alongside source
- `go test ./...` runs all tests
- `test/` contains sample WireGuard configs (gitignored)

## Dependencies

- wireguard-tools (`wg`)
- `gum` (terminal UI, optional — falls back to stdin)
- `curl` (IP checks)
- Linux kernel with `ip netns` support, iptables, sysctl
- Root/sudo access for namespace operations
- Go 1.22+

## Build

```bash
go build -o oc-vpn ./cmd/oc-vpn
```
