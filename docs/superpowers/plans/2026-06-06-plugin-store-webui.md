# Plugin Store Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a server-rendered web UI plugin store to the Flowbot Registry using templ + htmx + daisyUI, served by the existing Fiber v3 server.

**Architecture:** New `internal/web/` package with templ templates for pages and components, web handlers that query `store.Adapter` directly (via a `StoreQuerier` interface), and htmx fragment endpoints for dynamic interactions. Existing `/api/v1/*` JSON API routes remain unchanged.

**Tech Stack:** Go 1.26, Fiber v3, ent ORM, `github.com/a-h/templ`, htmx v2, daisyUI v5 + Tailwind CSS (CDN), `github.com/stretchr/testify`

**File Structure:**

```
internal/web/
├── render.go                          # Fiber templ adapter (package web)
├── render_test.go                     # Render adapter tests
├── handlers.go                        # Page handlers: Browse, Namespace, Detail
├── handlers_test.go                   # Page handler tests
├── htmx.go                            # htmx fragment handlers: search, pagination, version
├── htmx_test.go                       # htmx fragment handler tests
└── templates/
    ├── layout.templ                   # Base HTML shell (package templates)
    ├── pages/
    │   ├── browse.templ               # Browse page (package pages)
    │   ├── detail.templ               # Detail page (package pages)
    │   └── namespace.templ            # Namespace page (package pages)
    └── components/
        ├── plugin_card.templ          # Plugin card (package components)
        ├── search_bar.templ           # Search bar with htmx (package components)
        ├── pagination.templ           # Load-more button (package components)
        ├── version_list.templ         # Version selector (package components)
        └── readme_render.templ        # README display (package components)

pkg/store/
├── store.go                           # +3 new methods, +1 interface
└── store_test.go                      # Tests for new methods

cmd/server/main.go                     # +web route registration
taskfile.yaml                          # +build:templ task
```

---

### Task 1: Add templ dependency and Fiber render adapter

**Files:**

- Modify: `go.mod` (via `go get`)
- Create: `internal/web/render.go`

- [ ] **Step 1: Add templ dependency**

```bash
go get github.com/a-h/templ
```

- [ ] **Step 2: Create render.go with Fiber adapter**

Create `internal/web/render.go`:

```go
// Package web contains the web UI handlers and templ rendering for the plugin store.
package web

import (
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v3"
)

// Render renders a templ component as an HTML response on a Fiber context.
func Render(c fiber.Ctx, component templ.Component) error {
	c.Response().Header.SetContentType("text/html; charset=utf-8")
	return component.Render(c.Context(), c.Response().BodyWriter())
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/web/...
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/web/render.go
git commit -m "feat: add templ dependency and Fiber render adapter"
```

---

### Task 2: Add new store methods to pkg/store

**Files:**

- Modify: `pkg/store/store.go`

- [ ] **Step 1: Add NamespaceGetByID method**

Append to `pkg/store/store.go`, after the existing `NamespaceGetByName` method:

```go
// NamespaceGetByID retrieves a namespace by its ID.
func (a *Adapter) NamespaceGetByID(ctx context.Context, id int) (*NamespaceRecord, error) {
	n, err := a.client.Namespace.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: namespace id %d", ErrNotFound, id)
		}
		return nil, fmt.Errorf("get namespace by id: %w", err)
	}
	return &NamespaceRecord{
		ID:   n.ID,
		Name: n.Name,
		Type: n.Type,
	}, nil
}
```

- [ ] **Step 2: Add PluginVersionListByPlugin method**

Append to `pkg/store/store.go`, after `PluginVersionUpdate`:

```go
// PluginVersionListByPlugin returns all versions for a plugin, newest first.
func (a *Adapter) PluginVersionListByPlugin(ctx context.Context, pluginID int) ([]PluginVersionRecord, error) {
	pvs, err := a.client.PluginVersion.Query().
		Where(pluginversion.PluginIDEQ(pluginID)).
		Order(ent.Desc(pluginversion.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list plugin versions: %w", err)
	}

	var records []PluginVersionRecord
	for _, pv := range pvs {
		records = append(records, *pluginVersionToRecord(pv))
	}

	return records, nil
}
```

Requires adding `"entgo.io/ent"` to the imports (for `ent.Desc`).

- [ ] **Step 3: Add PluginListByNamespace method**

Append to `pkg/store/store.go`:

```go
// PluginListByNamespace returns plugins in a namespace with optional search and pagination.
func (a *Adapter) PluginListByNamespace(ctx context.Context, namespaceID int, query string, limit, offset int) ([]PluginRecord, int, error) {
	q := a.client.Plugin.Query().Where(plugin.NamespaceIDEQ(namespaceID))
	if query != "" {
		q = q.Where(plugin.NameContainsFold(query))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count plugins by namespace: %w", err)
	}

	ps, err := q.Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list plugins by namespace: %w", err)
	}

	var records []PluginRecord
	for _, p := range ps {
		records = append(records, PluginRecord{
			ID:          p.ID,
			NamespaceID: p.NamespaceID,
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Description: p.Description,
			LogoURL:     p.LogoURL,
		})
	}

	return records, total, nil
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./pkg/store/...
```

- [ ] **Step 5: Commit**

```bash
git add pkg/store/store.go
git commit -m "feat: add NamespaceGetByID, PluginVersionListByPlugin, PluginListByNamespace store methods"
```

---

### Task 3: Extract StoreQuerier interface for testability

**Files:**

- Modify: `pkg/store/store.go`

- [ ] **Step 1: Add StoreQuerier interface**

Add after the record type definitions (after line 39) in `pkg/store/store.go`:

