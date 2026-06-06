# fx DI Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace manual dependency wiring in `cmd/server/main.go` with `go.uber.org/fx` modular DI.

**Architecture:** Each package exports an `fx.Module` that provides its constructors. `cmd/server/fx.go` aggregates all modules and provides top-level components (viper config, ent client, fiber app). `main.go` becomes a 3-line `fx.New(AllModules()).Run()` call.

**Tech Stack:** `go.uber.org/fx`, existing ent/fiber/viper/pgx stack

**Testing note:** Module tests verify compilation (module is a valid `fx.Option`). Full wiring is tested by the integration test in Task 8.

---

## File Map

| Action | File                          | Purpose                            |
| ------ | ----------------------------- | ---------------------------------- |
| Create | `pkg/store/fx.go`             | store.Module                       |
| Create | `pkg/store/fx_test.go`        | compilation test                   |
| Create | `pkg/jwt/fx.go`               | jwt.Module                         |
| Create | `pkg/jwt/fx_test.go`          | compilation test                   |
| Create | `pkg/oci/fx.go`               | oci.Module                         |
| Create | `pkg/oci/fx_test.go`          | compilation test                   |
| Create | `internal/service/fx.go`      | service.Module                     |
| Create | `internal/service/fx_test.go` | compilation test                   |
| Create | `internal/handler/fx.go`      | handler.Module                     |
| Create | `internal/handler/fx_test.go` | compilation test                   |
| Create | `internal/web/fx.go`          | web.Module                         |
| Create | `internal/web/fx_test.go`     | compilation test                   |
| Create | `cmd/server/fx.go`            | top-level providers + AllModules() |
| Create | `cmd/server/fx_test.go`       | integration test                   |
| Modify | `cmd/server/main.go`          | rewrite to 3-line fx entry         |
| Modify | `go.mod` / `go.sum`           | add `go.uber.org/fx` dep           |

---

### Task 1: Add fx dependency

**Files:**

- Modify: `go.mod`

- [ ] **Step 1: Add `go.uber.org/fx` to go.mod**

```bash
go get go.uber.org/fx@latest
```

- [ ] **Step 2: Verify build compiles (no usage yet, just dependency added)**

```bash
go build ./...
```

Expected: builds successfully, no new errors.

---

### Task 2: pkg/store fx module

**Files:**

- Create: `pkg/store/fx.go`
- Create: `pkg/store/fx_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/store/fx_test.go`:

```go
package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

func TestModule(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, Module)
	_ = fx.Options(Module)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./pkg/store/ -run TestModule -v
```

Expected: FAIL — `Module` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `pkg/store/fx.go`:

```go
package store

import "go.uber.org/fx"

// Module provides the store adapter via fx dependency injection.
// Provides both *Adapter and the StoreQuerier interface for consumers
// that depend on the interface (e.g. web handlers).
var Module = fx.Module("store",
	fx.Provide(
		NewAdapter,
		func(a *Adapter) StoreQuerier { return a },
	),
)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./pkg/store/ -run TestModule -v
```

Expected: PASS.

---

### Task 3: pkg/jwt fx module

**Files:**

- Create: `pkg/jwt/fx.go`
- Create: `pkg/jwt/fx_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/jwt/fx_test.go`:

```go
package jwt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

func TestModule(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, Module)
	_ = fx.Options(Module)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./pkg/jwt/ -run TestModule -v
```

Expected: FAIL — `Module` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `pkg/jwt/fx.go`:

```go
package jwt

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Module provides the jwt token service via fx dependency injection.
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

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./pkg/jwt/ -run TestModule -v
```

Expected: PASS.

---

### Task 4: pkg/oci fx module

**Files:**

- Create: `pkg/oci/fx.go`
- Create: `pkg/oci/fx_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/oci/fx_test.go`:

```go
package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

func TestModule(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, Module)
	_ = fx.Options(Module)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./pkg/oci/ -run TestModule -v
```

Expected: FAIL — `Module` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `pkg/oci/fx.go`:

```go
package oci

import (
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Module provides the OCI registry client via fx dependency injection.
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

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./pkg/oci/ -run TestModule -v
```

Expected: PASS.

---

### Task 5: internal/service fx module

**Files:**

- Create: `internal/service/fx.go`
- Create: `internal/service/fx_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/service/fx_test.go`:

```go
package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

func TestModule(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, Module)
	_ = fx.Options(Module)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/ -run TestModule -v
```

Expected: FAIL — `Module` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/service/fx.go`:

```go
package service

