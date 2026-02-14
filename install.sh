#!/usr/bin/env bash
set -euo pipefail

# Install glow from local repo checkout.
# Usage: ./install.sh

# Resolve repo root relative to this script.
REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

# Embed version info from git.
VERSION="$(git -C "$REPO_DIR" describe --tags --always --dirty 2>/dev/null || echo "dev")"
COMMIT="$(git -C "$REPO_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")"

echo "Building glow ${VERSION} (${COMMIT})..."

cd "$REPO_DIR"
go install -ldflags "-X main.Version=${VERSION} -X main.CommitSHA=${COMMIT}" .

echo "Installed glow to $(go env GOPATH)/bin/glow"
echo ""

# Check if GOPATH/bin is in PATH.
GOBIN="$(go env GOPATH)/bin"
if [[ ":${PATH}:" != *":${GOBIN}:"* ]]; then
    echo "WARNING: ${GOBIN} is not in your PATH."
    echo "Add this to your shell profile (~/.zshrc):"
    echo ""
    echo "  export PATH=\"${GOBIN}:\$PATH\""
    echo ""
fi
