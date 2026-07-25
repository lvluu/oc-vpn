# Architecture

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

### Flow

1. **Namespace creation** — Each profile gets `ocvpn-<name>` namespace, connected to the host via a veth pair with a unique `/24` subnet (MD5-derived).
2. **WireGuard setup** — Uses `wg set` (not `wg-quick`) to configure the tunnel inside the namespace with no route manipulation.
3. **Routing** — The endpoint IP gets a host route via the veth; the default route goes through `wg0`.
4. **NAT** — iptables MASQUERADE forwards namespace traffic through the host's physical NIC.
5. **DNS** — `/etc/resolv.conf` is written inside the namespace.

### Subnet Allocation

Each namespace gets a unique `10.200.X.0/24` subnet derived from `MD5(namespace_name)[0] % 254 + 1`. This prevents routing conflicts when multiple profiles are active simultaneously.

## Project Structure

```
oc-vpn/
├── cmd/oc-vpn/
│   ├── main.go          # CLI entry point, command dispatch
│   └── version.go       # Version info (injected via ldflags)
├── internal/
│   ├── profiles/        # Profile CRUD, config parsing, wg-quick field stripping
│   ├── namespace/       # Network namespace lifecycle, veth pairs, iptables NAT
│   ├── wireguard/       # WireGuard operations (wg set), DNS, IP/geo lookups
│   ├── doctor/          # System requirement checks
│   └── tui/             # Terminal UI: profile picker, status dashboard
├── docs/                # Documentation
├── .goreleaser.yml      # Release automation
├── install.sh           # Curl-pipe installer
└── justfile             # Build commands
```

## Key Design Decisions

- **`wg set` over `wg-quick`** — wg-quick auto-manages routes which conflicts with namespace networking. We parse configs manually and strip wg-quick-only fields (Address, DNS, Table, SaveConfig, PostUp/Down).
- **MD5-based veth naming** — Linux limits interface names to 15 chars. We hash the namespace name to generate short veth names like `vceec` / `vceecn`.
- **Unique /24 subnets** — Multiple namespaces with the same `10.200.0.0/24` caused the host to route replies out the wrong veth. Each profile now gets a deterministic unique subnet.
- **Address in meta.json** — `Address` is a wg-quick-only field stripped during import but needed for `ip addr add` on `wg0`. Saved to `meta.json` as fallback.