```go
// StoreQuerier defines read operations for plugin store queries.
// Adapter implements this interface, enabling mock injection in tests.
type StoreQuerier interface {
	NamespaceGetByName(ctx context.Context, name string) (*NamespaceRecord, error)
	NamespaceGetByID(ctx context.Context, id int) (*NamespaceRecord, error)
	PluginGetByNamespaceAndName(ctx context.Context, namespaceID int, name string) (*PluginRecord, error)
	PluginList(ctx context.Context, query string, limit, offset int) ([]PluginRecord, int, error)
	PluginListByNamespace(ctx context.Context, namespaceID int, query string, limit, offset int) ([]PluginRecord, int, error)
	PluginVersionListByPlugin(ctx context.Context, pluginID int) ([]PluginVersionRecord, error)
	PluginVersionGetByPluginAndVersion(ctx context.Context, pluginID int, version string) (*PluginVersionRecord, error)
}
```

- [ ] **Step 2: Verify interface compliance**

```bash
go build ./pkg/store/...
```

The compiler will confirm that `*Adapter` satisfies `StoreQuerier` at the point of use in handlers.

- [ ] **Step 3: Commit**

```bash
git add pkg/store/store.go
git commit -m "feat: add StoreQuerier interface for testable web handlers"
```

---

### Task 4: Create layout.templ base template

**Files:**

- Create: `internal/web/templates/layout.templ`

- [ ] **Step 1: Create layout.templ**

Create `internal/web/templates/layout.templ`:

```templ
package templates

templ Layout() {
	<html data-theme="light" lang="en">
		<head>
			<meta charset="UTF-8"/>
			<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
			<title>Flowbot Registry — Plugin Store</title>
			<link href="https://cdn.jsdelivr.net/npm/daisyui@5.0.0/full.css" rel="stylesheet" type="text/css"/>
			<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>
			<script src="https://unpkg.com/htmx.org@2.0.4" integrity="sha384-HGfTtf4xJwT7Dkj2G50mJdh6zqDbkxFl8Ft5cI9Z5xcGZErYI5Icrk+2CkWRIEw" crossorigin="anonymous"></script>
		</head>
		<body class="min-h-screen bg-base-200">
			<div class="navbar bg-base-100 shadow-sm">
				<div class="flex-1">
					<a href="/" class="btn btn-ghost text-xl normal-case">Flowbot Registry</a>
				</div>
				<div class="flex-none gap-2">
					<form action="/" method="get">
						<label class="input input-bordered flex items-center gap-2">
							<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="h-4 w-4 opacity-70">
								<path fill-rule="evenodd" d="M9.965 11.026a5 5 0 1 1 1.06-1.06l2.755 2.754a.75.75 0 1 1-1.06 1.06l-2.755-2.754ZM10.5 7a3.5 3.5 0 1 1-7 0 3.5 3.5 0 0 1 7 0Z" clip-rule="evenodd"/>
							</svg>
							<input type="search" name="q" placeholder="Search plugins..."
							       class="grow" value=""
							       hx-get="/web/search"
							       hx-trigger="keyup changed delay:300ms"
							       hx-target="#plugin-grid"
							       hx-swap="innerHTML"/>
						</form>
					</form>
				</div>
			</div>
			<main class="container mx-auto px-4 py-8">
				{ children... }
			</main>
		</body>
	</html>
}
```

- [ ] **Step 2: Generate templ code**

```bash
go run github.com/a-h/templ/cmd/templ@latest generate
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/web/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/web/templates/layout.templ internal/web/templates/layout_templ.go
git commit -m "feat: add base layout template with navbar and CDN links"
```

---

### Task 5: Create plugin_card component

**Files:**

- Create: `internal/web/templates/components/plugin_card.templ`

- [ ] **Step 1: Create plugin_card.templ**

Create `internal/web/templates/components/plugin_card.templ`:

```templ
package components

templ PluginCard(namespaceName string, name string, displayName string, description string, logoURL string) {
	<a href={ "/" + namespaceName + "/" + name } class="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
		<div class="card-body p-5">
			<div class="flex items-start gap-3">
				if logoURL != "" {
					<figure class="w-12 h-12 rounded-lg overflow-hidden shrink-0 bg-base-300">
						<img src={ logoURL } alt={ displayName } class="w-full h-full object-cover"/>
					</figure>
				} else {
					<div class="w-12 h-12 rounded-lg shrink-0 bg-primary/10 flex items-center justify-center">
						<span class="text-primary font-bold text-lg">{ pluginInitial(displayName, name) }</span>
					</div>
				}
				<div class="min-w-0">
					<h2 class="card-title text-base">
						if displayName != "" {
							{ displayName }
						} else {
							{ name }
						}
					</h2>
					<div class="badge badge-sm badge-outline">{ namespaceName }</div>
				</div>
			</div>
			if description != "" {
				<p class="text-sm text-base-content/70 line-clamp-2 mt-2">{ description }</p>
			}
		</div>
	</a>
}

func pluginInitial(displayName, name string) string {
	fallback := name
	if displayName != "" {
		fallback = displayName
	}
	if len(fallback) > 0 {
		return string([]rune(fallback)[0:1])
	}
	return "?"
}
```

- [ ] **Step 2: Generate templ code**

```bash
go run github.com/a-h/templ/cmd/templ@latest generate
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/web/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/web/templates/components/plugin_card.templ internal/web/templates/components/plugin_card_templ.go
git commit -m "feat: add plugin card templ component"
```

---

### Task 6: Create search_bar and pagination components

**Files:**

- Create: `internal/web/templates/components/search_bar.templ`
- Create: `internal/web/templates/components/pagination.templ`

- [ ] **Step 1: Create search_bar.templ**

Create `internal/web/templates/components/search_bar.templ`:

```templ
package components

templ SearchBar(query string) {
	<div class="mb-6">
		<label class="input input-bordered flex items-center gap-2 w-full max-w-md">
			<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="h-4 w-4 opacity-70">
				<path fill-rule="evenodd" d="M9.965 11.026a5 5 0 1 1 1.06-1.06l2.755 2.754a.75.75 0 1 1-1.06 1.06l-2.755-2.754ZM10.5 7a3.5 3.5 0 1 1-7 0 3.5 3.5 0 0 1 7 0Z" clip-rule="evenodd"/>
			</svg>
			<input type="search" name="q" placeholder="Search plugins..."
			       class="grow" value={ query }
			       hx-get="/web/search"
			       hx-trigger="keyup changed delay:300ms"
			       hx-target="#plugin-grid"
			       hx-swap="innerHTML"/>
		</label>
	</div>
}
```

