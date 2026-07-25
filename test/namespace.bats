#!/usr/bin/env bats
# test/namespace.bats — Unit tests for lib/namespace.sh

load test_helper/common

setup() {
  setup_tmpdir
  source "${LIB_DIR}/namespace.sh"
}

teardown() {
  teardown_tmpdir
}

# ── ns_name ───────────────────────────────────────────────────

@test "ns_name generates correct namespace name" {
  run ns_name "us-east"
  [[ "$output" == "ocvpn-us-east" ]]
}

@test "ns_name handles simple names" {
  run ns_name "test"
  [[ "$output" == "ocvpn-test" ]]
}

@test "ns_name uses ocvpn prefix" {
  run ns_name "anything"
  [[ "$output" == ocvpn-* ]]
}

# ── ns_exists (mocked) ────────────────────────────────────────

@test "ns_exists returns false when namespace does not exist" {
  # Without root, ip netns list will just return empty or error
  # The function should return non-zero for nonexistent namespace
  run ns_exists "definitely-does-not-exist-$$"
  [[ "$status" -ne 0 ]]
}
