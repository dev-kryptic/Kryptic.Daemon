#!/usr/bin/env bash
# Builds the release artifacts `kryptic update` and the download page expect:
# cross-compiled binaries + checksums.txt, all in dist/.
#
# Code signing/notarization (Apple Developer ID, Windows Authenticode) happens
# after this script, on the signed artifacts, before uploading to the GitHub
# release - the checksums must be regenerated if signing modifies the binaries.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"

"$ROOT/build-cross.sh"

echo "checksums…"
cd "$DIST"
: > checksums.txt
for artifact in kryptic_* kryptic-tray_*; do
  [[ -f "$artifact" ]] || continue
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$artifact" >> checksums.txt
  else
    shasum -a 256 "$artifact" >> checksums.txt
  fi
done

echo
cat checksums.txt
echo
echo "Upload the contents of dist/ to the GitHub release (tag v<version>)."