- [ ] **Step 2: Create pagination.templ**

Create `internal/web/templates/components/pagination.templ`:

```templ
package components

templ LoadMoreButton(offset int, limit int, total int, query string) {
	if offset+limit < total {
		<div class="flex justify-center mt-8" id="load-more-container">
			<button class="btn btn-outline"
			        hx-get={ templ.URL("/web/plugins?offset=" + fmt.Sprintf("%d", offset+limit) + "&limit=" + fmt.Sprintf("%d", limit) + "&q=" + query) }
			        hx-trigger="click"
			        hx-target="this"
			        hx-swap="outerHTML">
				Load More
			</button>
		</div>
	}
}

func fmt.Sprintf(format string, a ...any) string {
	// Redirect to standard library.
	return ""
}
```

- [ ] **Step 3: Fix pagination.templ to use proper Go fmt**

templ renders Go expressions directly. Replace the `templ.URL` + `fmt.Sprintf` approach. Rewrite pagination.templ:

```templ
package components

import "fmt"

templ LoadMoreButton(offset int, limit int, total int, query string) {
	if offset+limit < total {
		<div class="flex justify-center mt-8" id="load-more-container">
			<button class="btn btn-outline"
			        hx-get={ fmt.Sprintf("/web/plugins?offset=%d&limit=%d&q=%s", offset+limit, limit, query) }
			        hx-trigger="click"
			        hx-target="this"
			        hx-swap="outerHTML">
				Load More
			</button>
		</div>
	}
}
```

- [ ] **Step 4: Generate templ code and verify compilation**

```bash
go run github.com/a-h/templ/cmd/templ@latest generate && go build ./internal/web/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/web/templates/components/search_bar.templ internal/web/templates/components/search_bar_templ.go internal/web/templates/components/pagination.templ internal/web/templates/components/pagination_templ.go
git commit -m "feat: add search bar and pagination templ components"
```

---

### Task 7: Create browse page template and handler with tests

**Files:**

- Create: `internal/web/templates/pages/browse.templ`
- Create: `internal/web/handlers.go`
- Create: `internal/web/handlers_test.go`

- [ ] **Step 1: Create browse.templ page template**

Create `internal/web/templates/pages/browse.templ`:

```templ
package pages

import (
	"github.com/flowline-io/flowbot-registry/internal/web/templates"
	"github.com/flowline-io/flowbot-registry/internal/web/templates/components"
	"github.com/flowline-io/flowbot-registry/pkg/store"
)

templ BrowsePage(plugins []store.PluginRecord, namespaceNames map[int]string, total int, limit int, offset int, query string) {
	@templates.Layout() {
		@components.SearchBar(query)
		<h1 class="text-2xl font-bold mb-6">Plugins</h1>
		if len(plugins) == 0 {
			<div class="flex flex-col items-center justify-center py-16 text-center">
				<div class="text-5xl mb-4 opacity-30">+</div>
				<h2 class="text-xl font-semibold mb-2">No plugins found</h2>
				if query != "" {
					<p class="text-base-content/60">No plugins matching "{ query }". Try a different search term.</p>
				} else {
					<p class="text-base-content/60">No plugins have been published yet. Use the CLI to publish your first plugin.</p>
				}
			</div>
		} else {
			<div id="plugin-grid">
				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
					for _, p := range plugins {
						<div>
							@components.PluginCard(namespaceNames[p.NamespaceID], p.Name, p.DisplayName, p.Description, p.LogoURL)
						</div>
					}
				</div>
				@components.LoadMoreButton(offset, limit, total, query)
			</div>
		}
	}
}
```

- [ ] **Step 2: Create handlers_test.go with failing tests**

Create `internal/web/handlers_test.go`:

```go
package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/store"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestApp(s store.StoreQuerier) *fiber.App {
	app := fiber.New()
	RegisterWebRoutes(app, s)
	return app
}

func TestBrowsePage(t *testing.T) {
	tests := []struct {
		name       string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name: "happy path with plugins",
			mock: &mockStore{
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin", DisplayName: "Test Plugin", Description: "A test plugin"},
				},
				pluginTotal: 1,
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Test Plugin", "testns", "A test plugin", `href="/testns/test-plugin"`},
		},
		{
			name: "empty store",
			mock: &mockStore{
				plugins:     []store.PluginRecord{},
				pluginTotal: 0,
				namespaces:  map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"No plugins have been published yet"},
		},
		{
			name: "store returns error",
			mock: &mockStore{
				listPluginsErr: errors.New("database connection failed"),
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
		})
	}
}

// mockStore implements store.StoreQuerier for testing.
type mockStore struct {
	plugins      []store.PluginRecord
	pluginTotal  int
	namespaces   map[int]*store.NamespaceRecord
	versions     []store.PluginVersionRecord

	listPluginsErr  error
	getNSByNameErr  error
	getNSByIDErr    error
	getPluginErr    error
	listVersionsErr error
	getVersionErr   error
}

func (m *mockStore) PluginList(_ context.Context, _ string, _, _ int) ([]store.PluginRecord, int, error) {
	if m.listPluginsErr != nil {
		return nil, 0, m.listPluginsErr
	}
	return m.plugins, m.pluginTotal, nil
}

func (m *mockStore) NamespaceGetByID(_ context.Context, id int) (*store.NamespaceRecord, error) {
	if m.getNSByIDErr != nil {
		return nil, m.getNSByIDErr
	}
	if ns, ok := m.namespaces[id]; ok {
		return ns, nil
	}
	return nil, store.ErrNotFound
}

func (m *mockStore) NamespaceGetByName(_ context.Context, name string) (*store.NamespaceRecord, error) {
	if m.getNSByNameErr != nil {
		return nil, m.getNSByNameErr
	}
	for _, ns := range m.namespaces {
		if ns.Name == name {
			return ns, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *mockStore) PluginGetByNamespaceAndName(_ context.Context, namespaceID int, name string) (*store.PluginRecord, error) {
	if m.getPluginErr != nil {
		return nil, m.getPluginErr
	}
	for _, p := range m.plugins {
		if p.NamespaceID == namespaceID && p.Name == name {
			return &p, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *mockStore) PluginListByNamespace(_ context.Context, _ int, _ string, _, _ int) ([]store.PluginRecord, int, error) {
	if m.listPluginsErr != nil {
		return nil, 0, m.listPluginsErr
	}
	return m.plugins, m.pluginTotal, nil
}

func (m *mockStore) PluginVersionListByPlugin(_ context.Context, _ int) ([]store.PluginVersionRecord, error) {
	if m.listVersionsErr != nil {
		return nil, m.listVersionsErr
	}
	return m.versions, nil
}

func (m *mockStore) PluginVersionGetByPluginAndVersion(_ context.Context, _ int, _ string) (*store.PluginVersionRecord, error) {
	if m.getVersionErr != nil {
		return nil, m.getVersionErr
	}
	if len(m.versions) > 0 {
		return &m.versions[0], nil
	}
	return nil, store.ErrNotFound
}
```

