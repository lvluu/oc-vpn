#!/usr/bin/env bash
# namespace.sh — Linux network namespace management for oc-vpn

NS_PREFIX="ocvpn"

ns_name() { echo "${NS_PREFIX}-${1}"; }

ns_exists() {
  local ns; ns=$(ns_name "$1")
  ip netns list 2>/dev/null | grep -qw "^${ns}"
}

ns_create() {
  local ns; ns=$(ns_name "$1")
  if ns_exists "$1"; then
    return 1
  fi

  ip netns add "$ns"
  ip netns exec "$ns" ip link set lo up

  # Create veth pair — use short names (Linux 15-char limit)
  # ns name: "ocvpn-<profile>" → veth: "v<hash4>" / "v<hash4>n"
  local hash; hash=$(echo -n "$ns" | md5sum | head -c 4)
  local host_veth="v${hash}"
  local ns_veth="v${hash}n"

  ip link add "$host_veth" type veth peer name "$ns_veth"
  ip link set "$ns_veth" netns "$ns"

  ip addr add 10.200.0.1/24 dev "$host_veth" 2>/dev/null || true
  ip link set "$host_veth" up

  ip netns exec "$ns" ip addr add 10.200.0.2/24 dev "$ns_veth"
  ip netns exec "$ns" ip link set "$ns_veth" up
  ip netns exec "$ns" ip route add default via 10.200.0.1

  # NAT so namespace traffic can reach the physical network
  sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true
  local dev; dev=$(ip route show default | awk '{print $5}' | head -1)
  if [[ -n "$dev" ]]; then
    iptables -t nat -A POSTROUTING -s 10.200.0.0/24 -o "$dev" -j MASQUERADE \
      -m comment --comment "oc-vpn-${ns}" 2>/dev/null || true
  fi
}

ns_destroy() {
  local ns; ns=$(ns_name "$1")

  # Disconnect WireGuard
  ip netns exec "$ns" wg-quick down wg0 2>/dev/null || \
    ip netns exec "$ns" ip link del wg0 2>/dev/null || true

  # Remove veth pair (host side deletion removes both ends)
  local hash; hash=$(echo -n "$ns" | md5sum | head -c 4)
  ip link del "v${hash}" 2>/dev/null || true

  # Remove iptables rule by comment
  local dev; dev=$(ip route show default | awk '{print $5}' | head -1)
  if [[ -n "$dev" ]]; then
    iptables -t nat -D POSTROUTING -s 10.200.0.0/24 -o "$dev" -j MASQUERADE \
      -m comment --comment "oc-vpn-${ns}" 2>/dev/null || true
  fi

  # Delete namespace
  ip netns del "$ns" 2>/dev/null || true
  rm -f "/etc/netns/${ns}/resolv.conf" 2>/dev/null || true
  rmdir "/etc/netns/${ns}" 2>/dev/null || true
}

ns_exec() {
  local ns; ns=$(ns_name "$1")
  shift
  ip netns exec "$ns" "$@"
}

ns_is_connected() {
  local ns; ns=$(ns_name "$1")
  ns_exists "$1" && ip netns exec "$ns" ping -c 1 -W 2 1.1.1.1 &>/dev/null
}
