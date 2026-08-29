# Changelog

Notes for each Kryptic daemon release. The signed-release workflow copies the
matching section into the GitHub Release.

## Unreleased

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