- [ ] **Step 3: Run tests, verify they fail**

```bash
go test ./internal/web/ -run TestBrowsePage -v
```

Expected: FAIL — `RegisterWebRoutes` and `BrowsePage` handler not defined.

- [ ] **Step 4: Create handlers.go with BrowsePage handler**

Create `internal/web/handlers.go`:

```go
package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/flowline-io/flowbot-registry/internal/web/templates"
	"github.com/flowline-io/flowbot-registry/internal/web/templates/pages"
	"github.com/flowline-io/flowbot-registry/pkg/store"
	"github.com/gofiber/fiber/v3"
)

// parsePagination extracts limit and offset from query params with clamping.
func parsePagination(c fiber.Ctx) (limit int, offset int) {
	limit, _ = strconv.Atoi(c.Query("limit", "20"))
	offset, _ = strconv.Atoi(c.Query("offset", "0"))
	if limit < 1 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// BrowsePage handles GET / — plugin listing with search and pagination.
func BrowsePage(s store.StoreQuerier) fiber.Handler {
	return func(c fiber.Ctx) error {
		query := c.Query("q", "")
		limit, offset := parsePagination(c)

		plugins, total, err := s.PluginList(c.Context(), query, limit, offset)
		if err != nil {
			slog.Error("browse: list plugins failed", "error", err)
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		}

		namespaceNames := make(map[int]string)
		for _, p := range plugins {
			if _, ok := namespaceNames[p.NamespaceID]; !ok {
				ns, nsErr := s.NamespaceGetByID(c.Context(), p.NamespaceID)
				if nsErr != nil {
					namespaceNames[p.NamespaceID] = "unknown"
				} else {
					namespaceNames[p.NamespaceID] = ns.Name
				}
			}
		}

		return Render(c, pages.BrowsePage(plugins, namespaceNames, total, limit, offset, query))
	}
}
```

- [ ] **Step 5: Run tests, verify BrowsePage passes**

```bash
go test ./internal/web/ -run TestBrowsePage -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/web/handlers.go internal/web/handlers_test.go internal/web/templates/pages/browse.templ
git commit -m "feat: add browse page with handler and tests"
```

---

### Task 8: Create htmx fragment handlers with tests

**Files:**

- Modify: `internal/web/handlers.go` (add RegisterWebRoutes, errorPage)
- Create: `internal/web/htmx.go`
- Create: `internal/web/htmx_test.go`

- [ ] **Step 1: Add errorPage and RegisterWebRoutes to handlers.go**

Add imports to `internal/web/handlers.go`:

```go
import (
	// ... existing imports ...
	"github.com/flowline-io/flowbot-registry/internal/web/templates"
	"github.com/flowline-io/flowbot-registry/internal/web/templates/pages"
)
```

Append to `internal/web/handlers.go`:

```go
// errorPage renders a full-page error response.
func errorPage(c fiber.Ctx, status int, message string) error {
	c.Status(status)
	return Render(c, templates.LayoutError(message))
}

// RegisterWebRoutes registers all web UI routes on the Fiber app.
func RegisterWebRoutes(app *fiber.App, s store.StoreQuerier) {
	app.Get("/web/search", SearchFragment(s))
	app.Get("/web/plugins", PluginGridFragment(s))
	app.Get("/web/versions/:namespace/:name/:version", VersionReadmeFragment(s))

	app.Get("/", BrowsePage(s))
}
```

Note: `templates.LayoutError` does not exist yet. We need to add it to `layout.templ`. Let's add it there.

- [ ] **Step 2: Add LayoutError to layout.templ**

Append to `internal/web/templates/layout.templ`:

```templ
templ LayoutError(message string) {
	@Layout() {
		<div class="flex flex-col items-center justify-center py-16">
			<div class="alert alert-error max-w-md">
				<svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 shrink-0 stroke-current" fill="none" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"/>
				</svg>
				<span>{ message }</span>
			</div>
		</div>
	}
}

templ LayoutNotFound(resource string) {
	@Layout() {
		<div class="flex flex-col items-center justify-center py-16 text-center">
			<div class="text-6xl font-bold text-base-content/20 mb-4">404</div>
			<h1 class="text-2xl font-bold mb-2">Not Found</h1>
			<p class="text-base-content/60">{ resource }</p>
			<a href="/" class="btn btn-primary mt-6">Back to Home</a>
		</div>
	}
}
```

- [ ] **Step 3: Create htmx_test.go with failing tests**

Create `internal/web/htmx_test.go`:

