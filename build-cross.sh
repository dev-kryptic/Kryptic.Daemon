#!/usr/bin/env bash
set -euo pipefail

# Cross-compiles the kryptic CLI (all platforms) and the kryptic-tray app
# (Windows + Linux; macOS uses the SwiftUI app in macos/ instead).
# Everything is pure Go - no cgo - so this runs from any host.

ROOT="$(cd "$(dirname "$0")" && pwd)"
DIST="$ROOT/dist"
mkdir -p "$DIST"

build() {
  local goos="$1" goarch="$2" package="$3" output="$4"
  shift 4
  echo "  $output"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath "$@" -o "$DIST/$output" "$package"
}

echo "kryptic CLI…"
build darwin  arm64 ./cmd/kryptic kryptic_darwin_arm64
build darwin  amd64 ./cmd/kryptic kryptic_darwin_amd64
build linux   amd64 ./cmd/kryptic kryptic_linux_amd64
build linux   arm64 ./cmd/kryptic kryptic_linux_arm64
build windows amd64 ./cmd/kryptic kryptic_windows_amd64.exe

echo "kryptic-tray…"
build linux   amd64 ./cmd/kryptic-tray kryptic-tray_linux_amd64
build linux   arm64 ./cmd/kryptic-tray kryptic-tray_linux_arm64
# -H windowsgui: no console window behind the tray icon
build windows amd64 ./cmd/kryptic-tray kryptic-tray_windows_amd64.exe -ldflags "-H windowsgui"

echo "Done: $DIST"
