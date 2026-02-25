#!/bin/sh
set -e

REPO="paymonp/nanotown"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="nt"

# Detect OS
OS=$(uname -s)
case "$OS" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *)      echo "Error: unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)       echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

# Fetch latest release tag
echo "Fetching latest release..."
if command -v curl >/dev/null 2>&1; then
    TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//')
elif command -v wget >/dev/null 2>&1; then
    TAG=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//')
else
    echo "Error: curl or wget is required"; exit 1
fi

if [ -z "$TAG" ]; then
    echo "Error: could not determine latest release tag"; exit 1
fi

ASSET="nanotown-${OS}-${ARCH}"
URL="https://github.com/$REPO/releases/download/$TAG/$ASSET"

echo "Downloading $ASSET ($TAG)..."
mkdir -p "$INSTALL_DIR"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$INSTALL_DIR/$BINARY_NAME" "$URL"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$INSTALL_DIR/$BINARY_NAME" "$URL"
fi

chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo "Installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"

# Check if install dir is in PATH
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo ""
        echo "NOTE: $INSTALL_DIR is not in your PATH."
        echo "Add it by appending this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo ""
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
        ;;
esac
