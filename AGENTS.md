# Flowbot Registry

Plugin Registry — OCI-based plugin marketplace for the Flowbot ecosystem. Provides plugin publish, discovery, installation, and OCI token authentication.

## Dir Reference

| Task              | Location               | Notes                                              |
| ----------------- | ---------------------- | -------------------------------------------------- |
| HTTP API server   | `cmd/server/`          | Fiber v3 entry point, routes, database init        |
| CLI tool          | `cmd/cli/`             | Cobra CLI (publish, install, search commands)      |
| Ent schemas       | `internal/ent/`        | ent ORM schema definitions for DB tables           |
| HTTP handlers     | `internal/handler/`    | Request parsing, validation, response formatting   |
| Business logic    | `internal/service/`    | Auth token issuance, plugin publish orchestration  |
| OCI integration   | `pkg/oci/`             | OCI registry client (manifest pull, layer extract) |
| JWT utilities     | `pkg/jwt/`             | RS256 JWT signing per Docker Registry v2 Token spec|
| GitHub Actions    | `.github/workflows/`   | CI: lint, test, build                              |

## Key Patterns

- **Format**: run command `go tool task format`
- **Lint**: `revive` (strict, see `revive.toml`); also `testifylint` and `actionlint`
- **Imports**: stdlib → third-party → internal
- **Naming**: packages lowercase, types CamelCase
- **Errors**: Wrap with `%w`, use sentinel errors for known conditions
- **JSON**: Use `github.com/bytedance/sonic` for Marshal/Unmarshal. `json.RawMessage` from stdlib is allowed.
- **Configuration**: `viper` + yaml, loaded from `config.yaml` or environment variables
- **Database**: All queries through ent generated client; migrations via `client.Schema.Create()` on startup; use transactions for multi-step operations
- **TDD**: Table-driven tests co-located with source (`*_test.go`). Each test must use `t.Run(tt.name, ...)` pattern. Minimum 3 cases per table. Happy path first, error cases required.
- **OCI Auth Flow**: When CLI encounters 401 from OCI registry, request a token from the store's auth endpoint, then retry with Bearer token.

## Anti-Patterns

- Never use `panic` outside initialization
- Never ignore errors (assign to `_` or handle)
- Never edit ent generated code
- Never hardcode credentials, secrets, or keys
- Never use `encoding/json` — use `github.com/bytedance/sonic`
- Never write SQL queries outside ent — use the generated client
- Never commit `private.pem` or other secrets

## Build & Test Commands

```bash
go tool task build            # Build server binary
go tool task build:cli        # Build CLI binary
go tool task lint             # Code lint (revive + testifylint + actionlint)
go tool task test             # Unit tests
go tool task test:race        # Race condition tests
go tool task test:coverage    # Coverage report
go tool task ent              # Generate ent code from schemas
```

## Configuration

- Runtime: `config.yaml`
- Build: `taskfile.yaml`
- Lint: `revive.toml`
- CI: `.github/workflows/lint.yml`, `test.yml`, `build.yml`

## Notes

- Go 1.26.3+, PostgreSQL required
- Do not use emojis
- Run lint and test after modifying code
- Text in English: comments, docs, commit messages
- Code must have TDD tests
- In functions, variables, structs, interfaces, etc., must be commented using godoc. These comments should explain "what" and "why," without repeating "how.", and should be kept synchronized with the code.
- NEVER git commit unless asked.
