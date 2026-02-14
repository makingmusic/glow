#!/usr/bin/env bash
set -euo pipefail

# Uninstall glow binary installed via install.sh.
# Usage: ./uninstall.sh

GOBIN="$(go env GOPATH)/bin"
GLOW_BIN="${GOBIN}/glow"

if [[ -f "$GLOW_BIN" ]]; then
    rm "$GLOW_BIN"
    echo "Removed ${GLOW_BIN}"
else
    echo "glow binary not found at ${GLOW_BIN} — nothing to remove."
    exit 1
fi
