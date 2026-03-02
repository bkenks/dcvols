#!/usr/bin/env sh
set -e

REPO="bkenks/dcvols"
BINARY_NAME="dcvols"
USER_INSTALL_DIR="$HOME/.local/bin"

if [ -w /usr/local/bin ]; then
    SYSTEM_INSTALL_DIR="/usr/local/bin"
    SUDO=""
elif command -v sudo >/dev/null 2>&1; then
    SYSTEM_INSTALL_DIR="/usr/local/bin"
    SUDO="sudo"
else
    SYSTEM_INSTALL_DIR=""
    SUDO=""
fi

die() {
    printf "\033[31mError: %s\033[0m\n" "$1" >&2
    exit 1
}

info() {
    printf "\033[32m%s\033[0m\n" "$1"
}

command -v go >/dev/null 2>&1 || die "Go is required but not installed. Install it from https://go.dev/dl/"
command -v git >/dev/null 2>&1 || die "git is required but not installed."

info "Cloning dcvols repository..."
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

git clone --depth=1 "https://github.com/${REPO}.git" "$TMP_DIR/src" >/dev/null 2>&1

info "Building dcvols..."
cd "$TMP_DIR/src"
CGO_ENABLED=0 go build -o "$TMP_DIR/$BINARY_NAME" . 2>&1 || die "Build failed."

mkdir -p "$USER_INSTALL_DIR"
cp "$TMP_DIR/$BINARY_NAME" "$USER_INSTALL_DIR/$BINARY_NAME"
chmod +x "$USER_INSTALL_DIR/$BINARY_NAME"
info "Installed to $USER_INSTALL_DIR/$BINARY_NAME"

if [ -n "$SYSTEM_INSTALL_DIR" ]; then
    $SUDO mkdir -p "$SYSTEM_INSTALL_DIR"
    $SUDO cp "$TMP_DIR/$BINARY_NAME" "$SYSTEM_INSTALL_DIR/$BINARY_NAME"
    $SUDO chmod +x "$SYSTEM_INSTALL_DIR/$BINARY_NAME"
    info "Installed to $SYSTEM_INSTALL_DIR/$BINARY_NAME"
fi

case ":$PATH:" in
    *":$USER_INSTALL_DIR:"*) ;;
    *)
        printf "\n\033[33mAdd this to your shell config (~/.bashrc, ~/.zshrc, etc.):\033[0m\n"
        printf '  export PATH="%s:$PATH"\n\n' "$USER_INSTALL_DIR"
        ;;
esac
