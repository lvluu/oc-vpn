#!/usr/bin/env bash
# tui.sh — gum-based terminal UI for oc-vpn

DIM='\033[2m'; NC='\033[0m'

# ── Profile list with status ──────────────────────────────────

tui_profile_line() {
  local name="$1"
  local status endpoint latency transfer

  if wg_is_up "$name"; then
    status="UP"
    endpoint=$(wg_endpoint "$name")
    latency=$(wg_latency "$name")
    transfer=$(wg_transfer "$name")
  elif ns_exists "$name"; then
    status="NS"
    endpoint="-"
    latency="-"
    transfer="-"
  else
    status="DOWN"
    endpoint=$(profile_endpoint "$name")
    latency="-"
    transfer="-"
  fi

  printf "%-20s %-6s %-28s %-10s %s" "$name" "$status" "$endpoint" "$latency" "$transfer"
}

tui_list_profiles() {
  gum style --border rounded --padding "0 1" --bold "oc-vpn profiles"
  echo ""

  local profiles
  profiles=($(profile_list))

  if [[ ${#profiles[@]} -eq 0 ]]; then
    echo "  No profiles. Import one:"
    echo "  oc-vpn import <config.conf> -n <name>"
    return
  fi

  # Header
  gum style --bold "  $(printf '%-20s %-6s %-28s %-10s %s' "PROFILE" "STATUS" "ENDPOINT" "LATENCY" "TRANSFER")"
  echo ""

  local name
  for name in "${profiles[@]}"; do
    local status
    if wg_is_up "$name"; then
      status="$(gum style --foreground 212 "$(tui_profile_line "$name")")"
    elif ns_exists "$name"; then
      status="$(gum style --foreground 214 "$(tui_profile_line "$name")")"
    else
      status="$(tui_profile_line "$name")"
    fi
    echo "  $status"
  done
}

# ── Interactive profile picker ────────────────────────────────

tui_pick_profile() {
  local profiles
  profiles=($(profile_list))

  if [[ ${#profiles[@]} -eq 0 ]]; then
    echo ""
    return 1
  fi

  local items=()
  local name
  for name in "${profiles[@]}"; do
    local tag=""
    if wg_is_up "$name"; then
      tag=" [UP]"
    elif ns_exists "$name"; then
      tag=" [NS]"
    else
      tag=" [DOWN]"
    fi
    items+=("${name}${tag}")
  done

  gum choose --header "Select profile" "${items[@]}" | sed 's/ \[.*\]//'
}

# ── Interactive action picker ─────────────────────────────────

tui_pick_action() {
  local profile="$1"
  local status
  if wg_is_up "$profile"; then
    status="UP"
  else
    status="DOWN"
  fi

  gum choose --header "${profile} [${status}]" \
    "Run opencode" \
    "Shell" \
    "IP check" \
    "Status" \
    "Up" \
    "Down" \
    "Remove" \
    "Back"
}

# ── Status dashboard ──────────────────────────────────────────

tui_status() {
  clear
  local profiles
  profiles=($(profile_list))

  gum style --border double --padding "0 1" --bold \
    "oc-vpn status                    $(date +%H:%M:%S)"
  echo ""

  if [[ ${#profiles[@]} -eq 0 ]]; then
    echo "  No profiles configured."
    return
  fi

  local name
  for name in "${profiles[@]}"; do
    local status_str color
    if wg_is_up "$name"; then
      status_str="UP"
      color=212
    elif ns_exists "$name"; then
      status_str="NS"
      color=214
    else
      status_str="DOWN"
      color=245
    fi

    local endpoint handshake latency transfer
    endpoint=$(wg_endpoint "$name")
    handshake=$(wg_handshake_ago "$name")
    latency=$(wg_latency "$name")
    transfer=$(wg_transfer "$name")

    gum style --border rounded --padding "0 1" \
      "$(gum style --bold --foreground $color "${name}")" \
      "  Status:    ${status_str}" \
      "  Endpoint:  ${endpoint}" \
      "  Handshake: ${handshake}" \
      "  Latency:   ${latency}" \
      "  Transfer:  ${transfer}"
    echo ""
  done
}

# ── Live status loop ──────────────────────────────────────────

tui_status_live() {
  trap 'return' INT TERM
  while true; do
    tui_status
    sleep 2
  done
}

# ── IP check ──────────────────────────────────────────────────

tui_ip_check() {
  local name="$1"
  gum spin --spinner dot --title "Checking IP via ${name}..." -- bash -c \
    "source '${SCRIPT_DIR}/lib/wireguard.sh'; source '${SCRIPT_DIR}/lib/profiles.sh'; source '${SCRIPT_DIR}/lib/namespace.sh'; wg_public_ip '${name}'"

  local ip; ip=$(wg_public_ip "$name")

  if [[ -z "$ip" || "$ip" == "-" ]]; then
    gum style --border rounded --foreground 196 --padding "0 1" \
      "Failed to get IP. Is the tunnel up?"
    return
  fi

  # Get location info
  local info
  info=$(wg_ip_info "$name")
  local country city isp lat lon
  country=$(echo "$info" | grep -oP '"country":\s*"\K[^"]+' || echo "?")
  city=$(echo "$info" | grep -oP '"city":\s*"\K[^"]+' || echo "?")
  isp=$(echo "$info" | grep -oP '"isp":\s*"\K[^"]+' || echo "?")

  gum style --border double --padding "1 2" --bold \
    "Public IP (via ${name})" \
    "" \
    "IP:       ${ip}" \
    "Location: ${city}, ${country}" \
    "ISP:      ${isp}"
}

# ── Import wizard ─────────────────────────────────────────────

tui_import() {
  echo ""
  gum style --bold "Import WireGuard Config"
  echo ""

  # File selection
  local conf
  conf=$(gum file --file --directory --height 15)
  [[ -z "$conf" ]] && return

  # Profile name
  local name
  name=$(gum input --placeholder "Profile name (e.g. us-east)")
  [[ -z "$name" ]] && return

  # DNS override (optional)
  local dns
  dns=$(gum input --placeholder "DNS server (optional, Enter to skip)")
  if [[ -n "$dns" ]]; then
    profile_import "$conf" -n "$name" --dns "$dns"
  else
    profile_import "$conf" -n "$name"
  fi

  local result=$?
  if [[ $result -eq 0 ]]; then
    gum style --foreground 212 "  ✓ Profile '${name}' imported"
  elif [[ $result -eq 2 ]]; then
    gum style --foreground 196 "  ✗ File not found: ${conf}"
  else
    gum style --foreground 196 "  ✗ Import failed"
  fi
}

# ── Main TUI loop ─────────────────────────────────────────────

tui_main() {
  while true; do
    clear
    tui_list_profiles

    echo ""
    local action
    action=$(gum choose --header "Action" \
      "Run opencode" \
      "Import profile" \
      "Status dashboard" \
      "IP check" \
      "Doctor" \
      "Quit")

    case "$action" in
      "Quit")
        clear
        return
        ;;
      "Import profile")
        tui_import
        gum confirm "Press Enter to continue" || true
        ;;
      "Status dashboard")
        tui_status_live
        ;;
      "Doctor")
        clear
        doctor_run
        gum confirm "Press Enter to continue" || true
        ;;
      "Run opencode"|"IP check")
        local profile
        profile=$(tui_pick_profile) || continue

        case "$action" in
          "Run opencode")
            cmd_up "$profile" 2>/dev/null || true
            clear
            ok "Launching opencode in ${profile}..."
            ip netns exec "$(ns_name "$profile")" su - "$USER" -c "opencode" 2>/dev/null || \
              ip netns exec "$(ns_name "$profile")" opencode
            gum confirm "Press Enter to continue" || true
            ;;
          "IP check")
            clear
            if ! wg_is_up "$profile"; then
              gum spin --spinner dot --title "Bringing up ${profile}..." -- bash -c \
                "source '${SCRIPT_DIR}/lib/wireguard.sh'; source '${SCRIPT_DIR}/lib/profiles.sh'; source '${SCRIPT_DIR}/lib/namespace.sh'; wg_up '${profile}'"
            fi
            tui_ip_check "$profile"
            gum confirm "Press Enter to continue" || true
            ;;
        esac
        ;;
    esac
  done
}
