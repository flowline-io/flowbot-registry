# Service Layer

Business logic for authentication and plugin operations. Orchestrates OCI manifest fetching, manifest validation, database upserts, and JWT token issuance.

## Structure

```
service/
├── service.go       # AuthService, PluginService, scope parsing
└── service_test.go  # Scope parsing tests
```

## Components

### AuthService

Handles Docker Registry v2 token authentication. Parses scope strings, validates namespace access, and issues RS256 JWT tokens.

- `ParseScopes(raw string) — parses "repository:ns/name:pull,push" into structured entries
- `IssueJWT(ctx, service, clientID, rawScope) — validates namespace, signs JWT

### PluginService

Handles plugin publishing. Fetches OCI manifests, extracts plugin.yaml and README.md, validates version and name, and upserts records in a transaction.

Publish flow:

1. Fetch OCI manifest by digest
2. Extract `plugin.yaml` and `README.md` from layers
3. Validate `plugin.yaml` exists, version matches, name matches request path
4. Upsert Namespace, Plugin, PluginVersion in a single database transaction

## Rules

- All database operations in publish flow use transactions (`TxAdapter`).
- Errors use sentinels: `ErrNotFound`, `ErrForbidden`, `ErrInvalidInput`.
- Logging via `log/slog` at key decision points.

## Testing

```bash
go test ./internal/service/...
```
