#!/usr/bin/env bash
# install.sh — Symlink oc-vpn to /usr/local/bin

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="/usr/local/bin/oc-vpn"

if [[ $EUID -ne 0 ]]; then
  echo "error: must run as root" >&2
  echo "usage: sudo ./install.sh" >&2
  exit 1
fi

ln -sf "${SCRIPT_DIR}/bin/oc-vpn" "$TARGET"
chmod +x "${SCRIPT_DIR}/bin/oc-vpn"

echo "✓ Installed: ${TARGET}"
echo "  Run: oc-vpn --help"
