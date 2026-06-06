# Handlers

HTTP request handlers for the registry API. Handles request parsing, validation, response formatting, and error mapping.

## Structure

```
handler/
├── routes.go   # Route registration
├── auth.go     # GET /api/v1/auth/token
└── plugin.go   # POST publish, GET list, GET version
```

## Patterns

- Handlers are thin: parse request, call service, format response.
- Business logic lives in `internal/service/`.
- Error discrimination: `ErrNotFound` -> 404, `ErrForbidden` -> 403, `ErrInvalidInput` -> 400, others -> 500 with sanitized message.
- All errors logged via `log/slog` before returning HTTP response.
- `Routes.go` wires all handlers to the Fiber app.

## Routes

| Method | Path                                                 | Handler            |
| ------ | ---------------------------------------------------- | ------------------ |
| GET    | `/api/v1/auth/token`                                 | AuthTokenHandler   |
| POST   | `/api/v1/plugins/:namespace/:name/publish`           | PublishHandler     |
| GET    | `/api/v1/plugins`                                    | ListPluginsHandler |
| GET    | `/api/v1/plugins/:namespace/:name/versions/:version` | GetVersionHandler  |

## Testing

```bash
go test ./internal/handler/...
```
