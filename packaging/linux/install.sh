#!/usr/bin/env bash
# Installs the kryptic CLI + systemd user service for the current user.
# Usage: ./install.sh [path-to-kryptic-binary]   (defaults to ./kryptic)
set -euo pipefail

BINARY="${1:-./kryptic}"
UNIT_SOURCE="$(cd "$(dirname "$0")" && pwd)/kryptic-daemon.service"

if [[ ! -f "$BINARY" ]]; then
  echo "kryptic binary not found at $BINARY" >&2
  exit 1
fi

echo "Installing kryptic to ~/.local/bin…"
install -Dm755 "$BINARY" "$HOME/.local/bin/kryptic"

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemd not found - start the daemon manually with: kryptic start &"
  exit 0
fi

echo "Installing systemd user service…"
install -Dm644 "$UNIT_SOURCE" "$HOME/.config/systemd/user/kryptic-daemon.service"
systemctl --user daemon-reload
systemctl --user enable --now kryptic-daemon

echo
echo "Done. The daemon starts at login and is running now."
echo "Tip: install 'secret-tool' (libsecret) so the session token lives in your keyring."
echo "Next: kryptic login"
