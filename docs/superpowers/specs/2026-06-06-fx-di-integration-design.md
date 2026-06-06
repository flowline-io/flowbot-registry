# fx DI Integration Design

## Summary

Replace the manual dependency wiring in `cmd/server/main.go` with `go.uber.org/fx` using a modular, per-package approach. Each package exports its own `fx.Module` or `fx.Option`, and `main.go` becomes a thin `fx.New(AllModules...).Run()` entry point.

## Scope

- **In**: `cmd/server/main.go` and all dependent packages (`pkg/store`, `pkg/jwt`, `pkg/oci`, `internal/service`, `internal/handler`, `internal/web`)
- **Out**: `cmd/cli/` (CLI tool wiring stays as-is)

## Dependency Graph

```
*viper.Viper              ── config singleton, provided by cmd/server
  ├── *ent.Client         ── opened from config DSN, closed on fx.Stop
  │   └── *store.Adapter  ── wraps ent client, NO fx.OnStop needed (client handles it)
  ├── *jwt.TokenService   ── reads keyPath/issuer/exp from viper
  ├── *oci.Client         ── reads registryURL from viper
  └── *fiber.App          ── starts with app.Listen on fx.Start, shutdown on fx.Stop

service.AuthService       ── depends on: jwt.TokenService + store.Adapter
service.PluginService     ── depends on: store.Adapter + oci.Client + registryURL(string)
handler.RegisterRoutes    ── depends on: fiber.App + AuthService + PluginService (fx.Invoke)
web.RegisterWebRoutes     ── depends on: fiber.App + store.Adapter (fx.Invoke)
```

## Per-Package fx Modules

### `cmd/server/fx.go` (new file)

- `fx.Provide` for `*viper.Viper`, `*ent.Client`, `*fiber.App`
- Aggregates all sub-modules into `func AllModules() fx.Option`
- `*ent.Client` lifecycle:
  1. After construction: run `client.Schema.Create(ctx)` via `fx.Invoke` (migration happens before any service starts)
  2. On shutdown: `client.Close()` via `fx.OnStop`
- `*fiber.App` lifecycle:
  - `fx.OnStart`: resolve listen addr from viper, call `app.Listen(listenAddr)` in a goroutine, print "server started" message
  - `fx.OnStop`: call `app.Shutdown()`
- `*viper.Viper` provided as a typed singleton (not `*viper.Viper` interface, since fx requires concrete types)

### `pkg/store/fx.go` (new file)

```go
var Module = fx.Module("store",
    fx.Provide(NewAdapter),
)
```

Straightforward: `NewAdapter` takes `*ent.Client` and returns `*Adapter`.

### `pkg/jwt/fx.go` (new file)

```go
var Module = fx.Module("jwt",
    fx.Provide(func(v *viper.Viper) (*TokenService, error) {
        return NewTokenService(
            v.GetString("auth.jwt_private_key_path"),
            v.GetString("auth.jwt_issuer"),
            time.Duration(v.GetInt("auth.jwt_expiration"))*time.Second,
        )
    }),
)
```

Factory function reads config from viper and delegates to existing `NewTokenService`.

### `pkg/oci/fx.go` (new file)

```go
var Module = fx.Module("oci",
    fx.Provide(func(v *viper.Viper) *Client {
        url := v.GetString("registry.url")
        if url == "" {
            url = "http://localhost:5000"
        }
        return NewClient(url)
    }),
)
```

Factory reads `registry.url` from viper with default fallback.

### `internal/service/fx.go` (new file)

```go
var Module = fx.Module("service",
    fx.Provide(
        NewAuthService,
        func(a *store.Adapter, ociClient *oci.Client, v *viper.Viper) *PluginService {
            url := v.GetString("registry.url")
            return NewPluginService(a, ociClient, url)
        },
    ),
)
```

`AuthService` constructor signature matches fx: `NewAuthService(*jwt.TokenService, *store.Adapter) *AuthService`.

`PluginService` needs an adapter because the existing `NewPluginService` takes `registryURL string` as a third param; the fx factory reads it from viper.

### `internal/handler/fx.go` (new file)

```go
var Module = fx.Module("handler",
    fx.Invoke(RegisterRoutes),
)
```

`RegisterRoutes(*fiber.App, *AuthService, *PluginService)` is invoked after all providers are ready. fx.Invoke runs after all fx.Provide calls complete.

### `internal/web/fx.go` (new file)

```go
var Module = fx.Module("web",
    fx.Invoke(RegisterWebRoutes),
)
```

Same pattern: `RegisterWebRoutes(*fiber.App, *store.Adapter)` invoked after construction.

## PGX Driver Registration

`sql.Register("postgres", stdlib.GetDefaultDriver())` moves from `init()` into the `newEntClient` constructor. This eliminates the package-level `init()` and keeps all side effects within the fx graph.

```go
func newEntClient(v *viper.Viper) (*ent.Client, error) {
    sql.Register("postgres", stdlib.GetDefaultDriver())
    dsn := v.GetString("database.dsn")
    return ent.Open("postgres", dsn)
}
```

`ent.Open` depends on the driver being registered, and calling it inline within the same constructor guarantees correct ordering.

## `cmd/server/main.go` After Refactor

```go
func main() {
    slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
    fx.New(AllModules()).Run()
}
```

All wiring, lifecycle hooks, side-effects, and graceful shutdown handled by fx.

## Config Defaults

All config defaults currently in `loadConfig()` must move into the viper provider:

```go
func newViper() *viper.Viper {
    v := viper.New()
    v.SetConfigName("config")
    v.SetConfigType("yaml")
    v.AddConfigPath(".")
    v.SetDefault("server.listen", ":8080")
    v.SetDefault("auth.jwt_private_key_path", "./private.pem")
    v.SetDefault("auth.jwt_expiration", 3600)
    v.SetDefault("auth.jwt_issuer", "flowbot-registry")
    v.SetDefault("registry.url", "http://localhost:5000")
    v.AutomaticEnv()
    if err := v.ReadInConfig(); err != nil {
        // same error handling as before
    }
    return v
}
```

## Files Changed

| File                     | Change                                      |
| ------------------------ | ------------------------------------------- |
| `cmd/server/main.go`     | Rewrite to ~5-line fx entry point           |
| `cmd/server/fx.go`       | NEW — all top-level provides + AllModules() |
| `pkg/store/fx.go`        | NEW — store module                          |
| `pkg/jwt/fx.go`          | NEW — jwt module                            |
| `pkg/oci/fx.go`          | NEW — oci module                            |
| `internal/service/fx.go` | NEW — service module                        |
| `internal/handler/fx.go` | NEW — handler module                        |
| `internal/web/fx.go`     | NEW — web module                            |
| `go.mod`                 | Add `go.uber.org/fx` dependency             |

## Error Handling

- `ent.Open` failure → fx.New returns error, `fx.Run` exits
- `jwt.NewTokenService` failure → fx provide error, app fails to start
- `app.Listen` failure → fx.OnStart error, app fails to start
- All errors logged via fx.Logger (which wraps slog)

## Testing Strategy

- Each package's fx module tested with `fxtest.New(t, module)` to verify construction succeeds
- Existing unit tests unchanged (constructors still work standalone)
- New test for `AllModules()` to verify the full app starts and shuts down cleanly
