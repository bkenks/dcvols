#!/usr/bin/env sh
set -e

BINARY_NAME="dcvols"
USER_INSTALL_DIR="$HOME/.local/bin"
SYSTEM_INSTALL_DIR="/usr/local/bin"

if [ -w "$SYSTEM_INSTALL_DIR" ]; then
    SUDO=""
elif command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
else
    SUDO=""
fi

info() {
    printf "\033[32m%s\033[0m\n" "$1"
}

warn() {
    printf "\033[33m%s\033[0m\n" "$1"
}

removed=0

if [ -f "$USER_INSTALL_DIR/$BINARY_NAME" ]; then
    rm -f "$USER_INSTALL_DIR/$BINARY_NAME"
    info "Removed $USER_INSTALL_DIR/$BINARY_NAME"
    removed=1
fi

if [ -f "$SYSTEM_INSTALL_DIR/$BINARY_NAME" ]; then
    $SUDO rm -f "$SYSTEM_INSTALL_DIR/$BINARY_NAME"
    info "Removed $SYSTEM_INSTALL_DIR/$BINARY_NAME"
    removed=1
fi

if [ "$removed" -eq 0 ]; then
    warn "No dcvols binary found in $USER_INSTALL_DIR or $SYSTEM_INSTALL_DIR"
fi
