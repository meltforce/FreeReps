#!/bin/bash
set -euo pipefail

# FreeReps Upload Tool Installer
# Install:   curl -sSL https://raw.githubusercontent.com/meltforce/FreeReps/main/scripts/install-upload.sh | bash
# Update:    curl -sSL ... | bash -s -- --update
# Uninstall: curl -sSL ... | bash -s -- --uninstall

REPO="meltforce/FreeReps"
BINARY_NAME="freereps-upload"
INSTALL_DIR="/usr/local/bin"
STATE_DIR="$HOME/.freereps-upload"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}==>${NC} $1"; }
warn()  { echo -e "${YELLOW}==>${NC} $1"; }
error() { echo -e "${RED}==>${NC} $1" >&2; }

detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        arm64|aarch64) echo "arm64" ;;
        x86_64|amd64)  echo "amd64" ;;
        *)
            error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

detect_os() {
    local os
    os=$(uname -s)
    case "$os" in
        Darwin) echo "darwin" ;;
        *)
            error "Unsupported OS: $os (freereps-upload is macOS only)"
            exit 1
            ;;
    esac
}

latest_version() {
    local version
    version=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' \
        | head -1 \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        error "Failed to determine latest version"
        exit 1
    fi
    echo "$version"
}

do_install() {
    local os arch version asset_name url

    os=$(detect_os)
    arch=$(detect_arch)
    version=$(latest_version)
    asset_name="${BINARY_NAME}-${os}-${arch}"

    info "Installing $BINARY_NAME $version ($os/$arch)..."

    url="https://github.com/$REPO/releases/download/$version/$asset_name"
    info "Downloading from $url"

    local tmpfile
    tmpfile=$(mktemp)
    trap "rm -f '$tmpfile'" EXIT

    if ! curl -sSL -o "$tmpfile" "$url"; then
        error "Download failed. Check that release $version has a $asset_name asset."
        exit 1
    fi

    chmod +x "$tmpfile"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$tmpfile" "$INSTALL_DIR/$BINARY_NAME"
    else
        info "Requesting sudo to install to $INSTALL_DIR..."
        sudo mv "$tmpfile" "$INSTALL_DIR/$BINARY_NAME"
    fi

    # Create state directory
    mkdir -p "$STATE_DIR"

    info "Installed $BINARY_NAME $version to $INSTALL_DIR/$BINARY_NAME"

    # Check for lzfse dependency
    if ! command -v lzfse &>/dev/null; then
        warn "lzfse is required but not installed."
        warn "Install it with: brew install lzfse"
    fi

    echo ""
    info "Usage:"
    echo "  $BINARY_NAME -server https://freereps.your-tailnet.ts.net -path /path/to/AutoSync"
    echo ""
    echo "  Typical iCloud path:"
    echo "  ~/Library/Mobile Documents/com~apple~CloudDocs/Health Auto Export/AutoSync"
}

do_update() {
    if ! command -v "$BINARY_NAME" &>/dev/null; then
        warn "$BINARY_NAME is not installed. Running install instead."
        do_install
        return
    fi

    local current_version
    current_version=$("$BINARY_NAME" -version 2>&1 | awk '{print $2}' || echo "unknown")
    info "Current version: $current_version"

    do_install
}

do_uninstall() {
    info "Uninstalling $BINARY_NAME..."

    local binary_path="$INSTALL_DIR/$BINARY_NAME"
    if [ -f "$binary_path" ]; then
        if [ -w "$INSTALL_DIR" ]; then
            rm -f "$binary_path"
        else
            sudo rm -f "$binary_path"
        fi
        info "Removed $binary_path"
    else
        warn "$binary_path not found"
    fi

    if [ -d "$STATE_DIR" ]; then
        read -rp "Remove state directory $STATE_DIR? [y/N] " confirm
        if [[ "$confirm" =~ ^[Yy]$ ]]; then
            rm -rf "$STATE_DIR"
            info "Removed $STATE_DIR"
        else
            info "Kept $STATE_DIR"
        fi
    fi

    info "Uninstall complete"
}

# Parse arguments
case "${1:-}" in
    --update)    do_update ;;
    --uninstall) do_uninstall ;;
    *)           do_install ;;
esac
