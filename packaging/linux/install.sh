#!/usr/bin/env bash
# Installs the kryptic CLI, tray app, desktop entry, and systemd user service
# for the current user (no root required).
# Usage: ./install.sh [path-to-kryptic-binary]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
UNIT_SOURCE="$SCRIPT_DIR/kryptic-daemon.service"
DESKTOP_SOURCE="$SCRIPT_DIR/dev.kryptic.Kryptic.desktop"

linux_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) uname -m ;;
  esac
}

ARCH="$(linux_arch)"

find_cli() {
  if [[ -n "${1:-}" && -f "$1" ]]; then
    echo "$1"
    return 0
  fi
  local candidate
  for candidate in \
    "./kryptic_linux_${ARCH}" \
    ./kryptic \
    ./kryptic_linux_amd64 \
    ./kryptic_linux_arm64
  do
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
    "$cli_dir/kryptic-tray_linux_${ARCH}" \
    "$cli_dir/kryptic-tray" \
    "./kryptic-tray_linux_${ARCH}" \
    ./kryptic-tray
  do
    if [[ -f "$candidate" ]]; then
      echo "$candidate"
      return 0
    fi
  done
  return 1
}

binary_arch() {
  local file="$1"
  if command -v file >/dev/null 2>&1; then
    case "$(file -b "$file")" in
      *x86-64*|*x86_64*) echo amd64 ;;
      *aarch64*|*arm64*) echo arm64 ;;
      *) echo unknown ;;
    esac
    return
  fi
  echo unknown
}

CLI="$(find_cli "${1:-}" || true)"
if [[ -z "$CLI" ]]; then
  echo "kryptic binary not found. Pass the path: $0 /path/to/kryptic" >&2
  echo "This machine is ${ARCH}. Copy kryptic_linux_${ARCH} and kryptic-tray_linux_${ARCH}." >&2
  exit 1
fi

CLI_ARCH="$(binary_arch "$CLI")"
if [[ "$CLI_ARCH" != unknown && "$CLI_ARCH" != "$ARCH" ]]; then
  echo "$CLI is $CLI_ARCH, this machine is $ARCH. Copy kryptic_linux_${ARCH} instead." >&2
  exit 1
fi

TRAY="$(find_tray "$CLI" || true)"
if [[ -n "$TRAY" ]]; then
  TRAY_ARCH="$(binary_arch "$TRAY")"
  if [[ "$TRAY_ARCH" != unknown && "$TRAY_ARCH" != "$ARCH" ]]; then
    echo "$TRAY is $TRAY_ARCH, this machine is $ARCH. Copy kryptic-tray_linux_${ARCH} instead." >&2
    exit 1
  fi
fi

ICON_PNG="$SCRIPT_DIR/kryptic.png"
ICON_SVG="$SCRIPT_DIR/kryptic.svg"

BINDIR="$HOME/.local/bin"
APPDIR="$HOME/.local/share/applications"
AUTOSTART="$HOME/.config/autostart"
ICONDIR="$HOME/.local/share/icons/hicolor/256x256/apps"
SCALEDIR="$HOME/.local/share/icons/hicolor/scalable/apps"
PIXMAP="$HOME/.local/share/pixmaps"

echo "Installing kryptic to $BINDIR…"
systemctl --user stop kryptic-daemon 2>/dev/null || true
pkill -x kryptic-tray 2>/dev/null || true
pkill -x kryptic 2>/dev/null || true

mkdir -p "$BINDIR"
install -m755 "$CLI" "$BINDIR/kryptic"

if [[ -n "$TRAY" ]]; then
  install -m755 "$TRAY" "$BINDIR/kryptic-tray"
else
  echo "Tray binary not found next to $CLI. Installing CLI only."
  echo "Copy kryptic-tray_linux_${ARCH} into this folder for the desktop app."
fi

ICON_VALUE=kryptic
if [[ -f "$ICON_PNG" ]]; then
  install -Dm644 "$ICON_PNG" "$ICONDIR/kryptic.png"
  install -Dm644 "$ICON_PNG" "$PIXMAP/kryptic.png"
  ICON_VALUE="$ICONDIR/kryptic.png"
fi
if [[ -f "$ICON_SVG" ]]; then
  install -Dm644 "$ICON_SVG" "$SCALEDIR/kryptic.svg"
  install -Dm644 "$ICON_SVG" "$PIXMAP/kryptic.svg"
  if [[ "$ICON_VALUE" == "kryptic" ]]; then
    ICON_VALUE="$SCALEDIR/kryptic.svg"
  fi
fi

if [[ -f "$DESKTOP_SOURCE" && -n "$TRAY" ]]; then
  mkdir -p "$APPDIR" "$AUTOSTART"
  # User-local installs are not on a default PATH for .desktop Exec= lookups
  # on every distro, so pin the tray path. Pin Icon= the same way: a named
  # lookup of "kryptic" is a generic gear when the theme has no such icon.
  sed -e "s#^Exec=kryptic-tray\$#Exec=$BINDIR/kryptic-tray#" \
      -e "s#^TryExec=kryptic-tray\$#TryExec=$BINDIR/kryptic-tray#" \
      -e "s#^Icon=kryptic\$#Icon=$ICON_VALUE#" \
      "$DESKTOP_SOURCE" > "$APPDIR/dev.kryptic.Kryptic.desktop"
  chmod 0644 "$APPDIR/dev.kryptic.Kryptic.desktop"
  install -m644 "$APPDIR/dev.kryptic.Kryptic.desktop" \
    "$AUTOSTART/dev.kryptic.Kryptic.desktop"
fi

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$APPDIR" 2>/dev/null || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q "$HOME/.local/share/icons/hicolor" 2>/dev/null || true
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemd not found - start the daemon manually with: kryptic start &"
elif [[ -f "$UNIT_SOURCE" ]]; then
  echo "Installing systemd user service…"
  mkdir -p "$HOME/.config/systemd/user"
  install -m644 "$UNIT_SOURCE" "$HOME/.config/systemd/user/kryptic-daemon.service"
  systemctl --user daemon-reload
  systemctl --user enable --now kryptic-daemon
else
  echo "kryptic-daemon.service not next to install.sh. Skipping systemd; the tray runs the daemon itself."
fi

if [[ -n "$TRAY" && -x "$BINDIR/kryptic-tray" && -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]]; then
  nohup "$BINDIR/kryptic-tray" >/dev/null 2>&1 &
  disown || true
  echo "Kryptic tray is running. It also starts at login."
elif [[ -n "$TRAY" ]]; then
  echo "Launch the desktop app with: $BINDIR/kryptic-tray"
fi

echo
echo "Done. Existing sign-in is kept. If this is a first install, run: kryptic login"
echo "Tip: install 'secret-tool' (libsecret) so the session token lives in your keyring."
echo "Tip: install 'python3-gi' and 'gir1.2-gtk-3.0' so Open Kryptic matches macOS/Windows."
echo "Tip: install 'yad' or 'zenity' so Server URI and other dialogs have the three buttons."
