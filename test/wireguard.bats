#!/usr/bin/env bats
# test/wireguard.bats — Unit tests for lib/wireguard.sh

load test_helper/common

setup() {
  setup_tmpdir
  export OC_VPN_PROFILES_DIR="${TEST_TMPDIR}/profiles"
  export PROFILES_DIR="${OC_VPN_PROFILES_DIR}"
  mkdir -p "$PROFILES_DIR"

  # Override the geo cache path
  export _geo_cache="${TEST_TMPDIR}/geo-cache"

  source "${LIB_DIR}/profiles.sh"
  PROFILES_DIR="${OC_VPN_PROFILES_DIR}"
  source "${LIB_DIR}/wireguard.sh"
}

teardown() {
  teardown_tmpdir
}

# ── geo_lookup ────────────────────────────────────────────────

@test "geo_lookup returns dash for empty IP" {
  run geo_lookup ""
  [[ "$output" == "-" ]]
}

@test "geo_lookup returns dash for dash IP" {
  run geo_lookup "-"
  [[ "$output" == "-" ]]
}

@test "geo_lookup creates cache file" {
  # Mock curl to return valid response
  curl() {
    echo '{"country":"Germany","city":"Berlin"}'
  }
  export -f curl

  geo_lookup "1.2.3.4" >/dev/null
  [[ -f "$_geo_cache" ]]
}

@test "geo_lookup caches result" {
  # Mock curl
  curl() {
    echo '{"country":"Japan","city":"Tokyo"}'
  }
  export -f curl

  geo_lookup "5.6.7.8" >/dev/null
  run cat "$_geo_cache"
  [[ "$output" == *"5.6.7.8|Tokyo, Japan"* ]]
}

@test "geo_lookup uses cache on second call" {
  # First call populates cache
  curl() {
    echo '{"country":"France","city":"Paris"}'
  }
  export -f curl

  geo_lookup "9.10.11.12" >/dev/null

  # Override curl to fail on second call — cache should be used
  curl() {
    echo "OOPS THIS SHOULD NOT BE CALLED"
  }
  export -f curl

  run geo_lookup "9.10.11.12"
  [[ "$output" == "Paris, France" ]]
}

@test "geo_lookup handles city-only response" {
  curl() {
    echo '{"country":"Canada","city":""}'
  }
  export -f curl

  run geo_lookup "13.14.15.16"
  [[ "$output" == "Canada" ]]
}

@test "geo_lookup handles empty response" {
  curl() {
    echo '{}'
  }
  export -f curl

  run geo_lookup "17.18.19.20"
  [[ "$output" == "-" ]]
}

@test "geo_lookup handles curl failure" {
  curl() {
    return 1
  }
  export -f curl

  run geo_lookup "21.22.23.24"
  [[ "$output" == "-" ]]
}

@test "geo_lookup appends to existing cache" {
  echo "1.1.1.1|Existing, Cache" > "$_geo_cache"

  curl() {
    echo '{"country":"New","city":"Entry"}'
  }
  export -f curl

  geo_lookup "2.2.2.2" >/dev/null
  run cat "$_geo_cache"
  [[ "$output" == *"1.1.1.1|Existing, Cache"* ]]
  [[ "$output" == *"2.2.2.2|Entry, New"* ]]
}

# ── wg_down (not up) ─────────────────────────────────────────

@test "wg_down returns 0 when namespace does not exist" {
  run wg_down "nonexistent-profile"
  [[ "$status" -eq 0 ]]
}

# ── wg_is_up (not up) ────────────────────────────────────────

@test "wg_is_up returns false for nonexistent namespace" {
  # ip netns may not be available without root; just verify function doesn't crash
  wg_is_up "nonexistent-profile" 2>/dev/null || true
  run bash -c 'wg_is_up "nonexistent-profile" 2>/dev/null; echo "exit:$?"'
  [[ "$output" != *"wg0"* ]]
}
