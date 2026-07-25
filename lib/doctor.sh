#!/usr/bin/env bash
# doctor.sh — System requirement checks for oc-vpn

doctor_check() {
  local name="$1" cmd="$2"
  local version_output
  if version_output=$(eval "$cmd" 2>&1); then
    gum style --foreground 212 "  ✓ ${name}  ${DIM}${version_output}${NC}"
    return 0
  else
    gum style --foreground 196 "  ✗ ${name}  ${DIM}${version_output:-not found}${NC}"
    return 1
  fi
}

doctor_run() {
  local errors=0

  gum style --border rounded --padding "0 1" --bold "System Check"
  echo ""

  # wireguard tools
  if command -v wg &>/dev/null; then
    local ver; ver=$(wg --version 2>&1 | head -1)
    gum style --foreground 212 "  ✓ wg  ${DIM}${ver}${NC}"
  else
    gum style --foreground 196 "  ✗ wg  ${DIM}not found — install: pacman -S wireguard-tools${NC}"
    ((errors++))
  fi

  # wireguard kernel module
  if modprobe wireguard 2>/dev/null || lsmod 2>/dev/null | grep -q wireguard; then
    gum style --foreground 212 "  ✓ wireguard kernel module  ${DIM}loaded${NC}"
  else
    gum style --foreground 214 "  ⚠ wireguard kernel module  ${DIM}not loaded (may load on demand)${NC}"
  fi

  # network namespaces
  if ip netns add _ocvpn_test 2>/dev/null; then
    ip netns del _ocvpn_test 2>/dev/null
    gum style --foreground 212 "  ✓ network namespaces  ${DIM}supported${NC}"
  else
    gum style --foreground 196 "  ✗ network namespaces  ${DIM}not available${NC}"
    ((errors++))
  fi

  # resolvconf
  if command -v resolvconf &>/dev/null; then
    gum style --foreground 212 "  ✓ resolvconf  ${DIM}available${NC}"
  else
    gum style --foreground 214 "  ⚠ resolvconf  ${DIM}not found — DNS will use /etc/netns/ fallback${NC}"
  fi

  # gum
  if command -v gum &>/dev/null; then
    local gver; gver=$(gum --version 2>&1 | head -1)
    gum style --foreground 212 "  ✓ gum  ${DIM}${gver}${NC}"
  else
    gum style --foreground 196 "  ✗ gum  ${DIM}not found — install: pacman -S gum${NC}"
    ((errors++))
  fi

  # curl (for IP checks)
  if command -v curl &>/dev/null; then
    gum style --foreground 212 "  ✓ curl  ${DIM}available${NC}"
  else
    gum style --foreground 214 "  ⚠ curl  ${DIM}not found — IP checks will not work${NC}"
  fi

  # root
  if [[ $EUID -eq 0 ]]; then
    gum style --foreground 212 "  ✓ root privileges  ${DIM}yes${NC}"
  else
    gum style --foreground 214 "  ⚠ root privileges  ${DIM}no — some commands need sudo${NC}"
  fi

  # profiles dir
  if [[ -d "$PROFILES_DIR" ]]; then
    local count; count=$(profile_count 2>/dev/null || echo 0)
    gum style --foreground 212 "  ✓ profiles dir  ${DIM}${PROFILES_DIR} (${count} profiles)${NC}"
  else
    gum style --foreground 214 "  ⚠ profiles dir  ${DIM}${PROFILES_DIR} (will be created on first import)${NC}"
  fi

  echo ""
  if (( errors > 0 )); then
    gum style --foreground 196 "  ${errors} issue(s) found. Install missing dependencies and try again."
  else
    gum style --foreground 212 "  All checks passed."
  fi
}
