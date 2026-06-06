# User Authentication System Design

**Date**: 2026-06-06
**Status**: Draft

## Overview

Add user registration, login, JWT-based session management, and auth middleware to the Flowbot Registry. Currently the system has no user concept — only a Docker Registry v2 JWT token endpoint and a simple API key passthrough.

## Design Decisions

| Decision               | Choice                                      | Rationale                                       |
| ---------------------- | ------------------------------------------- | ----------------------------------------------- |
| User identity          | email + password                            | Simple, email serves as unique identifier       |
| Password hashing       | bcrypt (cost=12)                            | Industry standard, Go stdlib compatible         |
| Session management     | JWT access token (1h) + refresh token (7d)  | Stateless, no session table needed at runtime   |
| User-Namespace binding | 1:1, auto-create on register                | Matches existing namespace-per-user model       |
| Protected routes       | Write operations only (publish)             | Read operations remain public for discovery     |
| CLI auth               | `flowbot login` command with token file     | Interactive login, auto-refresh                 |
| JWT signing            | RS256 (separate from Docker v2 JWT service) | Different claims structure, reuse same key pair |

## 1. Data Model

### User (new)

| Field         | Type   | Constraints       |
| ------------- | ------ | ----------------- |
| id            | int    | PK, immutable     |
| email         | string | unique, not empty |
| password_hash | string | not empty         |
| created_at    | time   | auto              |
| updated_at    | time   | auto              |

### RefreshToken (new)

| Field      | Type   | Constraints                  |
| ---------- | ------ | ---------------------------- |
| id         | int    | PK, immutable                |
| user_id    | int    | FK -> User, index            |
| token_hash | string | unique (SHA256 of raw token) |
| expires_at | time   | not null                     |
| created_at | time   | auto                         |

### Namespace (modified)

Add nullable `user_id` field (FK -> User).

- Namespace name is globally unique (existing DB constraint). Ownership cannot be claimed by registering with an existing name — whether that namespace is owned or unowned.
- New namespaces created on registration have `user_id` set.
- Existing namespaces have `user_id = NULL` (backward compatible). These are treated as **unowned** — see security rules below.
- When a user publishes to their namespace, `RequireNamespace` middleware checks `namespace.user_id == current_user_id`. If `user_id` is NULL, the request is rejected with 403 — unowned namespaces cannot be published to through normal user flow (must be assigned ownership via direct DB intervention or future admin API).

### Edges

- User -> RefreshTokens (one-to-many)
- User -> Namespaces (one-to-many, currently 1:1 in practice)
- Namespace -> User (many-to-one)

## 2. Service Layer

### UserService (`internal/service/auth_user.go`)

```go
type UserService struct {
    store      *store.Adapter
    jwtSvc     *jwt.UserTokenService
    privateKey *rsa.PrivateKey
}

// Register creates user + namespace + issues tokens.
func (s *UserService) Register(ctx, email, password string) (*AuthResult, error)

// Login validates credentials and issues tokens.
func (s *UserService) Login(ctx, email, password string) (*AuthResult, error)

// RefreshToken validates a refresh token and issues a new token pair.
// The old refresh token is deleted (rotation) to prevent reuse.
func (s *UserService) RefreshToken(ctx, rawRefreshToken string) (*AuthResult, error)

// ValidateAccessToken parses and validates an access token JWT.
// Used by auth middleware.
func (s *UserService) ValidateAccessToken(tokenStr string) (*AccessTokenClaims, error)
```

`AuthResult`: `{ User, AccessToken, RefreshToken, ExpiresAt }`

`AccessTokenClaims`: `{ UserID, Email, exp, iat, jti }`

### JWT: UserTokenService (`pkg/jwt/usertoken.go`)

Separate from the existing `TokenService` (Docker Registry v2). Dedicated to user auth tokens.

- `GenerateAccessToken(userID int, email string) (string, time.Time, error)` — RS256, 1h expiry
- `GenerateRefreshToken() (string, error)` — random 32-byte hex
- `ParseAccessToken(tokenStr string) (*AccessTokenClaims, error)` — validates and returns claims

### Password Hashing

```go
// bcrypt cost=12
func HashPassword(password string) (string, error)
func CheckPassword(hash, password string) bool
```

Password minimum length: 8 characters (enforced at handler level via `ValidatePassword`).

### Existing AuthService — Docker v2 Token Bridge

The `GET /api/v1/auth/token` endpoint is **modified** to act as the bridge between user authentication and OCI registry access. It is no longer unauthenticated.

**Auth flow:**

1. CLI (after `flowbot login`) requests `GET /api/v1/auth/token?scope=repository:namespace/plugin:push,pull&service=<registry>`
2. CLI includes `Authorization: Bearer <User_Access_Token>` header
3. `AuthTokenHandler` extracts and validates the User JWT via `UserService.ValidateAccessToken`
4. Parses the `scope` parameter to extract the namespace name
5. Queries the namespace and verifies `namespace.user_id == user_id_from_jwt` (this is the ownership check)
6. Only if ownership is confirmed, issues a Docker Registry v2 JWT with the requested actions (push/pull)
7. If no User token present, or namespace ownership mismatch, returns `401` or `403`

