#!/usr/bin/env bash
set -euo pipefail

# Embeds icon, VERSIONINFO, and an asInvoker manifest into the Windows
# executables. go build picks up rsrc_windows_amd64.syso automatically.
# Optional first argument is the product version (1.2.3 or 1.2.3.0).

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION="${1:-}"
flags=(make --arch amd64 --in winres/winres.json --out rsrc)
if [[ -n "$VERSION" ]]; then
  flags+=(--file-version "$VERSION" --product-version "$VERSION")
fi

for pkg in cmd/kryptic cmd/kryptic-tray; do
  (cd "$ROOT/$pkg" && go run github.com/tc-hib/go-winres@v0.3.3 "${flags[@]}")
done
