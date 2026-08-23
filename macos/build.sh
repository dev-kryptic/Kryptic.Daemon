#!/usr/bin/env bash
set -euo pipefail

# Usage: build.sh [--debug]
#   --debug builds a debug configuration app whose daemon targets the local
#   Daemon BFF (http://localhost:5211) instead of the hosted platform.

ROOT="$(cd "$(dirname "$0")" && pwd)"
APP_NAME="Kryptic"
CONFIGURATION="release"
if [[ "${1:-}" == "--debug" ]]; then
  CONFIGURATION="debug"
fi
BUILD_DIR="$ROOT/.build/$CONFIGURATION"
APP_BUNDLE="$ROOT/dist/${APP_NAME}.app"
RESOURCES="$ROOT/Sources/KrypticDaemon/Resources"

echo "Generating icons…"
swift "$ROOT/Scripts/GenerateIcons.swift" "$RESOURCES"

echo "Building Go daemon…"
# Release builds pass VERSION so the bundled CLI reports the tagged version.
# Local builds keep the default baked into internal/server.
GO_LDFLAGS=""
if [[ -n "${VERSION:-}" ]]; then
  GO_LDFLAGS="-X github.com/dev-kryptic/daemon/internal/server.Version=$VERSION"
fi
(cd "$ROOT/.." && go build -trimpath -ldflags "$GO_LDFLAGS" -o "$ROOT/.build/kryptic" ./cmd/kryptic)

echo "Building Kryptic daemon ($CONFIGURATION, macOS 13+)…"
cd "$ROOT"
swift build -c "$CONFIGURATION"

echo "Packaging ${APP_NAME}.app…"
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

cp "$BUILD_DIR/KrypticDaemon" "$APP_BUNDLE/Contents/MacOS/"
cp "$ROOT/.build/kryptic" "$APP_BUNDLE/Contents/MacOS/kryptic"
cp "$ROOT/Sources/KrypticDaemon/Info.plist" "$APP_BUNDLE/Contents/Info.plist"
cp "$RESOURCES/"* "$APP_BUNDLE/Contents/Resources/"

# SwiftPM's generated Bundle.module looks for this next to Contents.
# Without it the menu-bar app fatalErrors on launch and exits silently
# (LSUIElement hides the Dock icon, so it looks like a no-op).
RESOURCE_BUNDLE="$BUILD_DIR/KrypticDaemon_KrypticDaemon.bundle"
if [[ ! -d "$RESOURCE_BUNDLE" ]]; then
  echo "Missing Swift resource bundle: $RESOURCE_BUNDLE" >&2
  exit 1
fi
cp -R "$RESOURCE_BUNDLE" "$APP_BUNDLE/KrypticDaemon_KrypticDaemon.bundle"

echo "Done: $APP_BUNDLE ($CONFIGURATION)"
echo "Run with: open \"$APP_BUNDLE\""
