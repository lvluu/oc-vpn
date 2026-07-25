#!/usr/bin/env bats
# test/profiles.bats — Unit tests for lib/profiles.sh

load test_helper/common

setup() {
  setup_tmpdir
  source_profiles
}

teardown() {
  teardown_tmpdir
}

# ── Path helpers ──────────────────────────────────────────────

@test "profile_dir returns correct path" {
  run profile_dir "us-east"
  [[ "$output" == "${PROFILES_DIR}/us-east" ]]
}

@test "profile_conf returns wg.conf path" {
  run profile_conf "us-east"
  [[ "$output" == "${PROFILES_DIR}/us-east/wg.conf" ]]
}

@test "profile_meta returns meta.json path" {
  run profile_meta "us-east"
  [[ "$output" == "${PROFILES_DIR}/us-east/meta.json" ]]
}

@test "profile_dns returns dns path" {
  run profile_dns "us-east"
  [[ "$output" == "${PROFILES_DIR}/us-east/dns" ]]
}

# ── profile_exists ────────────────────────────────────────────

@test "profile_exists returns false for nonexistent profile" {
  run profile_exists "nonexistent"
  [[ "$status" -ne 0 ]]
}

@test "profile_exists returns true for existing profile" {
  mkdir -p "${PROFILES_DIR}/test-profile"
  run profile_exists "test-profile"
  [[ "$status" -eq 0 ]]
}

# ── profile_list ──────────────────────────────────────────────

@test "profile_list returns empty when no profiles exist" {
  run profile_list
  [[ -z "$output" ]]
}

@test "profile_list returns all profile names" {
  mkdir -p "${PROFILES_DIR}/alpha" "${PROFILES_DIR}/bravo" "${PROFILES_DIR}/charlie"
  run profile_list
  [[ "$output" == *"alpha"* ]]
  [[ "$output" == *"bravo"* ]]
  [[ "$output" == *"charlie"* ]]
}

@test "profile_list ignores files" {
  mkdir -p "${PROFILES_DIR}/valid-profile"
  touch "${PROFILES_DIR}/not-a-profile"
  run profile_list
  [[ "$output" == *"valid-profile"* ]]
  [[ "$output" != *"not-a-profile"* ]]
}

# ── profile_count ─────────────────────────────────────────────

@test "profile_count returns 0 when empty" {
  run profile_count
  [[ "$output" =~ ^[[:space:]]*0[[:space:]]*$ ]] || [[ "$output" == "0" ]]
}

@test "profile_count returns correct count" {
  mkdir -p "${PROFILES_DIR}/a" "${PROFILES_DIR}/b" "${PROFILES_DIR}/c"
  run profile_count
  [[ "$output" =~ 3 ]]
}

# ── profile_import ────────────────────────────────────────────

@test "profile_import creates profile directory" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  profile_import "$conf" -n "test-vpn"
  [[ -d "${PROFILES_DIR}/test-vpn" ]]
  [[ -f "${PROFILES_DIR}/test-vpn/wg.conf" ]]
  rm -f "$conf"
}

@test "profile_import creates meta.json with endpoint" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  profile_import "$conf" -n "test-vpn"
  [[ -f "${PROFILES_DIR}/test-vpn/meta.json" ]]
  grep -q "162.159.192.1:2" "${PROFILES_DIR}/test-vpn/meta.json"
  rm -f "$conf"
}

@test "profile_import strips Address from wg.conf" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  profile_import "$conf" -n "test-vpn"
  run grep -c "^Address" "${PROFILES_DIR}/test-vpn/wg.conf"
  [[ "$output" == "0" ]]
  rm -f "$conf"
}

@test "profile_import strips DNS from wg.conf" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  profile_import "$conf" -n "test-vpn"
  run grep -c "^DNS" "${PROFILES_DIR}/test-vpn/wg.conf"
  [[ "$output" == "0" ]]
  rm -f "$conf"
}

@test "profile_import strips MTU from wg.conf" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  profile_import "$conf" -n "test-vpn"
  run grep -c "^MTU" "${PROFILES_DIR}/test-vpn/wg.conf"
  [[ "$output" == "0" ]]
  rm -f "$conf"
}

