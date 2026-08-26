#!/usr/bin/env bash
# Builds a signed static apt repository from the release .deb files.
# apt verifies every package against the Kryptic release GPG key, so
# installs and upgrades are trusted with no sideload warnings.
#
# Requires: dpkg-dev (dpkg-scanpackages), apt-utils (apt-ftparchive), gpg
# with the release key already imported.
set -euo pipefail

VERSION=""
KEY_ID=""
OUT=""
DEBS=()

usage() {
  echo "Usage: $0 --version X.Y.Z --key-id GPGKEY --out DIR --deb FILE [--deb FILE ...]" >&2
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --key-id) KEY_ID="${2:-}"; shift 2 ;;
    --out) OUT="${2:-}"; shift 2 ;;
    --deb) DEBS+=("${2:-}"); shift 2 ;;
    *) usage ;;
  esac
done

[[ -n "$VERSION" && -n "$KEY_ID" && -n "$OUT" && ${#DEBS[@]} -gt 0 ]] || usage
for deb in "${DEBS[@]}"; do
  [[ -f "$deb" ]] || { echo "deb not found: $deb" >&2; exit 1; }
done

command -v dpkg-scanpackages >/dev/null || { echo "dpkg-scanpackages missing (install dpkg-dev)" >&2; exit 1; }
command -v apt-ftparchive >/dev/null || { echo "apt-ftparchive missing (install apt-utils)" >&2; exit 1; }

GPG_SIGN=(gpg --batch --yes --local-user "$KEY_ID")
if [[ -n "${GPG_PASSPHRASE:-}" ]]; then
  GPG_SIGN+=(--pinentry-mode loopback --passphrase "$GPG_PASSPHRASE")
fi

rm -rf "$OUT"
mkdir -p "$OUT/pool/main/k/kryptic"
cp "${DEBS[@]}" "$OUT/pool/main/k/kryptic/"

ARCHES=()
for deb in "$OUT"/pool/main/k/kryptic/*.deb; do
  arch="$(dpkg-deb --field "$deb" Architecture)"
  ARCHES+=("$arch")
done

cd "$OUT"

for arch in "${ARCHES[@]}"; do
  bin_dir="dists/stable/main/binary-${arch}"
  mkdir -p "$bin_dir"
  dpkg-scanpackages --arch "$arch" pool > "$bin_dir/Packages"
  gzip -9 -k -f "$bin_dir/Packages"
done

apt-ftparchive \
  -o "APT::FTPArchive::Release::Origin=Kryptic" \
  -o "APT::FTPArchive::Release::Label=Kryptic" \
  -o "APT::FTPArchive::Release::Suite=stable" \
  -o "APT::FTPArchive::Release::Codename=stable" \
  -o "APT::FTPArchive::Release::Components=main" \
  -o "APT::FTPArchive::Release::Architectures=${ARCHES[*]}" \
  -o "APT::FTPArchive::Release::Description=Kryptic daemon, CLI and tray application" \
  release dists/stable > dists/stable/Release

"${GPG_SIGN[@]}" --armor --detach-sign --output dists/stable/Release.gpg dists/stable/Release
"${GPG_SIGN[@]}" --clearsign --output dists/stable/InRelease dists/stable/Release

# Public key for the signed-by= sources entry.
gpg --batch --yes --armor --export "$KEY_ID" > kryptic.asc

# GitHub Pages serves this directory; stop Jekyll from mangling paths.
touch .nojekyll
echo "$VERSION" > VERSION

echo "apt repository written to $OUT"
