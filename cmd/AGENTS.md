# Entry Points

Two binaries serving distinct roles.

| Binary | Main file       | Purpose                          |
| ------ | --------------- | -------------------------------- |
| server | `server/main.go` | HTTP API server (Fiber v3, slog) |
| cli    | `cli/main.go`    | Plugin management CLI (cobra)    |

## Server (`cmd/server/`)

Fiber v3 HTTP server with ent ORM. Initializes database, runs auto-migration, creates services, registers routes, handles graceful shutdown.

Key dependencies: fiber v3, ent, pgx, viper, slog.

## CLI (`cmd/cli/`)

Cobra CLI with three subcommands:

- `flowbot plugin publish` — reads `plugin.yaml`, collects artifacts, pushes to OCI registry
- `flowbot plugin install ns/name:version` — fetches version info from store API, pulls OCI artifact, extracts
- `flowbot plugin search [query]` — searches plugin registry via store API

CLI files share package `main`. Shared utilities (`errNotImplemented`, `httpClient`, `doJSONGet`) are in `search.go`.

## Build

```bash
go tool task build          # Server binary
go tool task build:cli      # CLI binary
```