This means `AuthService` now depends on `UserService` (or directly on `store.StoreQuerier` and `jwt.UserTokenService`) for user auth validation.

**Existing callers:** The `AuthService.IssueJWT` signature changes to accept a `userID` parameter (0 for unauthenticated/public access, e.g., pull-only tokens for public plugins).

## 3. HTTP Layer

### New Endpoints

| Method | Path                    | Auth | Handler           |
| ------ | ----------------------- | ---- | ----------------- |
| POST   | `/api/v1/auth/register` | None | `RegisterHandler` |
| POST   | `/api/v1/auth/login`    | None | `LoginHandler`    |
| POST   | `/api/v1/auth/refresh`  | None | `RefreshHandler`  |

### New Middleware (`internal/middleware/auth.go`)

**`AuthRequired`**:

1. Extract `Authorization: Bearer <token>` header
2. Call `UserService.ValidateAccessToken(token)`
3. On success: inject `user_id`, `email` into `c.Locals()`
4. On failure: return `401 {"error": "unauthorized"}`

**`RequireNamespace`** (depends on `store.StoreQuerier`):

1. Read `user_id` from `c.Locals()`
2. Read `:namespace` from route params
3. Query namespace by name via `StoreQuerier.NamespaceGetByName`
4. If namespace.user_id is NULL (unowned) -> `403`
5. If `namespace.user_id != current_user_id` -> `403`
6. On match -> pass

### Error Mapping

| Condition                                        | HTTP Status |
| ------------------------------------------------ | ----------- |
| Missing/invalid token                            | 401         |
| Wrong password                                   | 401         |
| Email already registered                         | 409         |
| Weak password (< 8 chars)                        | 400         |
| Expired refresh token                            | 401         |
| Namespace access denied (mismatch or NULL owner) | 403         |
| Invalid JSON body                                | 400         |
| Rate limited (too many login attempts)           | 429         |

### Routes

```go
func RegisterRoutes(app *fiber.App, authSvc *service.AuthService, userSvc *service.UserService, pluginSvc *service.PluginService, registryURL string) {
    api := app.Group("/api/v1")

    // Public: Registry info, plugin listing
    api.Get("/registry", RegistryInfoHandler(registryURL))
    api.Get("/plugins", ListPluginsHandler(pluginSvc))
    api.Get("/plugins/:namespace/:name/versions/:version", GetVersionHandler(pluginSvc))

    // Public: User auth (register/login/refresh) — login endpoint has rate limiter
    api.Post("/auth/register", RegisterHandler(userSvc))
    api.Post("/auth/login", middleware.LoginRateLimit(), LoginHandler(userSvc))
    api.Post("/auth/refresh", RefreshHandler(userSvc))

    // Authenticated: Docker v2 token (requires User access token)
    auth := api.Group("", middleware.AuthRequired(userSvc))
    auth.Get("/auth/token", AuthTokenHandler(authSvc))

    // Authenticated: plugin publishing (requires namespace ownership)
    auth.Post("/plugins/:namespace/:name/publish", middleware.RequireNamespace(store), PublishHandler(pluginSvc))
}
```

### Rate Limiting on Login

A lightweight in-memory rate limiter middleware (`middleware.LoginRateLimit`) is applied to `POST /api/v1/auth/login`:

- Uses `fiber/middleware/limiter` (or equivalent token-bucket)
- Limits each IP to **10 requests per minute**
- Exceeding the limit returns `429 Too Many Requests`
- This provides basic brute-force protection without external dependencies (no Redis required).

## 4. Store Layer

### New Methods on Adapter

**User CRUD:**

```go
UserCreate(ctx, email, passwordHash string) (*UserRecord, error)
UserGetByEmail(ctx, email string) (*UserRecord, error)
UserGetByID(ctx, id int) (*UserRecord, error)
```

**RefreshToken CRUD:**

```go
RefreshTokenCreate(ctx, userID int, tokenHash string, expiresAt time.Time) (*RefreshTokenRecord, error)
RefreshTokenGetByHash(ctx, tokenHash string) (*RefreshTokenRecord, error)
RefreshTokenDeleteByID(ctx, id int) error
RefreshTokenDeleteExpired(ctx context.Context) (int64, error)
```

**Namespace extended:**

```go
// NamespaceCreate gains userID parameter. Pass 0 for unowned namespaces
// (existing callers updated to pass 0).
NamespaceCreate(ctx, name, nsType string, userID int) (*NamespaceRecord, error)
```

### Record Types

```go
type UserRecord struct {
    ID           int       `json:"id"`
    Email        string    `json:"email"`
    PasswordHash string    `json:"-"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type RefreshTokenRecord struct {
    ID        int       `json:"id"`
    UserID    int       `json:"user_id"`
    TokenHash string    `json:"-"`
    ExpiresAt time.Time `json:"expires_at"`
    CreatedAt time.Time `json:"created_at"`
}
```

## 5. CLI Login

### `flowbot login` command

```
Flowbot Plugin Registry

