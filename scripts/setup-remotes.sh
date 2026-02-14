#!/usr/bin/env bash
# Setup git remotes for the makingmusic/glow fork.
# Run once after cloning on a new machine.

set -euo pipefail

# Ensure origin points to the fork
ORIGIN_URL=$(git remote get-url origin 2>/dev/null || echo "")
if [[ "$ORIGIN_URL" != *"makingmusic/glow"* ]]; then
  echo "Warning: origin does not point to makingmusic/glow — skipping."
  echo "  origin = $ORIGIN_URL"
  exit 1
fi

# Add upstream if it doesn't exist
if git remote get-url upstream &>/dev/null; then
  echo "upstream remote already exists."
else
  git remote add upstream https://github.com/charmbracelet/glow
  echo "Added upstream remote (charmbracelet/glow)."
fi

# Disable push to upstream
git remote set-url --push upstream DISABLED
echo "Push to upstream disabled."

echo ""
echo "Remotes configured:"
git remote -v
