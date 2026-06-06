# flowbot-registry

Plugin Registry — OCI-based plugin marketplace for the Flowbot ecosystem. Provides plugin publish, discovery, installation, and OCI token authentication.

## Architecture

```
                  Registry API (fiber v3)
                         │
  ┌──────────────────────┼──────────────────────┐
  │                      │                      │
  ▼                      ▼                      ▼
Auth Handler         Plugin Handler        Search Handler
  │                      │                      │
  ▼                      ▼                      ▼
AuthService          PluginService         PluginService
  │                      │                      │
  ▼                      ▼                      ▼
JWT Service          OCI Client            Store Adapter
(pkg/jwt)           (pkg/oci)             (pkg/store)
                                               │
                                               ▼
                                          ent ORM
                                           │
                                           ▼
                                      PostgreSQL

  ┌──────────────────────────────────────────┐
  │              CLI (cobra)                  │
  │  publish │ install │ search              │
  └──────────────────────────────────────────┘
```

## API Endpoints

| Method | Path                                    | Description                          |
| ------ | --------------------------------------- | ------------------------------------ |
| GET    | `/api/v1/auth/token`                    | Docker Registry v2 token (RS256 JWT) |
| POST   | `/api/v1/plugins/:ns/:name/publish`     | Publish plugin version               |
| GET    | `/api/v1/plugins`                       | Search/list plugins                  |
| GET    | `/api/v1/plugins/:ns/:name/versions/:v` | Get version details                  |

## Database

ent ORM with PostgreSQL. Three core tables with unique composite indexes:

| Table             | Unique Index        |
| ----------------- | ------------------- |
| `namespaces`      | name                |
| `plugins`         | namespace_id + name |
| `plugin_versions` | plugin_id + version |

## Quick Start

```bash
# Install tools
go mod download

# Generate ent code
go tool task ent

# Build
go tool task build
go tool task build:cli

# Run server
go run ./cmd/server

# Run CLI
go run ./cmd/cli search
```

## Development

```bash
go tool task lint          # Code lint (revive + testifylint + actionlint)
go tool task test          # Unit tests
go tool task test:race     # Race condition tests
go tool task test:coverage # Coverage report
go tool task format        # Format code
```

## Configuration

See `config.yaml` (not committed, generated from environment variables):

```yaml
server:
  listen: ':8128'
database:
  dsn: postgres://user:pass@host:5432/flowbot_registry?sslmode=disable
registry:
  url: http://localhost:5000
auth:
  jwt_private_key_path: ./private.pem
```

## License

GPL v3
