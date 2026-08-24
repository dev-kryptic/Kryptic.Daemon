#!/usr/bin/env bash
# Installs the kryptic CLI + daemon LaunchAgent for the current user.
# Usage: ./install.sh [path-to-kryptic-binary]   (defaults to ./kryptic)
set -euo pipefail

BINARY="${1:-./kryptic}"
PLIST_SOURCE="$(cd "$(dirname "$0")" && pwd)/dev.kryptic.daemon.plist"
AGENTS="$HOME/Library/LaunchAgents"

if [[ ! -f "$BINARY" ]]; then
  echo "kryptic binary not found at $BINARY" >&2
  exit 1
fi

echo "Installing kryptic to /usr/local/bin (may prompt for sudo)…"
launchctl unload "$AGENTS/dev.kryptic.daemon.plist" 2>/dev/null || true
kryptic stop 2>/dev/null || true
sudo install -m 755 "$BINARY" /usr/local/bin/kryptic

echo "Installing LaunchAgent…"
mkdir -p "$AGENTS"
cp "$PLIST_SOURCE" "$AGENTS/dev.kryptic.daemon.plist"

# Reload cleanly whether or not it was already loaded.
launchctl unload "$AGENTS/dev.kryptic.daemon.plist" 2>/dev/null || true
launchctl load "$AGENTS/dev.kryptic.daemon.plist"

echo
echo "Done. The daemon starts at login and is running now."
echo "Existing sign-in is kept. If this is a first install, run: kryptic login"
