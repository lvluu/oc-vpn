#!/usr/bin/env bats
# test/cli.bats — Unit tests for bin/oc-vpn

load test_helper/common

OC_VPN="${SCRIPT_DIR}/bin/oc-vpn"

setup() {
  setup_tmpdir
  export OC_VPN_PROFILES_DIR="${TEST_TMPDIR}/profiles"
  export PROFILES_DIR="${OC_VPN_PROFILES_DIR}"
  mkdir -p "$PROFILES_DIR"
}

teardown() {
  teardown_tmpdir
}

# ── Help & version ───────────────────────────────────────────

@test "oc-vpn --help shows usage" {
  run "$OC_VPN" --help
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"oc-vpn"* ]]
  [[ "$output" == *"USAGE"* ]]
  [[ "$output" == *"COMMANDS"* ]]
}

@test "oc-vpn -h shows usage" {
  run "$OC_VPN" -h
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"USAGE"* ]]
}

@test "oc-vpn help shows usage" {
  run "$OC_VPN" help
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"COMMANDS"* ]]
}

@test "oc-vpn --version shows version" {
  run "$OC_VPN" --version
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"0.1.0"* ]]
}

@test "oc-vpn -v shows version" {
  run "$OC_VPN" -v
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"oc-vpn 0.1.0"* ]]
}

# ── No args shows help ───────────────────────────────────────

@test "oc-vpn with no args shows usage (needs root for TUI)" {
  # Without root, will fail with "must run as root" or show usage
  run "$OC_VPN"
  [[ "$status" -ne 0 ]] || [[ "$output" == *"USAGE"* ]]
}

# ── Unknown command ───────────────────────────────────────────

@test "oc-vpn unknown command fails" {
  run "$OC_VPN" boguscommand
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"unknown command"* ]]
}

# ── Import validation ─────────────────────────────────────────

@test "oc-vpn import with no args fails" {
  run "$OC_VPN" import
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"usage:"* ]]
}

@test "oc-vpn import without name fails" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  run "$OC_VPN" import "$conf"
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"must specify"* ]]
  rm -f "$conf"
}

@test "oc-vpn import nonexistent file fails" {
  run "$OC_VPN" import /nonexistent/file.conf -n test
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"config file not found"* ]]
}

@test "oc-vpn import unknown flag fails" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  run "$OC_VPN" import "$conf" --bogus "val" -n test
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"unknown flag"* ]]
  rm -f "$conf"
}

@test "oc-vpn import succeeds with valid config" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"

  run "$OC_VPN" import "$conf" -n "test-vpn"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"imported"* ]]
  [[ -d "${PROFILES_DIR}/test-vpn" ]]
  rm -f "$conf"
}

# ── Remove validation ─────────────────────────────────────────

@test "oc-vpn remove with no args fails" {
  run "$OC_VPN" remove
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"usage:"* ]]
}

@test "oc-vpn remove nonexistent profile fails" {
  run "$OC_VPN" remove "nonexistent"
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"not found"* ]]
}

# ── Export ─────────────────────────────────────────────────────

@test "oc-vpn export with no args fails" {
  run "$OC_VPN" export
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"usage:"* ]]
}

@test "oc-vpn export nonexistent profile fails" {
  run "$OC_VPN" export "nonexistent"
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"not found"* ]]
}

@test "oc-vpn export shows config contents" {
  local conf; conf=$(mktemp)
  make_wg_conf "$conf"
  "$OC_VPN" import "$conf" -n "export-test" 2>/dev/null

  run "$OC_VPN" export "export-test"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"export-test"* ]]
  [[ "$output" == *"PrivateKey"* ]]
  rm -f "$conf"
}

# ── List ──────────────────────────────────────────────────────

@test "oc-vpn list works with no profiles" {
  run "$OC_VPN" list
  [[ "$status" -eq 0 ]]
}

# ── Commands requiring root ───────────────────────────────────

@test "oc-vpn up without root fails" {
  if [[ $EUID -eq 0 ]]; then
    skip "running as root"
  fi
  run "$OC_VPN" up "anything"
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"root"* ]]
}

@test "oc-vpn down without root fails" {
  if [[ $EUID -eq 0 ]]; then
    skip "running as root"
  fi
  run "$OC_VPN" down "anything"
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"root"* ]]
}

@test "oc-vpn status without root fails" {
  if [[ $EUID -eq 0 ]]; then
    skip "running as root"
  fi
  run "$OC_VPN" status
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"root"* ]]
}

@test "oc-vpn run without root fails" {
  if [[ $EUID -eq 0 ]]; then
    skip "running as root"
  fi
  run "$OC_VPN" run "anything"
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"root"* ]]
}
