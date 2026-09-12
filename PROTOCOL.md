# Kryptic Daemon Socket Protocol - v1

The contract between the Kryptic daemon and every language Package. The daemon listens on a
local OS socket; Packages connect during application startup, request the secrets for the
current project + environment, inject them, and disconnect. One request per connection.

## Transport

| Platform | Endpoint |
| --- | --- |
| Windows | Named pipe `\\.\pipe\kryptic-daemon` |
| macOS | Unix domain socket `~/Library/Application Support/kryptic/kryptic-daemon.sock` |
| Linux | `$XDG_RUNTIME_DIR/kryptic-daemon.sock`, fallback `~/.config/kryptic/kryptic-daemon.sock` |

The socket always lives in a per-user 0700 directory, never in `/tmp`: a
predictable path in a world-writable directory would let another local user
squat on or swap the socket. Clients also refuse to connect to a socket whose
owner uid is not their own.

The path can be overridden with the `KRYPTIC_SOCKET_PATH` environment variable
(Packages must honor it - it is how tests point a package at a mock daemon).

The daemon authenticates every connection by the caller's OS credentials: on
macOS/Linux it reads the peer user id from the kernel (`LOCAL_PEERCRED` /
`SO_PEERCRED`) and serves secrets only to a process running as the same user;
on Windows the named pipe's security descriptor enforces the equivalent. A
connection from any other user is dropped without a reply. The socket's `0600`
permission is a second layer, not the primary gate.

## Framing

Newline-delimited JSON (NDJSON): the client writes exactly one JSON object terminated by
`\n`, the daemon replies with exactly one JSON object terminated by `\n`, then the
connection closes. No length prefixes, no multiplexing - the payloads are small and the
socket is local.

All property names are camelCase. Unknown properties must be ignored (forward
compatibility).

## Requests

Every request carries `v` (protocol version, currently `1`) and `type`.

### `secrets` - fetch the bundle for a project + environment

```json
{ "v": 1, "type": "secrets", "projectId": "proj_a1b2c3d4e5f6", "environment": "development" }
```

Success response:

```json
{
  "v": 1,
  "ok": true,
  "secrets": [
    { "key": "DATABASE_URL", "value": "postgres://…" },
    { "key": "REDIS_URL", "value": "redis://…" }
  ]
}
```

`secrets` is a list (not a map) so ordering is stable and duplicate handling is explicit.

### `status` - daemon health and identity (backs IDE plugins and `kryptic status`)

```json
{ "v": 1, "type": "status" }
```

```json
{
  "v": 1,
  "ok": true,
  "authenticated": true,
  "email": "dev@company.com",
  "organization": "Acme",
  "daemonVersion": "1.0.0",
  "apiUrl": "https://daemon.kryptic.dev",
  "orgKeyGranted": true,
  "connection": "connected",
  "activeProfileId": "a1b2c3d4e5f60708",
  "profiles": [
    {
      "id": "a1b2c3d4e5f60708",
      "email": "dev@company.com",
      "organization": "Acme",
      "api": "https://daemon.kryptic.dev",
      "active": true,
      "signedIn": true
    }
  ]
}
```

`apiUrl` is the Daemon BFF the **active** profile is using. Each saved profile
has its own `api`. `orgKeyGranted` is whether an admin has sealed the
organization key to this device. Signed in is not enough to decrypt: without
this grant every secrets fetch returns `access_denied`. `connection` is
`connected`, `awaiting_approval`, or `signed_out`. `profiles` lists saved
accounts on this install; switching the active profile does not sign the
others out. Clients that do not understand these fields must ignore them
(forward compatibility). Older daemons omit `orgKeyGranted`; treat a missing
field as granted.

### `flush` - drop the daemon's in-memory secrets cache

Additive in v1. Used by the menu-bar app's "Refresh Secrets Cache" and `kryptic flush`
so an updated secret is refetched immediately instead of after the 5-minute TTL.

```json
{ "v": 1, "type": "flush" }
```

```json
{ "v": 1, "ok": true, "cleared": 2 }
```

### `reset-auth` - drop the daemon's in-memory auth state

Additive in v1. Sent by `kryptic logout` (and the tray / menu-bar sign-out) so a
running daemon immediately drops its cached access token and decrypted bundles
instead of serving them until the token expires. Packages never send this.

```json
{ "v": 1, "type": "reset-auth" }
```

```json
{ "v": 1, "ok": true }
```

### `switch-profile` - activate another saved account

Additive in v1. The menu bar and tray send this so secrets start coming from
the chosen profile. Other profiles stay signed in. The daemon drops its
in-memory token and cache, points at that profile's Daemon BFF, then replies
with the same body as `status`.

```json
{ "v": 1, "type": "switch-profile", "profileId": "a1b2c3d4e5f60708" }
```

### `set-api` - change the active profile's Daemon BFF

Additive in v1. Signs that profile out of the previous host. Other profiles
keep their URL and session.

```json
{ "v": 1, "type": "set-api", "api": "https://daemon.example.com" }
```

### `delete-profile` - remove a saved account from this install

Additive in v1. Revokes that profile's device on its server, then deletes its
keys. Other profiles are not touched.

```json
{ "v": 1, "type": "delete-profile", "profileId": "a1b2c3d4e5f60708" }
```

## Error responses

```json
{ "v": 1, "ok": false, "error": "not_authenticated", "message": "Run `kryptic login` first." }
```

| `error` | Meaning | Package behavior |
| --- | --- | --- |
| `not_authenticated` | Daemon running but no session | warn once, continue without secrets |
| `access_denied` | User lacks project access, or this device has no organization-key grant yet | warn once, continue |
| `unknown_project` | Project id not found | warn once, continue |
| `unknown_environment` | Environment slug not found | warn once, continue |
| `unsupported_version` | Daemon does not speak the requested `v` | warn once, continue |
| `internal` | Anything else | warn once, continue |

## Package rules (protocol requirements every client package must implement)

1. **Passive detection.** If the socket does not exist, connection is refused, or the
   response times out (`KRYPTIC_TIMEOUT_MS`, default 2000), the Package logs one warning
   (unless `KRYPTIC_SILENT=true`) and returns without modifying the environment.
   A package must never throw or crash the host application.
2. **Development only.** In non-development environments the Package is a no-op, before any
   socket I/O. Each Package uses its runtime's idiomatic signal (`ASPNETCORE_ENVIRONMENT`,
   `NODE_ENV`, `RAILS_ENV`, …). `KRYPTIC_DISABLED=true` force-disables everywhere.
3. **Project discovery.** The Package finds `kryptic.json` by walking up from the current
   working directory. `KRYPTIC_PROJECT_ID` / `KRYPTIC_ENV` env vars override the file.
4. **Injection must not overwrite** environment variables that are already set - real
   environment always wins over injected values.

## Versioning

`v` is bumped only for breaking changes to framing or required fields. Additive fields
do not bump the version. A daemon must answer a request with an unknown `v` with the
`unsupported_version` error rather than closing the connection.
