# Changelog

Notes for each Kryptic daemon release. The signed-release workflow copies the
matching section into the GitHub Release.

## Unreleased

### Added

- Multiple accounts on one install. `kryptic login --add`, `kryptic profile`,
  `kryptic profile switch`, and `kryptic profile delete` keep personal and
  work sessions side by side. Each profile has its own config, including
  Daemon BFF URL, so cloud and self-host can live on the same machine.
  Sign-out and `kryptic reset-device` apply to the active profile;
  `kryptic reset-device --all` wipes every profile.
- Menu-bar and tray status dot: green when connected, amber while connecting
  or awaiting an organization-key grant, gray when signed out.
- Open Kryptic on macOS (SwiftUI), Windows (Win32), and Linux
  (system dialogs). Choose it from the menu bar or tray, or run
  `kryptic panel` on Windows and Linux.
- Device keys persist across `kryptic logout`. `kryptic reset-device` deletes
  them and revokes the machine so the next login needs a new admin grant.
- Device-flow poll signs the start challenge so the platform can prove this
  install still holds the private key.

### Changed

- `kryptic login` reuses an existing OS-user key pair instead of generating one
  every time. Uninstall (deb prerm) wipes leftover keys.
- `kryptic config set-api` and Server URI edit the active profile only.
  Other profiles keep their URL and session. `kryptic login --add --api URL`
  creates a new profile against that host.
- Open Kryptic title, Sign In / Sign Out verbs, hover trash to remove an
  account, Kryptic Cloud as the first Server URI option, and a status-dot
  legend in About.

### Fixed

- Debug builds and local docs point the daemon at the IDE Daemon BFF
  (`http://localhost:5237`). Compose still publishes that BFF on `:5211`.

## 1.1.2 - 2026-09-11

### Changed

- Update code signing certificate.

## 1.1.1 - 2026-09-10

### Changed

- `github.com/dev-kryptic/Kryptic.Encryption.Go` v1.1.0. The `ksm2_` auth
  derivation now comes from the library instead of a local copy.

## 1.1.0 - 2026-09-10

### Changed

- `github.com/dev-kryptic/Kryptic.Encryption.Go` v1.0.2.
- Machine token exchange supports the new `ksm2_` client-secret format.
  Legacy secrets keep working unchanged.
- The daemon socket moved out of `/tmp` into a per-user directory:
  `~/Library/Application Support/kryptic` on macOS, `$XDG_RUNTIME_DIR` or
  `~/.config/kryptic` on Linux. Third-party clients should resolve the new
  path (see PROTOCOL.md).
- `kryptic ci export` (dotenv/shell) and `kryptic secrets export` skip keys
  that are not valid shell identifiers, with a stderr warning; JSON output
  keeps every key.

### Security

- Hardening across the self-update process, macOS keychain handling, local
  IPC, and request encoding.

## 1.0.0 - 2026-09-05

First production release.

### Changed

- Menu **Check for Updates** no longer uses an ellipsis.

## 0.13.14 - 2026-09-03

Windows dialogs use Kryptic primary and ghost buttons, not Win32 chrome.

### Changed

- Windows dialog and progress actions are owner-drawn Kryptic buttons
  (accent primary, bordered ghost) instead of the default Win32 chrome.

## 0.13.13 - 2026-09-02

Stay signed in through transient errors, and add a support diagnostics log.

### Added

- Diagnostics log at `…/kryptic/logs/kryptic.krypticlog` (2 MiB, one rotated
  backup). Function only: no secrets, tokens, or names. `kryptic logs`,
  `kryptic logs --reveal`, and **Reveal Diagnostics Log** in the menu bar / tray.

### Changed

- A stored session stays signed in through transient platform errors (timeouts,
  5xx, a stale access token). Only a missing or rejected refresh token shows as
  logged out.
- Daemon and CLI serialize refresh-token rotation so they cannot spend the same
  token.
- Menu bar and tray menus share one layout: status, sign in/out, Operations,
  Settings, Help & Support (GitHub, docs, diagnostics log), About, Quit.
