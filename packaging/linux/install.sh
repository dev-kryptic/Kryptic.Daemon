#!/usr/bin/env bash
# Installs the kryptic CLI, tray app, desktop entry, and systemd user service
# for the current user (no root required).
# Usage: ./install.sh [path-to-kryptic-binary]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
UNIT_SOURCE="$SCRIPT_DIR/kryptic-daemon.service"
DESKTOP_SOURCE="$SCRIPT_DIR/dev.kryptic.Kryptic.desktop"

find_cli() {
  if [[ -n "${1:-}" && -f "$1" ]]; then
    echo "$1"
    return 0
  fi
  local candidate
  for candidate in ./kryptic ./kryptic_linux_amd64 ./kryptic_linux_arm64; do
    if [[ -f "$candidate" ]]; then
      echo "$candidate"
      return 0
    fi
  done
  return 1
}

find_tray() {
  local cli_dir
  cli_dir="$(cd "$(dirname "$1")" && pwd)"
  local candidate
  for candidate in \
    "$cli_dir/kryptic-tray" \
    "$cli_dir/kryptic-tray_linux_amd64" \
    "$cli_dir/kryptic-tray_linux_arm64" \
    ./kryptic-tray \
    ./kryptic-tray_linux_amd64 \
    ./kryptic-tray_linux_arm64
  do
    if [[ -f "$candidate" ]]; then
      echo "$candidate"
      return 0
    fi
  done
  return 1
}

CLI="$(find_cli "${1:-}" || true)"
if [[ -z "$CLI" ]]; then
  echo "kryptic binary not found. Pass the path: $0 /path/to/kryptic" >&2
  exit 1
fi

TRAY="$(find_tray "$CLI" || true)"
ICON_SRC="$REPO_ROOT/macos/Sources/KrypticDaemon/Resources/logo.png"
if [[ ! -f "$ICON_SRC" ]]; then
  ICON_SRC="$REPO_ROOT/cmd/kryptic-tray/assets/kryptic.png"
fi

BINDIR="$HOME/.local/bin"
APPDIR="$HOME/.local/share/applications"
AUTOSTART="$HOME/.config/autostart"
ICONDIR="$HOME/.local/share/icons/hicolor/256x256/apps"
PIXMAP="$HOME/.local/share/pixmaps"

echo "Installing kryptic to $BINDIR…"
systemctl --user stop kryptic-daemon 2>/dev/null || true
pkill -x kryptic-tray 2>/dev/null || true
pkill -x kryptic 2>/dev/null || true

install -Dm755 "$CLI" "$BINDIR/kryptic"

if [[ -n "$TRAY" ]]; then
  install -Dm755 "$TRAY" "$BINDIR/kryptic-tray"
else
  echo "Tray binary not found next to $CLI. Installing CLI and service only."
  echo "Download kryptic-tray_linux_$(uname -m) from https://kryptic.dev/download for the desktop app."
fi

if [[ -f "$DESKTOP_SOURCE" && -n "$TRAY" ]]; then
  mkdir -p "$APPDIR"
  # User-local installs are not on a default PATH for .desktop Exec= lookups
  # on every distro, so pin the tray path.
  sed -e "s#^Exec=kryptic-tray\$#Exec=$BINDIR/kryptic-tray#" \
      -e "s#^TryExec=kryptic-tray\$#TryExec=$BINDIR/kryptic-tray#" \
      "$DESKTOP_SOURCE" > "$APPDIR/dev.kryptic.Kryptic.desktop"
  chmod 0644 "$APPDIR/dev.kryptic.Kryptic.desktop"
  install -Dm644 "$APPDIR/dev.kryptic.Kryptic.desktop" \
    "$AUTOSTART/dev.kryptic.Kryptic.desktop"
fi

if [[ -f "$ICON_SRC" ]]; then
  install -Dm644 "$ICON_SRC" "$ICONDIR/kryptic.png"
  install -Dm644 "$ICON_SRC" "$PIXMAP/kryptic.png"
fi

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$APPDIR" 2>/dev/null || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q "$HOME/.local/share/icons/hicolor" 2>/dev/null || true
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemd not found - start the daemon manually with: kryptic start &"
else
  echo "Installing systemd user service…"
  install -Dm644 "$UNIT_SOURCE" "$HOME/.config/systemd/user/kryptic-daemon.service"
  systemctl --user daemon-reload
  systemctl --user enable --now kryptic-daemon
fi

if [[ -n "$TRAY" && -x "$BINDIR/kryptic-tray" && -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]]; then
  nohup "$BINDIR/kryptic-tray" >/dev/null 2>&1 &
  disown || true
  echo "Kryptic tray is running. It also starts at login."
elif [[ -n "$TRAY" ]]; then
  echo "Launch the desktop app with: $BINDIR/kryptic-tray"
fi

echo
echo "Done. The daemon starts at login and is running now."
echo "Tip: install 'secret-tool' (libsecret) so the session token lives in your keyring."
echo "Existing sign-in is kept. If this is a first install, run: kryptic login"
