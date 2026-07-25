#!/usr/bin/env bash
set -euo pipefail

# oc-vpn installer
# Usage: curl -fsSL https://raw.githubusercontent.com/lvluu/oc-vpn/main/install.sh | bash
#
# Override version:  VERSION=1.2.3 curl -fsSL ... | bash
# Override dest:     INSTALL_DIR=~/bin curl -fsSL ... | bash

REPO="lvluu/oc-vpn"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="oc-vpn"

detect_arch() {
  local arch
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64)   echo "amd64" ;;
    aarch64|arm64)  echo "arm64" ;;
    *)
      echo "error: unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac
}

detect_os() {
  local os
  os=$(uname -s)
  case "$os" in
    Linux)  echo "linux" ;;
    *)
      echo "error: unsupported OS: $os (oc-vpn requires Linux)" >&2
      exit 1
      ;;
  esac
}

resolve_version() {
  if [ "$VERSION" = "latest" ]; then
    local tag
    tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
      | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    if [ -z "$tag" ]; then
      echo "error: could not determine latest version" >&2
      exit 1
    fi
    echo "$tag"
  else
    echo "$VERSION"
  fi
}

main() {
  local os arch version url tmpdir

  os=$(detect_os)
  arch=$(detect_arch)
  version=$(resolve_version)
  version_clean="${version#v}"

  local asset="${BINARY}_${version_clean}_${os}_${arch}.tar.gz"
  url="https://github.com/$REPO/releases/download/${version}/${asset}"

  echo "Installing oc-vpn ${version} (${os}/${arch})..."

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  echo "Downloading $url..."
  curl -fsSL "$url" -o "$tmpdir/$asset"

  echo "Extracting..."
  tar -xzf "$tmpdir/$asset" -C "$tmpdir"

  echo "Installing to $INSTALL_DIR/$BINARY..."
  sudo install -m 755 "$tmpdir/$BINARY" "$INSTALL_DIR/$BINARY"

  echo ""
  echo "Installed: $INSTALL_DIR/$BINARY"
  "$INSTALL_DIR/$BINARY" version
  echo ""
  echo "Quick start:"
  echo "  sudo oc-vpn import <wireguard.conf> -n my-vpn"
  echo "  sudo oc-vpn up my-vpn"
}

main "$@"
