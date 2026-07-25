#!/usr/bin/env bash
# doctor.sh — System requirement checks for oc-vpn

doctor_run() {
  local errors=0

  gum style --border rounded --padding "0 1" --bold "System Check"
  echo ""

  # wireguard tools
  if command -v wg &>/dev/null; then
    local ver; ver=$(wg --version 2>&1 | head -1)
    gum style --foreground 212 "  ✓ wg  ${ver}"
  else
    gum style --foreground 196 "  ✗ wg  not found — install: pacman -S wireguard-tools"
    ((errors++))
  fi

  # wireguard kernel module
  if modprobe wireguard 2>/dev/null || lsmod 2>/dev/null | grep -q wireguard; then
    gum style --foreground 212  "  ✓ wireguard kernel module  loaded"
  else
    gum style --foreground 214  "  ⚠ wireguard kernel module  not loaded (may load on demand)"
  fi

  # network namespaces (needs root)
  if [[ $EUID -eq 0 ]]; then
    if ip netns add _ocvpn_test 2>/dev/null; then
      ip netns del _ocvpn_test 2>/dev/null
      gum style --foreground 212  "  ✓ network namespaces  supported"
    else
      gum style --foreground 196  "  ✗ network namespaces  not available"
      ((errors++))
    fi
  else
    # Not root — check if command exists and if sudo works
    if command -v ip &>/dev/null; then
      gum style --foreground 212  "  ✓ network namespaces  available (needs sudo)"
    else
      gum style --foreground 196  "  ✗ network namespaces  ip command not found"
      ((errors++))
    fi
  fi

  # resolvconf
  if command -v resolvconf &>/dev/null; then
    gum style --foreground 212  "  ✓ resolvconf  available"
  else
    gum style --foreground 214  "  ⚠ resolvconf  not found — DNS will use /etc/netns/ fallback"
  fi

  # gum
  if command -v gum &>/dev/null; then
    local gver; gver=$(gum --version 2>&1 | head -1)
    gum style --foreground 212  "  ✓ gum  ${gver}"
  else
    gum style --foreground 196  "  ✗ gum  not found — install: pacman -S gum"
    ((errors++))
  fi

  # curl (for IP checks)
  if command -v curl &>/dev/null; then
    gum style --foreground 212  "  ✓ curl  available"
  else
    gum style --foreground 214  "  ⚠ curl  not found — IP checks will not work"
  fi

  # root
  if [[ $EUID -eq 0 ]]; then
    gum style --foreground 212  "  ✓ root privileges  yes"
  else
    gum style --foreground 214  "  ⚠ root privileges  no — run with sudo"
  fi

  # profiles dir
  if [[ -d "$PROFILES_DIR" ]]; then
    local count; count=$(profile_count 2>/dev/null || echo 0)
    gum style --foreground 212  "  ✓ profiles dir  ${PROFILES_DIR} (${count} profiles)"
  else
    gum style --foreground 214  "  ⚠ profiles dir  ${PROFILES_DIR} (will be created on first import)"
  fi

  echo ""
  if (( errors > 0 )); then
    gum style --foreground 196 "  ${errors} issue(s) found."
  else
    gum style --foreground 212 "  All checks passed."
  fi
}