- Windows dialog actions are native push buttons instead of underlined text.

## 0.13.12 - 2026-09-02

Offline folder scan from the tray, and clearer local-hook docs.

### Added

- **Scan…** in the Windows/Linux tray and the macOS menu bar. Pick a folder,
  run the embedded gitleaks engine fully offline (no sign-in, no network),
  watch a 0-100% progress bar, and cancel to stop the walk. On completion,
  `kryptic-scan-report.md` is written at the folder you chose, with a result
  dialog and **Open Report**. A cancelled scan does not leave a finished report.

### Changed

- `kryptic scan` walks honor `context` cancellation so the tray can stop
  promptly. TTY progress and quiet-when-not-a-TTY (hooks, CI) are unchanged.
- Docs describe the pre-commit setup as a local git hook on the developer
  machine. Staged files never leave the laptop.

## 0.13.11 - 2026-08-31

Scan progress on a TTY, and a branded Markdown export.

### Added

- `kryptic scan --export [FILE|DIR]` writes a Markdown report with the
  Kryptic logo, a summary, and redacted findings. Omit the path to write
  `kryptic-scan-report.md` in the directory you ran the command from.

### Changed

- `kryptic scan` shows a 0-100% progress bar on a terminal. CI, pipes, and
  git hooks stay quiet.

## 0.13.10 - 2026-08-29

Return the protocol skip codes for a missing environment and a stale grant.

### Fixed

- A missing environment now returns `unknown_environment` instead of
  collapsing every 404 into `unknown_project`.
- A stale or missing organization-key grant now returns `access_denied`
  (as PROTOCOL.md already specified) instead of `internal`, so packages
  skip the same way they do for a project the user cannot read.

## 0.13.9 - 2026-08-29

Confirm sign-out, and make a running daemon notice it immediately.

### Fixed

- `kryptic logout` now tells a running daemon to drop its in-memory access
  token and decrypted secrets cache (new `reset-auth` socket request). Signing
  out from a second terminal previously left `kryptic status` reporting the
  user as signed in, and cached secrets servable, for up to 15 minutes on
  every OS.

### Changed

- Signing out now asks for confirmation everywhere, because logout deletes the
  device's encryption key and the next login needs a fresh org-key grant from
  an admin. The CLI prompts `[y/N]` when run interactively (`--yes` or a
  non-interactive stdin skips it), and the tray (Windows/Linux) and menu-bar
  app (macOS) show a confirmation dialog.

## 0.13.8 - 2026-08-28

One tray instance per user, and Windows starts Kryptic at login.

### Fixed

- Launching the tray twice no longer shows two tray icons. A per-user lock
  (flock on Linux, named mutex on Windows) makes the second launch a no-op.
  The daemon socket could not guard this: a tray remote-controlling a
  systemd-run daemon holds no socket.
- Windows starts Kryptic at login, like Linux and macOS already did. The
  installer registers the tray under `HKCU\...\Run` and the tray re-writes
  the value on every start, so in-place updated installs get autostart
  without re-running the installer. Turning Kryptic off in Task Manager's
  Startup tab is respected: Windows keeps that toggle in a separate key.

## 0.13.7 - 2026-08-28

Linux tray restarts itself after an in-place update and self-heals the
launcher icon.

### Fixed

- Linux tray restarts itself after an in-place update. `/proc/self/exe`
  points at the renamed-and-deleted previous binary after the update's
  rename dance, so the old re-exec failed silently and the previous version
  kept running until a manual quit and relaunch.
- Linux tray installs the launcher icon on every start (self-heal). Existing
  installs from older packages get the green brand mark on the next update,
  with no reinstall needed.

## 0.13.6 - 2026-08-28

Linux launcher shows the green brand mark instead of GNOME's generic gear.

### Fixed

- Linux launcher shows the green brand mark. `install.sh` and the `.deb`
  ship `kryptic.png` / `kryptic.svg`. The previous packages named
  `Icon=kryptic` without installing an icon, so GNOME showed a generic gear.

## 0.13.5 - 2026-08-27

