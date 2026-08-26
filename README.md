# Kryptic Daemon

The Kryptic daemon and `kryptic` CLI in a single Go codebase (macOS, Linux, Windows).
It is the authentication boundary between a
developer's machine and the Kryptic platform: sign in once via the browser, then every
SDK on the machine gets its secrets through a local socket. Secrets live in daemon
memory only - never on disk.

## Install

Download a signed installer from [kryptic.dev/download](https://kryptic.dev/download).
That page detects your OS and architecture and serves the current build.

```bash
# Debian/Ubuntu/elementary: signed apt repository (verified installs + updates)
sudo install -d /etc/apt/keyrings
curl -fsSL https://kryptic.dev/apt/kryptic.asc | sudo tee /etc/apt/keyrings/kryptic.asc >/dev/null
echo "deb [signed-by=/etc/apt/keyrings/kryptic.asc] https://kryptic.dev/apt stable main" | sudo tee /etc/apt/sources.list.d/kryptic.list
sudo apt update && sudo apt install kryptic
kryptic login
```

```bash
# Any Linux (detects arch, verifies checksums, installs the CLI + tray + user service)
curl -fsSL https://kryptic.dev/install.sh | sh
kryptic login
```

The apt repository is signed with the Kryptic release GPG key, so apt
verifies every install and upgrade. The `.deb` on the download page is the
same package for manual installs (`sudo apt install ./kryptic_*.deb`);
opening a downloaded file in a store GUI shows a generic third-party
warning because the file is local, so prefer the repository.

On macOS and Windows, run the installer from the download page, then `kryptic login`.

```
kryptic login                 # browser device flow, refresh token -> OS credential store
kryptic start                 # run the daemon (launchd/systemd/service manager keeps it alive)
kryptic stop                  # stop the running daemon
kryptic status                # daemon + session status
kryptic whoami                # signed-in user and organization
kryptic secrets list          # projects and environments you can pull
kryptic secrets get KEY --project proj_x --env development
kryptic flush                 # clear the daemon's secrets cache
kryptic scan [PATH|--staged]  # scan for leaked secrets (gitleaks ruleset, local only)
kryptic update                # replace this binary with the latest published build
kryptic update --check        # report whether a newer release exists
kryptic config                # show the Daemon BFF URL
kryptic config set-api URL    # point this machine at a different Kryptic server
kryptic logout
```

## How it works

- **Auth**: device flow against the Daemon BFF - the CLI prints a code, the browser
  approves it, the rotating refresh token is stored in the platform credential store:
  macOS Keychain (`/usr/bin/security`), Windows Credential Manager (advapi32), or
  libsecret on Linux (`secret-tool`), with a 0600 file fallback when no store is
  available. Access tokens (15 min) stay in memory.
- **Scanning**: `kryptic scan` runs the gitleaks default ruleset (222 rules) fully
  locally - nothing leaves the machine. Findings are redacted in the output and a
  non-zero exit code makes it usable as a pre-commit or CI gate.
- **Serving**: a unix socket (`/tmp/kryptic-daemon.sock`, 0600, override with
  `KRYPTIC_SOCKET_PATH`) speaking [PROTOCOL.md](PROTOCOL.md) v1. Every connection
  is authenticated by the caller's OS credentials (`LOCAL_PEERCRED`/`SO_PEERCRED`
  on macOS/Linux, the pipe security descriptor on Windows), so only a process
  running as the same user can pull secrets. Bundles are cached in memory for 5
  minutes.
- **Self-update**: `kryptic update` verifies the downloaded binary against the
  release `checksums.txt`. The menu-bar and tray apps also have **Check for
  Updates**, which downloads the signed installer (macOS `.pkg`, Windows setup,
  Linux `.deb` when you installed from a package) and opens it. Reinstall from
  [kryptic.dev/download](https://kryptic.dev/download) if you prefer.
- **API**: talks to `https://daemon.kryptic.dev`. Set a different Daemon BFF with
  `kryptic config set-api URL` (or **Server URL** in the menu). `KRYPTIC_API`
  overrides the saved value, which is how local development points at a BFF on
  localhost. Changing the URL signs you out, because tokens belong to one server.

## Build

```bash
go build -o kryptic ./cmd/kryptic     # ~8 MB static binary, stdlib only
```

`build-cross.sh` cross-compiles the CLI for macOS/Linux/Windows and the
`kryptic-tray` app for Linux and Windows into `dist/`. The tray app is the
Windows/Linux counterpart of the macOS menu-bar app: the same menu (status,
sign in/out, cancel sign-in, refresh secrets cache, About, quit), running the
daemon in-process - one binary, no bundling. On Windows the daemon serves the
named pipe `\\.\pipe\kryptic-daemon`.

`packaging/linux/build-deb.sh` wraps those Linux binaries in a `.deb` with a
`.desktop` file, icon, AppStream metainfo, and a systemd user unit.
`packaging/linux/build-apt-repo.sh` turns the release `.deb` files into the
GPG-signed apt repository served at `https://kryptic.dev/apt`.
`packaging/linux/install.sh` does the user-local install into `~/.local`
without root.

`macos/build.sh` packages the menu-bar app with this binary bundled inside.
`macos/build.sh --debug` builds it pointed at a local platform (`http://localhost:5211`,
the docker-compose Daemon BFF) so sign-in opens the local management client; an explicit
`KRYPTIC_API` env var overrides either default (e.g. `http://localhost:5237` for a BFF
run from the IDE).

`macos/` contains the earlier SwiftUI menu-bar implementation, kept as reference until
the Go daemon reaches installer/notarization parity.

License: GPL-3.0. Third-party notices: THIRD_PARTY_NOTICES.
