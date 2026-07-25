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
  ip netns "$ns" ip link set lo up
}

ns_destroy() {
  local ns; ns=$(ns_name "$1")
  # Disconnect WireGuard first
  ip netns exec "$ns" wg-quick down wg0 2>/dev/null || \
    ip netns exec "$ns" ip link del wg0 2>/dev/null || true
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
