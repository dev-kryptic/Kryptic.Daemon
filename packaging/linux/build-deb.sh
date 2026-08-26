#!/usr/bin/env bash
# Builds a desktop-capable .deb: CLI, tray, systemd user unit, .desktop,
# icon, AppStream metainfo, and copyright. Used by the signed-release workflow.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION=""
DEBARCH=""
CLI=""
TRAY=""
OUT=""
MAINTAINER="${MAINTAINER:-Kryptic <hello@kryptic.dev>}"

usage() {
  echo "Usage: $0 --version X.Y.Z --debarch amd64|arm64 --cli PATH --tray PATH --out FILE.deb" >&2
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --debarch) DEBARCH="${2:-}"; shift 2 ;;
    --cli) CLI="${2:-}"; shift 2 ;;
    --tray) TRAY="${2:-}"; shift 2 ;;
    --out) OUT="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -n "$VERSION" && -n "$DEBARCH" && -n "$CLI" && -n "$TRAY" && -n "$OUT" ]] || usage
[[ -f "$CLI" ]] || { echo "CLI binary not found: $CLI" >&2; exit 1; }
[[ -f "$TRAY" ]] || { echo "tray binary not found: $TRAY" >&2; exit 1; }

if [[ "$MAINTAINER" != *"<"* ]]; then
  MAINTAINER="Kryptic <hello@kryptic.dev>"
fi

ICON="$ROOT/macos/Sources/KrypticDaemon/Resources/AppIcon.png"
[[ -f "$ICON" ]] || { echo "app icon not found: $ICON" >&2; exit 1; }

PKGROOT="$(mktemp -d)"
trap 'rm -rf "$PKGROOT"' EXIT

install -d -m 0755 \
  "$PKGROOT/DEBIAN" \
  "$PKGROOT/usr/bin" \
  "$PKGROOT/usr/lib/systemd/user" \
  "$PKGROOT/usr/share/applications" \
  "$PKGROOT/etc/xdg/autostart" \
  "$PKGROOT/usr/share/metainfo" \
  "$PKGROOT/usr/share/icons/hicolor/256x256/apps" \
  "$PKGROOT/usr/share/icons/hicolor/128x128/apps" \
  "$PKGROOT/usr/share/pixmaps" \
  "$PKGROOT/usr/share/doc/kryptic"

install -m 0755 "$CLI" "$PKGROOT/usr/bin/kryptic"
install -m 0755 "$TRAY" "$PKGROOT/usr/bin/kryptic-tray"

sed 's#ExecStart=%h/.local/bin/kryptic start#ExecStart=/usr/bin/kryptic start#' \
  "$ROOT/packaging/linux/kryptic-daemon.service" \
  > "$PKGROOT/usr/lib/systemd/user/kryptic-daemon.service"
chmod 0644 "$PKGROOT/usr/lib/systemd/user/kryptic-daemon.service"

install -m 0644 "$ROOT/packaging/linux/dev.kryptic.Kryptic.desktop" \
  "$PKGROOT/usr/share/applications/dev.kryptic.Kryptic.desktop"
install -m 0644 "$ROOT/packaging/linux/dev.kryptic.Kryptic.desktop" \
  "$PKGROOT/etc/xdg/autostart/dev.kryptic.Kryptic.desktop"
install -m 0644 "$ROOT/packaging/linux/dev.kryptic.Kryptic.metainfo.xml" \
  "$PKGROOT/usr/share/metainfo/dev.kryptic.Kryptic.metainfo.xml"
install -m 0644 "$ROOT/packaging/linux/copyright" \
  "$PKGROOT/usr/share/doc/kryptic/copyright"
install -m 0644 "$ROOT/CHANGELOG.md" \
  "$PKGROOT/usr/share/doc/kryptic/changelog"
gzip -n -9 "$PKGROOT/usr/share/doc/kryptic/changelog"

install -m 0644 "$ICON" "$PKGROOT/usr/share/icons/hicolor/256x256/apps/kryptic.png"
install -m 0644 "$ICON" "$PKGROOT/usr/share/icons/hicolor/128x128/apps/kryptic.png"
install -m 0644 "$ICON" "$PKGROOT/usr/share/pixmaps/kryptic.png"

install -m 0755 "$ROOT/packaging/linux/deb/preinst" "$PKGROOT/DEBIAN/preinst"
install -m 0755 "$ROOT/packaging/linux/deb/postinst" "$PKGROOT/DEBIAN/postinst"
install -m 0755 "$ROOT/packaging/linux/deb/prerm" "$PKGROOT/DEBIAN/prerm"

INSTALLED_SIZE="$(du -sk --exclude=DEBIAN "$PKGROOT" | awk '{print $1}')"

cat > "$PKGROOT/DEBIAN/control" <<EOF
Package: kryptic
Version: $VERSION
Section: utils
Priority: optional
Architecture: $DEBARCH
Maintainer: $MAINTAINER
Homepage: https://kryptic.dev
Vcs-Git: https://github.com/dev-kryptic/Kryptic.Daemon.git
Vcs-Browser: https://github.com/dev-kryptic/Kryptic.Daemon
Bugs: https://github.com/dev-kryptic/Kryptic.Daemon/issues
Installed-Size: $INSTALLED_SIZE
Recommends: libsecret-tools
Suggests: zenity | yad
Description: Kryptic secrets daemon, CLI and tray application
 Local daemon, CLI, and tray app for securely accessing Kryptic-managed
 secrets. Sign in once, then every project on this machine can inject
 secrets at startup.
EOF

mkdir -p "$(dirname "$OUT")"
dpkg-deb --root-owner-group --build "$PKGROOT" "$OUT"
echo "Built $OUT"
