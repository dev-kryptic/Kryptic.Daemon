# Contributing

This repository is the Kryptic daemon, `kryptic` CLI, and tray / menu-bar apps.
It is the authentication boundary on a developer machine: secrets live in
daemon memory and are served over a local socket (or Windows named pipe).

## What we accept

- Bug fixes in the CLI, daemon, tray, or packaging
- Test coverage
- Documentation corrections (README, PROTOCOL.md)
- Portability fixes for macOS, Linux, and Windows

## What we do not accept

- Persisting secret values to disk
- Serving secrets to a different OS user
- Telemetry or crash reporters that could include secret material
- Public GitHub issues for vulnerabilities (email security@kryptic.dev)

Protocol changes in [PROTOCOL.md](PROTOCOL.md) must stay compatible with the
language packages (`Kryptic.Net`, `Kryptic.Node`, `Kryptic.Python`,
`Kryptic.Java`, `Kryptic.Go`, `Kryptic.Ruby`, `Kryptic.Cpp`, `Kryptic.Rust`).
Coordinate a protocol bump across those repositories.

## Development

```bash
go test ./...
go build -o kryptic ./cmd/kryptic
```

`build-cross.sh` cross-compiles the CLI and tray. `macos/build.sh` packages
the menu-bar app.

## Licensing of contributions

This repository is GPL-3.0. By opening a pull request you confirm the
contribution is your own work (or you have the right to submit it) and you
license it under GPL-3.0. There is no CLA.

## Code of conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