```go
package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchFragment(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name:  "search returns matching plugins",
			query: "test",
			mock: &mockStore{
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin", DisplayName: "Test Plugin"},
				},
				pluginTotal: 1,
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Test Plugin", "testns"},
		},
		{
			name:  "search returns no results",
			query: "nonexistent",
			mock: &mockStore{
				plugins:     []store.PluginRecord{},
				pluginTotal: 0,
				namespaces:  map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"No plugins matching"},
		},
		{
			name: "store returns error",
			mock: &mockStore{
				listPluginsErr: store.ErrNotFound,
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/web/search?q="+tt.query, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
		})
	}
}

func TestPluginGridFragment(t *testing.T) {
	tests := []struct {
		name       string
		offset     string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name:   "load more returns cards and new button",
			offset: "0",
			mock: &mockStore{
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "p1", DisplayName: "Plugin 1"},
				},
				pluginTotal: 25,
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "ns1", Type: "user"},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Plugin 1", "Load More"},
		},
		{
			name:   "last page returns no button",
			offset: "20",
			mock: &mockStore{
				plugins: []store.PluginRecord{
					{ID: 2, NamespaceID: 1, Name: "p2", DisplayName: "Plugin 2"},
				},
				pluginTotal: 25,
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "ns1", Type: "user"},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Plugin 2"},
		},
		{
			name: "store error returns error fragment",
			mock: &mockStore{
				listPluginsErr: store.ErrNotFound,
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/web/plugins?offset="+tt.offset+"&limit=20", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
			// Load More button should NOT appear on last page
			if tt.name == "last page returns no button" {
				assert.NotContains(t, string(body), "Load More")
			}
		})
	}
}

func TestVersionReadmeFragment(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		pluginName   string
		version      string
		mock         *mockStore
		wantStatus   int
		wantBody     []string
		wantNotBody  []string
	}{
		{
			name:       "happy path renders readme",
			namespace:  "testns",
			pluginName: "test-plugin",
			version:    "1.0.0",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin"},
				},
				versions: []store.PluginVersionRecord{
					{ID: 1, PluginID: 1, Version: "1.0.0", ReadmeHTML: "<h1>Hello</h1>", ManifestJSON: map[string]any{"name": "test-plugin"}},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"<h1>Hello</h1>", "1.0.0"},
		},
		{
			name:       "version not found returns inline error",
			namespace:  "testns",
			pluginName: "test-plugin",
			version:    "9.9.9",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin"},
				},
				versions: []store.PluginVersionRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Version not found"},
		},
		{
			name:       "namespace not found returns inline error",
			namespace:  "nonexistent",
			pluginName: "test-plugin",
			version:    "1.0.0",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Version not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			url := "/web/versions/" + tt.namespace + "/" + tt.pluginName + "/" + tt.version
			req := httptest.NewRequest(http.MethodGet, url, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
			for _, s := range tt.wantNotBody {
				assert.NotContains(t, string(body), s)
			}
		})
	}
}
```

- [ ] **Step 4: Run tests, verify they fail**

```bash
go test ./internal/web/ -run "TestSearchFragment|TestPluginGridFragment|TestVersionReadmeFragment" -v
```

Expected: FAIL — handlers not defined.

- [ ] **Step 5: Create htmx.go with fragment handlers**

Create `internal/web/htmx.go`:

```go
package web

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/flowline-io/flowbot-registry/internal/web/templates/components"
	"github.com/flowline-io/flowbot-registry/pkg/store"
	"github.com/gofiber/fiber/v3"
)

// SearchFragment handles GET /web/search — returns plugin grid HTML fragment for htmx search.
func SearchFragment(s store.StoreQuerier) fiber.Handler {
	return func(c fiber.Ctx) error {
		query := c.Query("q", "")
		limit, offset := parsePagination(c)

		plugins, total, err := s.PluginList(c.Context(), query, limit, offset)
		if err != nil {
			slog.Error("search fragment: list plugins failed", "error", err)
			return Render(c, htmxError("Something went wrong. Please try again later."))
		}

		namespaceNames := resolveNamespaces(c, s, plugins)
		return Render(c, browseFragment(plugins, namespaceNames, total, limit, offset, query))
	}
}

// PluginGridFragment handles GET /web/plugins — returns plugin cards + load-more button for pagination.
func PluginGridFragment(s store.StoreQuerier) fiber.Handler {
	return func(c fiber.Ctx) error {
		query := c.Query("q", "")
		limit, offset := parsePagination(c)

		plugins, total, err := s.PluginList(c.Context(), query, limit, offset)
		if err != nil {
			slog.Error("plugin grid fragment: list plugins failed", "error", err)
			return Render(c, htmxError("Something went wrong. Please try again later."))
		}

		namespaceNames := resolveNamespaces(c, s, plugins)

		var parts []templ.Component
		for _, p := range plugins {
			parts = append(parts, components.PluginCard(namespaceNames[p.NamespaceID], p.Name, p.DisplayName, p.Description, p.LogoURL))
		}
		if offset+limit < total {
			parts = append(parts, components.LoadMoreButton(offset, limit, total, query))
		}

		return Render(c, htmxComponentList(parts))
	}
}

// VersionReadmeFragment handles GET /web/versions/:namespace/:name/:version — returns README fragment for version switching.
func VersionReadmeFragment(s store.StoreQuerier) fiber.Handler {
	return func(c fiber.Ctx) error {
		namespace := c.Params("namespace")
		name := c.Params("name")
		version := c.Params("version")

		ns, err := s.NamespaceGetByName(c.Context(), namespace)
		if err != nil {
			slog.Warn("version fragment: namespace not found", "namespace", namespace, "error", err)
			return Render(c, htmxError("Version not found"))
		}

		p, err := s.PluginGetByNamespaceAndName(c.Context(), ns.ID, name)
		if err != nil {
			slog.Warn("version fragment: plugin not found", "namespace", namespace, "name", name, "error", err)
			return Render(c, htmxError("Version not found"))
		}

		pv, err := s.PluginVersionGetByPluginAndVersion(c.Context(), p.ID, version)
		if err != nil {
			slog.Warn("version fragment: version not found", "version", version, "error", err)
			return Render(c, htmxError("Version not found"))
		}

		return Render(c, components.ReadmeRender(pv.Version, pv.ReadmeHTML, pv.ManifestJSON))
	}
}

// resolveNamespaces looks up namespace names for a set of plugins.
func resolveNamespaces(c fiber.Ctx, s store.StoreQuerier, plugins []store.PluginRecord) map[int]string {
	names := make(map[int]string)
	for _, p := range plugins {
		if _, ok := names[p.NamespaceID]; !ok {
			ns, err := s.NamespaceGetByID(c.Context(), p.NamespaceID)
			if err != nil {
				names[p.NamespaceID] = "unknown"
			} else {
				names[p.NamespaceID] = ns.Name
			}
		}
	}
	return names
}

// htmxError returns a templ component for inline error display.
func htmxError(message string) templ.Component {
	return components.AlertError(message)
}

// htmxComponentList renders a list of components sequentially.
func htmxComponentList(components []templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		for _, c := range components {
			if err := c.Render(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
}

// browseFragment renders the internal plugin grid fragment used by SearchFragment.
func browseFragment(plugins []store.PluginRecord, namespaceNames map[int]string, total, limit, offset int, query string) templ.Component {
	var parts []templ.Component
	for _, p := range plugins {
		parts = append(parts, components.PluginCard(namespaceNames[p.NamespaceID], p.Name, p.DisplayName, p.Description, p.LogoURL))
	}
	if offset+limit < total {
		parts = append(parts, components.LoadMoreButton(offset, limit, total, query))
	}
	return htmxComponentList(parts)
}
```

