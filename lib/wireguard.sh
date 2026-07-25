#!/usr/bin/env bash
# wireguard.sh — WireGuard operations inside network namespaces
#
# Uses "wg set" directly instead of wg-quick to avoid automatic
# route management that conflicts with namespace networking.

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

  # ── Parse config manually ──────────────────────────────────
  local private_key address dns endpoint public_key allowed_ips
  private_key=$(sed -n 's/^PrivateKey\s*=\s*//p' "$conf" | head -1)
  address=$(sed -n 's/^Address\s*=\s*//p' "$conf" | head -1)
  dns=$(sed -n 's/^DNS\s*=\s*//p' "$conf" | head -1)
  endpoint=$(sed -n 's/^Endpoint\s*=\s*//p' "$conf" | head -1)
  public_key=$(sed -n '/^\[Peer\]/,/^$/s/^PublicKey\s*=\s*//p' "$conf" | head -1)
  allowed_ips=$(sed -n '/^\[Peer\]/,/^$/s/^AllowedIPs\s*=\s*//p' "$conf" | head -1)
  local keepalive
  keepalive=$(sed -n '/^\[Peer\]/,/^$/s/^PersistentKeepalive\s*=\s*//p' "$conf" | head -1)

  [[ -z "$private_key" || -z "$endpoint" || -z "$public_key" ]] && return 1

  # ── Create WireGuard interface ─────────────────────────────
  ns_exec "$name" ip link add dev wg type wireguard

  # Apply config via wg set (no route management)
  local wg_set_args=(
    "$name" wg set wg0
    private-key <(echo "$private_key")
    peer "$public_key"
    endpoint "$endpoint"
    allowed-ips "${allowed_ips:-0.0.0.0/0}"
  )
  [[ -n "$keepalive" ]] && wg_set_args+=(persistent-keepalive "$keepalive")

  ns_exec "$name" wg setconf wg0 <(sed -n '/^\[Interface\]/,/^\[Peer\]/p' "$conf" | sed '1d;$d')
  ns_exec "$name" wg set wg0 peer "$public_key" endpoint "$endpoint" allowed-ips "${allowed_ips:-0.0.0.0/0}" ${keepalive:+persistent-keepalive "$keepalive"}

  # ── Assign IP address ──────────────────────────────────────
  # Strip CIDR mask for ip addr (e.g., "10.104.8.99/32" → "10.104.8.99/32" kept as-is)
  ns_exec "$name" ip addr add "$address" dev wg 2>/dev/null || true
  ns_exec "$name" ip link set mtu 1420 up dev wg

  # ── DNS setup ──────────────────────────────────────────────
  if [[ -n "$dns" ]] && command -v resolvconf &>/dev/null; then
    # resolvconf is available — let it handle DNS
    echo "$dns" | tr ',' '\n' | sed 's/^nameserver /nameserver /' | \
      ns_exec "$name" resolvconf -a tun.wg -m 0 -x 2>/dev/null || true
  elif [[ -n "$dns" ]]; then
    # No resolvconf — use /etc/netns/ fallback
    local ns; ns=$(ns_name "$name")
    mkdir -p "/etc/netns/${ns}"
    echo "$dns" | tr ',' '\n' | sed 's/^/nameserver /' > "/etc/netns/${ns}/resolv.conf"
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

# ── Geo lookup (cached) ──────────────────────────────────────

_geo_cache="${OC_VPN_PROFILES_DIR:-$HOME/.config/oc-vpn}/geo-cache"

geo_lookup() {
  local ip="$1"
  [[ -z "$ip" || "$ip" == "-" ]] && { echo "-"; return; }

  # Check cache
  if [[ -f "$_geo_cache" ]]; then
    local cached
    cached=$(sed -n "s/^${ip}|//p" "$_geo_cache" | head -1)
    if [[ -n "$cached" ]]; then
      echo "$cached"
      return
    fi
  fi

  # Query ip-api.com (free, no key, 45 req/min)
  local response
  response=$(curl -s --max-time 3 "http://ip-api.com/json/${ip}?fields=country,city" 2>/dev/null || true)

  local country city result
  country=$(echo "$response" | sed -n 's/.*"country":"\([^"]*\)".*/\1/p' | head -1)
  city=$(echo "$response" | sed -n 's/.*"city":"\([^"]*\)".*/\1/p' | head -1)

  if [[ -n "$country" ]]; then
    if [[ -n "$city" ]]; then
      result="${city}, ${country}"
    else
      result="${country}"
    fi
  else
    result="-"
  fi

  # Cache it
  mkdir -p "$(dirname "$_geo_cache")"
  echo "${ip}|${result}" >> "$_geo_cache"

  echo "$result"
}
