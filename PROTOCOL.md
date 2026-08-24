# Kryptic Daemon Socket Protocol - v1

The contract between the Kryptic daemon and every language Package. The daemon listens on a
local OS socket; Packages connect during application startup, request the secrets for the
current project + environment, inject them, and disconnect. One request per connection.

## Transport

| Platform | Endpoint |
| --- | --- |
| Windows | Named pipe `\\.\pipe\kryptic-daemon` |
| macOS | Unix domain socket `/tmp/kryptic-daemon.sock` |
| Linux | `$XDG_RUNTIME_DIR/kryptic-daemon.sock`, fallback `/tmp/kryptic-daemon.sock` |

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
  "apiUrl": "https://daemon.kryptic.dev"
}
```

`apiUrl` is the Daemon BFF this process is using. Clients that do not understand
it must ignore it (forward compatibility).

### `flush` - drop the daemon's in-memory secrets cache

Additive in v1. Used by the menu-bar app's "Refresh Secrets Cache" and `kryptic flush`
so an updated secret is refetched immediately instead of after the 5-minute TTL.

```json
{ "v": 1, "type": "flush" }
```

```json
{ "v": 1, "ok": true, "cleared": 2 }
```

## Error responses

```json
{ "v": 1, "ok": false, "error": "not_authenticated", "message": "Run `kryptic login` first." }
```

| `error` | Meaning | Package behavior |
| --- | --- | --- |
| `not_authenticated` | Daemon running but no session | warn once, continue without secrets |
| `access_denied` | User lacks access to the project/environment | warn once, continue |
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