Required imports in `htmx.go`: add `"io"`, `"context"` for `templ.ComponentFunc`.

- [ ] **Step 6: Add AlertError to components**

Create `internal/web/templates/components/alert.templ`:

```templ
package components

templ AlertError(message string) {
	<div class="alert alert-error">
		<svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 shrink-0 stroke-current" fill="none" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"/>
		</svg>
		<span>{ message }</span>
	</div>
}
```

- [ ] **Step 7: Generate templ and run tests**

```bash
go run github.com/a-h/templ/cmd/templ@latest generate && go test ./internal/web/ -run "TestSearchFragment|TestPluginGridFragment|TestVersionReadmeFragment" -v
```

- [ ] **Step 8: Commit**

```bash
git add internal/web/htmx.go internal/web/htmx_test.go internal/web/handlers.go internal/web/templates/layout.templ internal/web/templates/components/alert.templ
git commit -m "feat: add htmx fragment handlers with tests"
```

---

### Task 9: Create detail page with tests

**Files:**

- Create: `internal/web/templates/components/version_list.templ`
- Create: `internal/web/templates/components/readme_render.templ`
- Create: `internal/web/templates/pages/detail.templ`
- Modify: `internal/web/handlers.go` (add DetailPage handler)
- Modify: `internal/web/handlers_test.go` (add DetailPage tests)

- [ ] **Step 1: Create version_list.templ**

Create `internal/web/templates/components/version_list.templ`:

```templ
package components

import "github.com/flowline-io/flowbot-registry/pkg/store"

templ VersionList(versions []store.PluginVersionRecord, namespace string, name string, activeVersion string) {
	<div class="flex flex-col gap-1">
		<h3 class="text-sm font-semibold mb-2 uppercase tracking-wide opacity-70">Versions</h3>
		for _, v := range versions {
			if v.Version == activeVersion {
				<a class="btn btn-sm btn-primary"
				   href={ "/" + namespace + "/" + name + "?version=" + v.Version }
				   hx-get={ "/web/versions/" + namespace + "/" + name + "/" + v.Version }
				   hx-target="#readme-content"
				   hx-swap="innerHTML">{ v.Version }</a>
			} else {
				<a class="btn btn-sm btn-ghost"
				   href={ "/" + namespace + "/" + name + "?version=" + v.Version }
				   hx-get={ "/web/versions/" + namespace + "/" + name + "/" + v.Version }
				   hx-target="#readme-content"
				   hx-swap="innerHTML">{ v.Version }</a>
			}
		}
	</div>
}
```

- [ ] **Step 2: Create readme_render.templ**

Create `internal/web/templates/components/readme_render.templ`:

```templ
package components

import "unsafe"

templ ReadmeRender(version string, readmeHTML string, manifest map[string]any) {
	<div id="readme-content">
		<div class="flex items-center gap-3 mb-4">
			<div class="badge badge-lg">{ version }</div>
			if author, ok := manifest["author"]; ok {
				if authorStr, ok2 := author.(string); ok2 && authorStr != "" {
					<div class="text-sm text-base-content/60">by { authorStr }</div>
				}
			}
		</div>
		<div class="divider"></div>
		if readmeHTML != "" {
			@unsafe.Raw(readmeHTML)
		} else {
			<div class="text-base-content/60 italic">No README provided for this version.</div>
		}
	</div>
}

func unsafe.Raw(html string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w, html)
		return err
	})
}
```

Note: The `unsafe.Raw` helper is needed to render raw HTML. Since templ escapes by default, we need a raw HTML helper. Let's rename to avoid conflict with stdlib `unsafe`. Use `rawHTML` instead.

- [ ] **Step 3: Fix readme_render.templ — use proper raw HTML helper**

```templ
package components

templ ReadmeRender(version string, readmeHTML string, manifest map[string]any) {
	<div id="readme-content">
		<div class="flex items-center gap-3 mb-4">
			<div class="badge badge-lg">{ version }</div>
			if author, ok := manifest["author"]; ok {
				if authorStr, ok2 := author.(string); ok2 && authorStr != "" {
					<div class="text-sm text-base-content/60">by { authorStr }</div>
				}
			}
		</div>
		<div class="divider"></div>
		if readmeHTML != "" {
			{ templ.Raw(readmeHTML) }
		} else {
			<div class="text-base-content/60 italic">No README provided for this version.</div>
		}
	}
}
```

- [ ] **Step 4: Create detail.templ page**

Create `internal/web/templates/pages/detail.templ`:

```templ
package pages

import (
	"github.com/flowline-io/flowbot-registry/internal/web/templates"
	"github.com/flowline-io/flowbot-registry/internal/web/templates/components"
	"github.com/flowline-io/flowbot-registry/pkg/store"
)

templ DetailPage(p store.PluginRecord, nsName string, versions []store.PluginVersionRecord, activeVersion *store.PluginVersionRecord) {
	@templates.Layout() {
		<div class="text-sm breadcrumbs mb-4">
			<ul>
				<li><a href="/">Home</a></li>
				<li><a href={ "/" + nsName }>{ nsName }</a></li>
				<li>{ p.Name }</li>
			</ul>
		</div>
		<div class="flex flex-col lg:flex-row gap-8">
			<aside class="lg:w-48 shrink-0">
				@components.VersionList(versions, nsName, p.Name, activeVersion.Version)
			</aside>
			<main class="flex-1 min-w-0">
				<div class="mb-6">
					if p.DisplayName != "" {
						<h1 class="text-3xl font-bold">{ p.DisplayName }</h1>
					} else {
						<h1 class="text-3xl font-bold">{ p.Name }</h1>
					}
					<div class="badge badge-outline mt-2">{ nsName }</div>
				</div>
				if p.Description != "" {
					<p class="text-lg text-base-content/70 mb-6">{ p.Description }</p>
					<div class="divider"></div>
				}
				@components.ReadmeRender(activeVersion.Version, activeVersion.ReadmeHTML, activeVersion.ManifestJSON)
			</main>
		</div>
	}
}
```

- [ ] **Step 5: Add DetailPage tests to handlers_test.go**

Append to `internal/web/handlers_test.go`:

```go
func TestDetailPage(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		pluginName string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name:       "happy path renders plugin detail",
			namespace:  "testns",
			pluginName: "test-plugin",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin", DisplayName: "Test Plugin", Description: "A test plugin"},
				},
				versions: []store.PluginVersionRecord{
					{ID: 1, PluginID: 1, Version: "2.0.0", ReadmeHTML: "<p>v2 readme</p>", ManifestJSON: map[string]any{"name": "test-plugin"}},
					{ID: 2, PluginID: 1, Version: "1.0.0", ReadmeHTML: "<p>v1 readme</p>", ManifestJSON: map[string]any{"name": "test-plugin"}},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Test Plugin", "testns", "A test plugin", "2.0.0", "v2 readme", "1.0.0"},
		},
		{
			name:      "namespace not found",
			namespace: "nonexistent",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusNotFound,
			wantBody:   []string{"Not Found", "nonexistent", "Back to Home"},
		},
		{
			name:       "plugin not found",
			namespace:  "testns",
			pluginName: "nonexistent",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{},
			},
			wantStatus: http.StatusNotFound,
			wantBody:   []string{"Not Found", "nonexistent", "Back to Home"},
		},
		{
			name:       "store error",
			namespace:  "testns",
			pluginName: "test-plugin",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin"},
				},
				listVersionsErr: errors.New("database connection failed"),
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/"+tt.namespace+"/"+tt.pluginName, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
		})
	}
}
```

- [ ] **Step 6: Run tests, verify they fail**

```bash
go test ./internal/web/ -run TestDetailPage -v
```

- [ ] **Step 7: Add DetailPage handler to handlers.go and page-not-found helper**

Append to `internal/web/handlers.go`:

```go
// DetailPage handles GET /:namespace/:name — plugin detail with versions and README.
func DetailPage(s store.StoreQuerier) fiber.Handler {
	return func(c fiber.Ctx) error {
		namespace := c.Params("namespace")
		name := c.Params("name")

		ns, err := s.NamespaceGetByName(c.Context(), namespace)
		if err != nil {
			slog.Warn("detail: namespace not found", "namespace", namespace, "error", err)
			if errors.Is(err, store.ErrNotFound) {
				c.Status(http.StatusNotFound)
				return Render(c, templates.LayoutNotFound("namespace `" + namespace + "` not found"))
			}
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		}

		p, err := s.PluginGetByNamespaceAndName(c.Context(), ns.ID, name)
		if err != nil {
			slog.Warn("detail: plugin not found", "namespace", namespace, "name", name, "error", err)
			if errors.Is(err, store.ErrNotFound) {
				c.Status(http.StatusNotFound)
				return Render(c, templates.LayoutNotFound("plugin `" + namespace + "/" + name + "` not found"))
			}
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		}

		versions, err := s.PluginVersionListByPlugin(c.Context(), p.ID)
		if err != nil {
			slog.Error("detail: list versions failed", "error", err)
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		}

		if len(versions) == 0 {
			slog.Warn("detail: no versions found", "plugin_id", p.ID)
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		}

		activeVersion := &versions[0]

		return Render(c, pages.DetailPage(*p, ns.Name, versions, activeVersion))
	}
}
```

Update `RegisterWebRoutes` in `handlers.go`:

```go
func RegisterWebRoutes(app *fiber.App, s store.StoreQuerier) {
	app.Get("/web/search", SearchFragment(s))
	app.Get("/web/plugins", PluginGridFragment(s))
	app.Get("/web/versions/:namespace/:name/:version", VersionReadmeFragment(s))

	app.Get("/", BrowsePage(s))
	app.Get("/:namespace", NamespacePage(s))
	app.Get("/:namespace/:name", DetailPage(s))
}
```

- [ ] **Step 8: Run tests, verify DetailPage passes**

```bash
go test ./internal/web/ -run TestDetailPage -v
```

- [ ] **Step 9: Commit**

```bash
git add internal/web/handlers.go internal/web/handlers_test.go internal/web/templates/components/version_list.templ internal/web/templates/components/readme_render.templ internal/web/templates/pages/detail.templ
git commit -m "feat: add detail page with version list and readme rendering"
```

---

### Task 10: Create namespace page with tests

**Files:**