Usage: flowbot login [--store-url <url>]

Interactive: prompts for email and password, sends POST /api/v1/auth/login,
saves tokens to ~/.flowbot/config.json.
```

### Token Storage

Path: `~/.flowbot/config.json`

```json
{
  "store_url": "http://localhost:8128",
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_at": "2026-06-07T12:00:00Z"
}
```

**File permissions:** The config file is created with `0600` (owner read-write only) using `os.OpenFile` with explicit permissions. This prevents other users on the same machine from reading the JWT tokens.

### Auto-refresh

Before each API call, if `access_token` is expired (or within 5 min of expiry):

1. POST `/api/v1/auth/refresh` with `refresh_token`
2. Update config file with new token pair
3. Retry original request

`--api-key` flag and `FLOWBOT_API_KEY` env var continue to work and take priority over stored tokens.

## 6. Configuration

Add to `config.yaml`:

```yaml
auth:
  jwt_private_key_path: ./private.pem # existing
  jwt_expiration: 3600 # existing
  jwt_issuer: flowbot-registry # existing
  access_token_expiration: 3600 # new (user access token, seconds)
  refresh_token_expiration: 604800 # new (user refresh token, seconds, 7d)
```

## 7. File Layout

```
internal/ent/schema/
├── user.go              # NEW: User schema
├── refeshtoken.go       # NEW: RefreshToken schema
└── namespace.go         # MODIFY: add user_id field + edge

internal/handler/
├── auth_user.go         # NEW: register/login/refresh handlers
├── auth.go              # MODIFIED: bridge to user auth (validates User JWT before issuing Docker v2 token)
├── auth_user_test.go    # NEW: handler tests
├── routes.go            # MODIFY: new routes + middleware
└── ...

internal/service/
├── auth_user.go         # NEW: UserService
├── auth_user_test.go    # NEW: service tests
├── service.go           # MODIFIED: AuthService.IssueJWT takes userID
├── fx.go                # MODIFY: provide UserService
└── ...

internal/middleware/
├── auth.go              # NEW: AuthRequired, RequireNamespace
├── auth_test.go         # NEW: middleware tests
├── ratelimit.go         # NEW: LoginRateLimit middleware
└── ratelimit_test.go    # NEW: rate limit tests

pkg/store/
├── store.go             # MODIFY: add User/RefreshToken CRUD
├── errors.go            # unchanged
└── ...

pkg/jwt/
├── usertoken.go         # NEW: UserTokenService
├── usertoken_test.go    # NEW: jwt tests
├── jwt.go               # unchanged (Docker v2 tokens)
├── fx.go                # MODIFY: provide UserTokenService
└── ...

cmd/cli/
├── login.go             # NEW: login command
├── config.go            # NEW: token storage helpers
├── main.go              # MODIFY: register login subcommand
└── ...

cmd/server/
└── fx.go                # MODIFY: UserService depends on new config keys

config.yaml              # MODIFY: add token expiration configs
```

## 8. Testing Strategy

Table-driven tests (minimum 3 cases each). TDD: tests written before implementation.

| Module                | Test File           | Cases                                                                                                                             |
| --------------------- | ------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/store`           | `store_test.go`     | User create success, duplicate email, get by email (found/not found); RefreshToken create, get by hash, delete expired            |
| `pkg/jwt`             | `usertoken_test.go` | Access token generate + parse valid, expired token, tampered token                                                                |
| `internal/service`    | `auth_user_test.go` | Register success, duplicate email, weak password; Login success, wrong password, user not found; Refresh valid, expired, tampered |
| `internal/handler`    | `auth_user_test.go` | Register 201, 409, 400; Login 200, 401; Refresh 200, 401                                                                          |
| `internal/middleware` | `auth_test.go`      | No token -> 401, valid token -> pass, invalid token -> 401, wrong namespace -> 403                                                |

## 9. Migration

- Adding `user` and `refresh_token` tables, adding nullable `user_id` to `namespace` — auto-migrated via `client.Schema.Create()` on startup.
- No data migration needed; existing records remain compatible.
- Existing `GET /api/v1/auth/token` now requires User authentication (see Docker v2 Token Bridge). Unauthenticated callers cannot obtain push-scoped tokens.

## 10. Out of Scope

- Email verification, password reset
- OAuth / GitHub login
- Organization/multi-user namespace management
- Role-based access control (admin, member, etc.)
- Account lockout after failed attempts
- Admin API for assigning ownership to unowned namespaces (requires direct DB intervention for now)

## 11. Known V1 Technical Debt

**Token Rotation Race Condition:** When `/api/v1/auth/refresh` executes `Delete(old) + Create(new)`, two concurrent refresh requests can cause the second request to fail (old token already deleted) and force the client to re-login. This is acceptable for V1. Future mitigation: introduce a 1-minute grace period where the old refresh token is marked revoked rather than immediately deleted, allowing the legitimate second request to succeed while still preventing token reuse after the grace window.
