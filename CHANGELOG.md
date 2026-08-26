# Changelog

Notes for each Kryptic daemon release. The signed-release workflow copies the
matching section into the GitHub Release.

## Unreleased

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