- Create: `internal/web/templates/pages/namespace.templ`
- Modify: `internal/web/handlers.go` (add NamespacePage handler)
- Modify: `internal/web/handlers_test.go` (add NamespacePage tests)

- [ ] **Step 1: Create namespace.templ**

Create `internal/web/templates/pages/namespace.templ`:

```templ
package pages

import (
	"github.com/flowline-io/flowbot-registry/internal/web/templates"
	"github.com/flowline-io/flowbot-registry/internal/web/templates/components"
	"github.com/flowline-io/flowbot-registry/pkg/store"
)

templ NamespacePage(nsName string, nsType string, plugins []store.PluginRecord, total int, limit int, offset int) {
	@templates.Layout() {
		<div class="text-sm breadcrumbs mb-4">
			<ul>
				<li><a href="/">Home</a></li>
				<li>{ nsName }</li>
			</ul>
		</div>
		<div class="mb-6">
			<h1 class="text-3xl font-bold">{ nsName }</h1>
			<div class="badge badge-outline mt-2">{ nsType }</div>
		</div>
		if len(plugins) == 0 {
			<div class="flex flex-col items-center justify-center py-16 text-center">
				<div class="text-5xl mb-4 opacity-30">+</div>
				<h2 class="text-xl font-semibold mb-2">No plugins</h2>
				<p class="text-base-content/60">This namespace has no published plugins yet.</p>
			</div>
		} else {
			<div id="plugin-grid">
				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
					for _, p := range plugins {
						<div>
							@components.PluginCard(nsName, p.Name, p.DisplayName, p.Description, p.LogoURL)
						</div>
					}
				</div>
				@components.LoadMoreButton(offset, limit, total, "")
			</div>
		}
	}
}
```

- [ ] **Step 2: Add NamespacePage tests to handlers_test.go**

Append to `internal/web/handlers_test.go`:

```go
func TestNamespacePage(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name:      "namespace not found",
			namespace: "nonexistent",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Not Found", "nonexistent", "Back to Home"},
		},

				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin", DisplayName: "Test Plugin", Description: "A test plugin"},
				},
				pluginTotal: 1,
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"testns", "user", "Test Plugin"},
		},
		{
			name:      "empty namespace",
			namespace: "emptyns",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					2: {ID: 2, Name: "emptyns", Type: "org"},
				},
				plugins:     []store.PluginRecord{},
				pluginTotal: 0,
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"emptyns", "org", "No plugins"},
		},
		{
			name:      "namespace not found",
			namespace: "nonexistent",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusNotFound,
			wantBody:   []string{"Not Found", "nonexistent", "Back to Home"},
		},
		{
			name:      "store error",
			namespace: "testns",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins:         []store.PluginRecord{},
				listPluginsErr:  errors.New("database connection failed"),
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/"+tt.namespace, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
		})
	}
}
```

- [ ] **Step 3: Run tests, verify they fail**

```bash
go test ./internal/web/ -run TestNamespacePage -v
```

- [ ] **Step 4: Add NamespacePage handler to handlers.go**

Append to `internal/web/handlers.go`:

```go
// NamespacePage handles GET /:namespace — all plugins in a namespace.
func NamespacePage(s store.StoreQuerier) fiber.Handler {
	return func(c fiber.Ctx) error {
		namespace := c.Params("namespace")
		limit, offset := parsePagination(c)

		ns, err := s.NamespaceGetByName(c.Context(), namespace)
		if err != nil {
			slog.Warn("namespace page: namespace not found", "namespace", namespace, "error", err)
			if errors.Is(err, store.ErrNotFound) {
				c.Status(http.StatusNotFound)
				return Render(c, templates.LayoutNotFound("namespace `" + namespace + "` not found"))
			}
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		}

		plugins, total, err := s.PluginListByNamespace(c.Context(), ns.ID, "", limit, offset)
		if err != nil {
			slog.Error("namespace page: list plugins failed", "error", err)
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		}

		return Render(c, pages.NamespacePage(ns.Name, ns.Type, plugins, total, limit, offset))
	}
}
```

- [ ] **Step 5: Run tests, verify NamespacePage passes**

```bash
go test ./internal/web/ -run TestNamespacePage -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/web/handlers.go internal/web/handlers_test.go internal/web/templates/pages/namespace.templ
git commit -m "feat: add namespace page with handler and tests"
```

---

### Task 11: Wire web routes in cmd/server/main.go

**Files:**

- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add web route registration in main.go**

In `cmd/server/main.go`, after the line `handler.RegisterRoutes(app, authSvc, pluginSvc)`, add:

```go
	web.RegisterWebRoutes(app, adapter)
```

Add the import:

```go
	"github.com/flowline-io/flowbot-registry/internal/web"
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/server
```

- [ ] **Step 3: Run all tests**

```bash
go test ./internal/web/... -v
go test ./pkg/store/... -v
```

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire web UI routes in server main"
```

---

### Task 12: Add build:templ task to taskfile

**Files:**

- Modify: `taskfile.yaml`

- [ ] **Step 1: Add templ build task**

Add to `taskfile.yaml`, before the `tidy` task:

```yaml
build:templ:
  desc: Generate Go code from templ templates
  cmds:
    - go tool templ generate
```

- [ ] **Step 2: Install templ as a Go tool**

```bash
go get -tool github.com/a-h/templ/cmd/templ
```

- [ ] **Step 3: Verify templ generation works**

```bash
go tool task build:templ
```

- [ ] **Step 4: Commit**

```bash
git add taskfile.yaml go.mod go.sum
git commit -m "build: add build:templ task to taskfile"
```

---

### Task 13: Final integration test

**Files:**

- None (verification only)

- [ ] **Step 1: Run full lint**

```bash
go tool task lint
```

- [ ] **Step 2: Run all tests**

```bash
go tool task test
```

- [ ] **Step 3: Verify build**

```bash
go tool task build
```

- [ ] **Step 4: Run format**

```bash
go tool task format
```

- [ ] **Step 5: Commit any remaining changes**

```bash
git add -A
git commit -m "chore: final lint, test, and format pass"
```