Linux updates replace the installed binaries in place instead of opening a
`.deb` in App Center.

### Changed

- Linux **Check for Updates** no longer opens the `.deb` in App Center. The
  tray and `kryptic update` replace the installed CLI and tray binaries in
  place, show a progress window, and prompt for permission only when `/usr`
  is not writable.
- `.deb` artifacts are not published in the signed-release workflow until the
  Snap Store listing is in App Center. Linux installs use `install.sh`.
- Apt repository publish is disabled in the signed-release workflow until the
  GitHub Pages deploy path is ready.

## 0.13.4 - 2026-08-27

Linux dark-mode trays show the white falcon, and the daemon app icon is the
green brand mark instead of a grey silhouette.

### Fixed

- Linux tray follows the desktop color-scheme and shows the white falcon on a
  dark panel. GNOME and Ubuntu paint tray pixmaps as-is, so the black falcon
  was invisible on the default dark top bar.

### Changed

- Tray and menu-bar icons are `Falcon.svg` / `Falcon-black.svg` on Linux,
  Windows, and macOS. Linux and Windows rasterize those SVGs at runtime.
- Daemon app icon is the green no-text brand mark (`logo.svg`), not the grey
  falcon silhouette. macOS `AppIcon.png`, Linux launcher icons, and Windows
  `.ico` files are generated from that SVG.

## 0.13.3 - 2026-08-26

Debian and Ubuntu installs now come from a GPG-verified apt repository
instead of sideloaded packages.

### Added

- Signed apt repository (`packaging/linux/build-apt-repo.sh`, published to
  GitHub Pages and served at `https://kryptic.dev/apt`). Debian/Ubuntu
  installs and upgrades are verified against the Kryptic release GPG key:
  `sudo apt install kryptic`, no sideload warnings.

### Removed

- Snap packaging. A sideloaded `.snap` needs `--dangerous` to install and a
  Snap Store listing needs manual review for classic confinement, so the
  Snap path is gone in favor of the signed apt repository.

## 0.13.2 - 2026-08-26

Ubuntu App Center only shows a publisher for Snap Store listings. This
release packages the Linux CLI and tray as a classic Snap.

### Added

- Classic Snap package (`packaging/linux/build-snap.sh`) so Ubuntu App Center
  can list Kryptic with a real publisher. Sideloaded `.deb` files cannot.

### Fixed

- Snapcraft now finds the tray desktop file: it is staged into the primed
  snap (`share/applications/kryptic-tray.desktop`) instead of the project
  `snap/gui/` assets folder.

## 0.13.1 - 2026-08-26

Linux now ships a real desktop application, not only a CLI and a systemd unit.

### Fixed

- Linux `.deb` and `install.sh` now install the tray as a desktop application
  (launcher, autostart, icon, AppStream metadata, maintainer, and license).
  Opening the previous package in AppCenter showed "Unknown publisher" and no
  launchable app because only the CLI and a systemd unit were packaged.

## 0.13.0 - 2026-08-26

Signed in is not the same as able to decrypt. This release makes a missing
organization-key grant, and a per-project secrets denial, visible on the
machine instead of failing silently.

### Added

- OS notifications (macOS, Linux, Windows) when the daemon is signed in but
  has not been granted the organization key, and when a secrets fetch is
  denied for a named project.
- `orgKeyGranted` on the local `status` reply. The menu bar, tray, and
  `kryptic status` / `kryptic whoami` show when this device is waiting for
  an admin grant under Approvals.
- `CHANGELOG.md`, used as the GitHub Release body.

### Fixed

- `kryptic start` refuses to run as root. A leftover `sudo kryptic start`
  owned the socket and pidfile, so the menu bar sat on "Daemon: starting…".
- `kryptic stop` reports an unreadable (root-owned) pidfile instead of
  "not running". The macOS installer now kills a leftover root `kryptic`
  and removes `/tmp/kryptic-daemon.sock`.
- Menu bar shows why `kryptic start` exited, instead of hanging on
  "starting…".
- `kryptic logout` warns that the device key is deleted and an admin must
  re-grant the organization key after the next login.
