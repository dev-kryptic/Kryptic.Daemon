# Changelog

Notes for each Kryptic daemon release. The signed-release workflow copies the
matching section into the GitHub Release.

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