import (
	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/flowline-io/flowbot-registry/pkg/store"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Module provides the service layer via fx dependency injection.
var Module = fx.Module("service",
	fx.Provide(
		NewAuthService,
		func(a *store.Adapter, ociClient *oci.Client, v *viper.Viper) *PluginService {
			url := v.GetString("registry.url")
			if url == "" {
				url = "http://localhost:5000"
			}
			return NewPluginService(a, ociClient, url)
		},
	),
)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/service/ -run TestModule -v
```

Expected: PASS.

---

### Task 6: internal/handler fx module

**Files:**

- Create: `internal/handler/fx.go`
- Create: `internal/handler/fx_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/handler/fx_test.go`:

```go
package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

func TestModule(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, Module)
	_ = fx.Options(Module)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestModule -v
```

Expected: FAIL — `Module` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/handler/fx.go`:

```go
package handler

import "go.uber.org/fx"

// Module registers HTTP API routes via fx dependency injection.
var Module = fx.Module("handler",
	fx.Invoke(RegisterRoutes),
)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestModule -v
```

Expected: PASS.

---

### Task 7: internal/web fx module

**Files:**

- Create: `internal/web/fx.go`
- Create: `internal/web/fx_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/web/fx_test.go`:

```go
package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

func TestModule(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, Module)
	_ = fx.Options(Module)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/web/ -run TestModule -v
```

Expected: FAIL — `Module` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/web/fx.go`:

```go
package web

import "go.uber.org/fx"

// Module registers web UI routes via fx dependency injection.
var Module = fx.Module("web",
	fx.Invoke(RegisterWebRoutes),
)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/web/ -run TestModule -v
```

Expected: PASS.

---

### Task 8: cmd/server fx wiring + main.go rewrite + integration test

**Files:**

- Create: `cmd/server/fx.go`
- Create: `cmd/server/fx_test.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write the integration test**

Create `cmd/server/fx_test.go`:

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestAllModules(t *testing.T) {
	t.Parallel()

	app := fxtest.New(t, AllModules())
	app.RequireStart()
	app.RequireStop()
}

func TestAllModulesIsFxOption(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, AllModules())
	_ = fx.Options(AllModules())
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/server/ -run TestAllModules -v
```

Expected: FAIL — `AllModules` undefined.

- [ ] **Step 3: Write cmd/server/fx.go**

Create `cmd/server/fx.go`:

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/flowline-io/flowbot-registry/internal/ent"
	"github.com/flowline-io/flowbot-registry/internal/handler"
	"github.com/flowline-io/flowbot-registry/internal/service"
	"github.com/flowline-io/flowbot-registry/internal/web"
	"github.com/flowline-io/flowbot-registry/pkg/jwt"
	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/flowline-io/flowbot-registry/pkg/store"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// AllModules returns all fx options needed to start the server.
func AllModules() fx.Option {
	return fx.Options(
		fx.Provide(
			newViper,
			newEntClient,
			newFiberApp,
		),
		fx.Invoke(runMigrations),
		store.Module,
		jwt.Module,
		oci.Module,
		service.Module,
		handler.Module,
		web.Module,
	)
}

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
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Error("config file error", "error", err)
		}
	}

	return v
}

func newEntClient(v *viper.Viper, lc fx.Lifecycle) (*ent.Client, error) {
	sql.Register("postgres", stdlib.GetDefaultDriver())

	dsn := v.GetString("database.dsn")
	if dsn == "" {
		dsn = "postgres://flowbot:flowbot@localhost:5432/flowbot_registry?sslmode=disable"
	}

	slog.Info("connecting to database")
	client, err := ent.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}

func runMigrations(client *ent.Client) error {
	slog.Info("running database migrations")
	return client.Schema.Create(context.Background())
}

func newFiberApp(v *viper.Viper, lc fx.Lifecycle) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "flowbot-registry",
	})

	listen := v.GetString("server.listen")
	if listen == "" {
		listen = ":8080"
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				slog.Info("server starting", "listen", listen)
				if err := app.Listen(listen); err != nil {
					slog.Error("server error", "error", err)
				}
			}()
			_, _ = fmt.Printf("flowbot-registry server started on %s\n", listen)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			slog.Info("shutting down")
			if err := app.Shutdown(); err != nil {
				slog.Error("shutdown error", "error", err)
			}
			slog.Info("server stopped")
			return nil
		},
	})

	return app
}
```

- [ ] **Step 4: Rewrite cmd/server/main.go**

Replace `cmd/server/main.go` with:

```go
// Package main is the entry point for the flowbot-registry HTTP API server.
package main

import (
	"log/slog"
	"os"

	"go.uber.org/fx"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	fx.New(AllModules()).Run()
}
```

- [ ] **Step 5: Run integration tests**

```bash
go test ./cmd/server/ -run TestAllModules -v
```

The `TestAllModules` test will fail if no database is available — this is expected and proves wiring works. The `TestAllModulesIsFxOption` test passes without a database.

- [ ] **Step 6: Build and verify the binary compiles**

```bash
go build ./cmd/server/
```

Expected: builds successfully.

---

### Task 9: Run full test suite and lint

**Files:** None (verification only)

- [ ] **Step 1: Run all tests**

```bash
go test ./... -count=1
```

Expected: all tests PASS.

- [ ] **Step 2: Run lint**

```bash
go tool task lint
```

Expected: no new lint errors.

- [ ] **Step 3: Run format**

```bash
go tool task format
```

Expected: no formatting changes needed.

---

### Task 10: Run all tests with race detector

**Files:** None (verification only)

- [ ] **Step 1: Run race tests**

```bash
go tool task test:race
```

Expected: PASS, no race conditions.
