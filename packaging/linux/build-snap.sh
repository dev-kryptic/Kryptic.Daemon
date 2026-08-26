#!/usr/bin/env bash
# Packs the Linux CLI + tray into a classic Snap for Ubuntu App Center.
# Requires snapcraft (sudo snap install snapcraft --classic).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION=""
ARCH=""
CLI=""
TRAY=""
OUT=""

usage() {
  echo "Usage: $0 --version X.Y.Z --arch amd64|arm64 --cli PATH --tray PATH --out FILE.snap" >&2
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --arch) ARCH="${2:-}"; shift 2 ;;
    --cli) CLI="${2:-}"; shift 2 ;;
    --tray) TRAY="${2:-}"; shift 2 ;;
    --out) OUT="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -n "$VERSION" && -n "$ARCH" && -n "$CLI" && -n "$TRAY" && -n "$OUT" ]] || usage
[[ "$ARCH" == "amd64" || "$ARCH" == "arm64" ]] || { echo "arch must be amd64 or arm64" >&2; exit 1; }
[[ -f "$CLI" ]] || { echo "CLI binary not found: $CLI" >&2; exit 1; }
[[ -f "$TRAY" ]] || { echo "tray binary not found: $TRAY" >&2; exit 1; }

if ! command -v snapcraft >/dev/null 2>&1; then
  echo "snapcraft not found. Install with: sudo snap install snapcraft --classic" >&2
  exit 1
fi

ICON="$ROOT/macos/Sources/KrypticDaemon/Resources/logo.png"
if [[ ! -f "$ICON" ]]; then
  ICON="$ROOT/cmd/kryptic-tray/assets/kryptic.png"
fi
[[ -f "$ICON" ]] || { echo "app icon not found" >&2; exit 1; }

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

install -d "$WORKDIR/prime-src" "$WORKDIR/snap/gui"
install -m 0755 "$CLI" "$WORKDIR/prime-src/kryptic"
install -m 0755 "$TRAY" "$WORKDIR/prime-src/kryptic-tray"
install -m 0644 "$ICON" "$WORKDIR/snap/gui/kryptic.png"
install -m 0644 "$ROOT/packaging/linux/snap/kryptic.desktop" \
  "$WORKDIR/snap/gui/kryptic-tray.desktop"

sed -e "s/@VERSION@/$VERSION/g" -e "s/@ARCH@/$ARCH/g" \
  "$ROOT/packaging/linux/snap/snapcraft.yaml.in" \
  > "$WORKDIR/snapcraft.yaml"

mkdir -p "$(dirname "$OUT")"
ABS_OUT="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"

(
  cd "$WORKDIR"
  export SNAPCRAFT_BUILD_ENVIRONMENT=host
  if ! snapcraft pack; then
    sudo --preserve-env=SNAPCRAFT_BUILD_ENVIRONMENT snapcraft pack
  fi
)

SNAP_FILE="$(find "$WORKDIR" -maxdepth 1 -name '*.snap' -print -quit)"
if [[ -z "$SNAP_FILE" ]]; then
  echo "snapcraft pack produced no .snap" >&2
  exit 1
fi
cp "$SNAP_FILE" "$ABS_OUT"
echo "Built $ABS_OUT"