@test "profile_import strips Table from wg.conf" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  profile_import "$conf" -n "test-vpn"
  run grep -c "^Table" "${PROFILES_DIR}/test-vpn/wg.conf"
  [[ "$output" == "0" ]]
  rm -f "$conf"
}

@test "profile_import strips SaveConfig from wg.conf" {
  local conf; conf=$(mktemp)
  make_wg_conf_with_extras "$conf"

  profile_import "$conf" -n "test-vpn"
  run grep -c "^SaveConfig" "${PROFILES_DIR}/test-vpn/wg.conf"
  [[ "$output" == "0" ]]
  rm -f "$conf"
}

@test "profile_import strips PreUp from wg.conf" {
  local conf; conf=$(mktemp)
  make_wg_conf_with_extras "$conf"

  profile_import "$conf" -n "test-vpn"
  run grep -c "^PreUp" "${PROFILES_DIR}/test-vpn/wg.conf"
  [[ "$output" == "0" ]]
  rm -f "$conf"
}

@test "profile_import strips PostDown from wg.conf" {
  local conf; conf=$(mktemp)
  make_wg_conf_with_extras "$conf"

  profile_import "$conf" -n "test-vpn"
  run grep -c "^PostDown" "${PROFILES_DIR}/test-vpn/wg.conf"
  [[ "$output" == "0" ]]
  rm -f "$conf"
}

@test "profile_import preserves PrivateKey" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  profile_import "$conf" -n "test-vpn"
  grep -q "^PrivateKey" "${PROFILES_DIR}/test-vpn/wg.conf"
  rm -f "$conf"
}

@test "profile_import preserves [Peer] section" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  profile_import "$conf" -n "test-vpn"
  grep -q "^\[Peer\]" "${PROFILES_DIR}/test-vpn/wg.conf"
  grep -q "PublicKey" "${PROFILES_DIR}/test-vpn/wg.conf"
  grep -q "AllowedIPs" "${PROFILES_DIR}/test-vpn/wg.conf"
  rm -f "$conf"
}

@test "profile_import returns 1 when missing name" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  run profile_import "$conf"
  [[ "$status" -eq 1 ]]
  rm -f "$conf"
}

@test "profile_import returns 1 when missing config path" {
  run profile_import -n "test-vpn"
  [[ "$status" -eq 1 ]]
}

@test "profile_import returns 2 when config file not found" {
  run profile_import "/nonexistent/file.conf" -n "test-vpn"
  [[ "$status" -eq 2 ]]
}

@test "profile_import returns 1 on unknown flag" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  run profile_import "$conf" --bogus "value"
  [[ "$status" -eq 1 ]]
  rm -f "$conf"
}

# ── profile_remove ────────────────────────────────────────────

@test "profile_remove deletes profile directory" {
  mkdir -p "${PROFILES_DIR}/to-delete"
  touch "${PROFILES_DIR}/to-delete/wg.conf"

  profile_remove "to-delete"
  [[ ! -d "${PROFILES_DIR}/to-delete" ]]
}

@test "profile_remove returns 1 for nonexistent profile" {
  run profile_remove "nonexistent"
  [[ "$status" -eq 1 ]]
}

# ── profile_endpoint ──────────────────────────────────────────

@test "profile_endpoint returns endpoint from config" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"
  profile_import "$conf" -n "test-vpn"

  run profile_endpoint "test-vpn"
  [[ "$output" == "162.159.192.1:2" ]]
  rm -f "$conf"
}

@test "profile_endpoint returns empty when no endpoint" {
  mkdir -p "${PROFILES_DIR}/no-endpoint"
  echo -e "[Interface]\nPrivateKey = test" > "${PROFILES_DIR}/no-endpoint/wg.conf"

  run profile_endpoint "no-endpoint"
  [[ -z "$output" ]]
}

@test "profile_endpoint returns 1 for nonexistent profile" {
  run profile_endpoint "nonexistent"
  [[ "$status" -eq 1 ]]
}
