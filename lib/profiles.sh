#!/usr/bin/env bash
# profiles.sh — Profile CRUD for oc-vpn

# Use real user's home even when sudoed
_real_home() {
  if [[ -n "${SUDO_USER:-}" ]]; then
    eval echo "~${SUDO_USER}"
  else
    echo "$HOME"
  fi
}

PROFILES_DIR="${OC_VPN_PROFILES_DIR:-$(_real_home)/.config/oc-vpn/profiles}"

profile_dir()  { echo "${PROFILES_DIR}/${1}"; }
profile_conf() { echo "${PROFILES_DIR}/${1}/wg.conf"; }
profile_meta() { echo "${PROFILES_DIR}/${1}/meta.json"; }
profile_dns()  { echo "${PROFILES_DIR}/${1}/dns"; }

profile_exists() { [[ -d "$(profile_dir "$1")" ]]; }

profile_list() {
  mkdir -p "$PROFILES_DIR"
  local d
  for d in "${PROFILES_DIR}"/*/; do
    [[ -d "$d" ]] || continue
    basename "$d"
  done
}

profile_count() {
  profile_list | wc -l
}

profile_import() {
  local conf_path="" name="" dns="" mtu=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -n|--name) name="$2"; shift 2 ;;
      --dns)     dns="$2"; shift 2 ;;
      --mtu)     mtu="$2"; shift 2 ;;
      -*)        return 1 ;;
      *)         conf_path="$1"; shift ;;
    esac
  done

  [[ -z "$conf_path" || -z "$name" ]] && return 1
  [[ -f "$conf_path" ]] || return 2

  local dir; dir=$(profile_dir "$name")
  mkdir -p "$dir"

  # Copy config
  cp "$conf_path" "${dir}/wg.conf"

  # ── Rewrite config to prevent host route hijack ────────────
  # Table = off → wg-quick won't touch host routing table
  if grep -q "^Table\s*=" "${dir}/wg.conf"; then
    sed -i 's/^Table\s*=.*/Table = off/' "${dir}/wg.conf"
  else
    sed -i '/^\[Interface\]/a Table = off' "${dir}/wg.conf"
  fi

  # SaveConfig = false → don't overwrite on shutdown
  if grep -q "^SaveConfig\s*=" "${dir}/wg.conf"; then
    sed -i 's/^SaveConfig\s*=.*/SaveConfig = false/' "${dir}/wg.conf"
  else
    sed -i '/^\[Interface\]/a SaveConfig = false' "${dir}/wg.conf"
  fi

  # DNS: keep if resolvconf is available, strip if not
  if ! command -v resolvconf &>/dev/null; then
    sed -i '/^DNS\s*=/d' "${dir}/wg.conf"
  fi

  # MTU override
  if [[ -n "$mtu" ]]; then
    if grep -q "^MTU\s*=" "${dir}/wg.conf"; then
      sed -i "s/^MTU\s*=.*/MTU = ${mtu}/" "${dir}/wg.conf"
    else
      sed -i '/^\[Interface\]/a MTU = '"$mtu" "${dir}/wg.conf"
    fi
  fi

  # DNS override
  if [[ -n "$dns" ]]; then
    echo "$dns" > "${dir}/dns"
  fi

  # Metadata
  local endpoint
  endpoint=$(sed -n 's/^Endpoint\s*=\s*//p' "${dir}/wg.conf" | head -1 || true)
  cat > "${dir}/meta.json" <<-META
{
  "name": "${name}",
  "imported_at": "$(date -Iseconds)",
  "source": "$(realpath "$conf_path")",
  "endpoint": "${endpoint}"
}
META

  return 0
}

profile_remove() {
  local name="$1"
  profile_exists "$name" || return 1
  rm -rf "$(profile_dir "$name")"
}

profile_endpoint() {
  local name="$1"
  profile_exists "$name" || return 1
  sed -n 's/^Endpoint\s*=\s*//p' "$(profile_conf "$name")" | head -1 || echo "-"
}

profile_dns_servers() {
  local name="$1"
  local dns_file; dns_file=$(profile_dns "$name")
  if [[ -f "$dns_file" ]]; then
    cat "$dns_file"
  else
    sed -n 's/^DNS\s*=\s*//p' "$(profile_conf "$name")" | head -1 || echo ""
  fi
}
