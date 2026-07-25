#!/usr/bin/env bash
# test_helper/common.bash — Shared setup for all test files

SCRIPT_DIR="$(cd "$(dirname "${BATS_TEST_FILENAME}")/.." && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

# Create isolated temp dir per test
setup_tmpdir() {
  TEST_TMPDIR="$(mktemp -d)"
  export TEST_TMPDIR
}

teardown_tmpdir() {
  [[ -n "${TEST_TMPDIR:-}" && -d "$TEST_TMPDIR" ]] && rm -rf "$TEST_TMPDIR"
}

# Point profiles to temp dir
set_test_profiles() {
  export OC_VPN_PROFILES_DIR="${TEST_TMPDIR}/profiles"
  export PROFILES_DIR="${TEST_TMPDIR}/profiles"
  mkdir -p "$PROFILES_DIR"
}

# Generate a minimal valid WireGuard config
make_wg_conf() {
  local out="${1:?need output path}"
  cat > "$out" <<-CONF
[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.104.1.2/32
DNS = 1.1.1.1
MTU = 1420
Table = auto

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
Endpoint = 162.159.192.1:2
PersistentKeepalive = 25
CONF
}

# Generate a WireGuard config with extra wg-quick fields
make_wg_conf_with_extras() {
  local out="${1:?need output path}"
  cat > "$out" <<-CONF
[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.104.1.2/32
DNS = 1.1.1.1, 8.8.8.8
MTU = 1280
Table = auto
SaveConfig = false
PreUp = echo starting
PostDown = echo stopping

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 198.51.100.1:51820
PersistentKeepalive = 25
CONF
}

# Source a lib file with test profiles set
source_profiles() {
  set_test_profiles
  source "${LIB_DIR}/profiles.sh"
  PROFILES_DIR="${OC_VPN_PROFILES_DIR}"
}
