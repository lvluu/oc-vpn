# oc-vpn

Run [opencode](https://opencode.ai) through isolated WireGuard VPN tunnels.

Each profile gets its own Linux network namespace. Multiple VPNs run side by side without touching your host routing table.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/lvluu/oc-vpn/main/install.sh | bash
```

Or from source:

```bash
git clone https://github.com/lvluu/oc-vpn.git && cd oc-vpn
just deploy
```

### Requirements

Linux with `wireguard-tools`, `iproute2`, `iptables`, `curl`.

```bash
# Ubuntu/Debian
sudo apt install wireguard-tools curl iptables iproute2

# Arch
sudo pacman -S wireguard-tools curl iptables iproute2
```

## Quick Start

```bash
# Import a WireGuard config
sudo oc-vpn import tunnel.conf -n my-vpn

# Bring it up
sudo oc-vpn up my-vpn

# Check it works
sudo oc-vpn ip my-vpn

# Run opencode inside the tunnel
sudo oc-vpn run my-vpn
```

## Commands

| Command | What it does |
|---------|-------------|
| `oc-vpn import <file> -n <name>` | Import a WireGuard config |
| `oc-vpn up <name>` | Connect a profile |
| `oc-vpn down <name>` | Disconnect |
| `oc-vpn run <name>` | Launch opencode in a tunnel |
| `oc-vpn shell <name>` | Shell inside the namespace |
| `oc-vpn status` | Dashboard for all profiles |
| `oc-vpn ip <name>` | Show public IP via tunnel |
| `oc-vpn list` | List all profiles |
| `oc-vpn doctor` | Check system requirements |

## Why?

WireGuard with `AllowedIPs=0.0.0.0/0` captures all traffic. Running two tunnels at once fights over the default route. oc-vpn isolates each tunnel in its own namespace so they never conflict. Your host stays clean.

```
namespace (tunnel A) ──veth──┐
                              ├── NAT ── physical NIC
namespace (tunnel B) ──veth──┘
```

## License

MIT
