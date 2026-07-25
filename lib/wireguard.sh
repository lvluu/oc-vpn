#!/usr/bin/env bash
# wireguard.sh — WireGuard operations inside network namespaces

wg_up() {
  local name="$1"
  profile_exists "$name" || return 1
  local conf; conf=$(profile_conf "$name")

  # Create namespace if needed
  if ! ns_exists "$name"; then
    ns_create "$name" || return 1
  fi

  # If interface already exists, skip
  if ns_exec "$name" wg show interfaces 2>/dev/null | grep -qw "wg0"; then
    return 0
  fi

  # Bring up WireGuard inside namespace
  ns_exec "$name" wg-quick up "$conf"

  # DNS setup — resolvconf handles it if available
  # Fallback: /etc/netns/<ns>/resolv.conf
  if ! command -v resolvconf &>/dev/null; then
    local ns; ns=$(ns_name "$name")
    local dns; dns=$(profile_dns_servers "$name")
    if [[ -n "$dns" ]]; then
      mkdir -p "/etc/netns/${ns}"
      echo "$dns" | tr ',' '\n' | sed 's/^/nameserver /' > "/etc/netns/${ns}/resolv.conf"
    fi
  fi
}

wg_down() {
  local name="$1"
  if ! ns_exists "$name"; then
    return 0
  fi
  ns_destroy "$name"
}

wg_is_up() {
  local name="$1"
  ns_exists "$name" && ns_exec "$name" wg show interfaces 2>/dev/null | grep -qw "wg0"
}

wg_status() {
  local name="$1"
  if ! wg_is_up "$name"; then
    echo "down"
    return
  fi

  # Handshake time
  local handshake
  handshake=$(ns_exec "$name" wg show wg0 latest-handshakes 2>/dev/null | awk '{print $2}' | head -1 || echo 0)
  local now; now=$(date +%s)
  local diff=$(( now - handshake ))

  if (( diff > 180 )); then
    echo "stale"
  elif (( handshake == 0 )); then
    echo "no-handshake"
  else
    echo "up"
  fi
}

wg_handshake_ago() {
  local name="$1"
  if ! wg_is_up "$name"; then
    echo "-"
    return
  fi
  local handshake
  handshake=$(ns_exec "$name" wg show wg0 latest-handshakes 2>/dev/null | awk '{print $2}' | head -1 || echo 0)
  if (( handshake == 0 )); then
    echo "never"
    return
  fi
  local now; now=$(date +%s)
  local diff=$(( now - handshake ))

  if (( diff < 60 )); then
    echo "${diff}s ago"
  elif (( diff < 3600 )); then
    echo "$(( diff / 60 ))m ago"
  else
    echo "$(( diff / 3600 ))h ago"
  fi
}

wg_transfer() {
  local name="$1"
  if ! wg_is_up "$name"; then
    echo "- / -"
    return
  fi
  local rx tx
  rx=$(cat "/sys/class/net/wg0/statistics/rx_bytes" 2>/dev/null || echo 0)
  tx=$(cat "/sys/class/net/wg0/statistics/tx_bytes" 2>/dev/null || echo 0)

  # Run inside namespace
  rx=$(ns_exec "$name" cat /sys/class/net/wg0/statistics/rx_bytes 2>/dev/null || echo 0)
  tx=$(ns_exec "$name" cat /sys/class/net/wg0/statistics/tx_bytes 2>/dev/null || echo 0)

  echo "$(numfmt --to=iec "$rx" 2>/dev/null || echo "${rx}B") ↑ / $(numfmt --to=iec "$tx" 2>/dev/null || echo "${tx}B") ↓"
}

wg_endpoint() {
  local name="$1"
  if ! wg_is_up "$name"; then
    echo "-"
    return
  fi
  ns_exec "$name" wg show wg0 endpoints 2>/dev/null | awk '{print $2}' | head -1 || echo "-"
}

wg_latency() {
  local name="$1"
  if ! wg_is_up "$name"; then
    echo "-"
    return
  fi
  local endpoint; endpoint=$(wg_endpoint "$name")
  local host; host=$(echo "$endpoint" | grep -oP '^\d+\.\d+\.\d+\.\d+' || true)
  if [[ -z "$host" ]]; then
    echo "-"
    return
  fi
  local rtt
  rtt=$(ns_exec "$name" ping -c 1 -W 3 "$host" 2>/dev/null | grep -oP 'time=\K[\d.]+' || true)
  echo "${rtt:-?}ms"
}

wg_public_ip() {
  local name="$1"
  if ! wg_is_up "$name"; then
    echo "-"
    return
  fi
  ns_exec "$name" curl -s --max-time 5 ifconfig.me 2>/dev/null || echo "-"
}

wg_ip_info() {
  local name="$1"
  if ! wg_is_up "$name"; then
    echo "{}"
    return
  fi
  ns_exec "$name" curl -s --max-time 5 "http://ip-api.com/json" 2>/dev/null || echo "{}"
}
