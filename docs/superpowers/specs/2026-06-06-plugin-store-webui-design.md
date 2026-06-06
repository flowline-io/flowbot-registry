# Plugin Store Web UI Design

Date: 2026-06-06

## Summary

Add a server-rendered web UI plugin store to the Flowbot Registry using templ + htmx + daisyUI. Served by the existing Fiber v3 server alongside the JSON API. Three pages: Browse/Home, Plugin Detail, Namespace. Three htmx-powered interactions: live search, load-more pagination, version switching.

## Architecture Decision

**Approach A: Web layer over Store** — a new `internal/web/` package queries `store.Adapter` directly, independent from the JSON API layer. Web routes and htmx fragment endpoints are additive, non-conflicting with existing `/api/v1/*` routes.

## Project Structure

New additions to the repo:

```
internal/web/
├── templates/
│   ├── layout.templ
│   ├── pages/
│   │   ├── browse.templ
│   │   ├── detail.templ
│   │   └── namespace.templ
│   └── components/
│       ├── plugin_card.templ
│       ├── search_bar.templ
│       ├── pagination.templ
│       ├── version_list.templ
│       └── readme_render.templ
├── handlers.go
├── htmx.go
└── render.go

cmd/server/main.go               # Modified: add web + htmx route registration
```

Build pipeline: `go tool task build` picks up templ-generated Go files automatically. A new taskfile target (`build:templ`) runs `templ generate` before build.

## Dependencies

New Go modules to add:

- `github.com/a-h/templ` — Go HTML templating engine, compiles `.templ` to `*_templ.go`
- Custom Fiber adapter in `internal/web/render.go` — calls `templ.Component.Render(ctx, w)` directly from handlers

Client-side (loaded via CDN in `layout.templ`):

- Tailwind CSS v4 (CDN: `@tailwindcss/browser`)
- daisyUI v5 (CDN: `daisyui.com/cdn`)
- htmx v2 (CDN: `unpkg.com/htmx.org`)

Choose a daisyUI theme; default `light` theme for now, configurable later.

## Routes

### Route Ordering

Parameterized page routes (`/:namespace`, `/:namespace/:name`) must be registered after all static/prefixed routes (`/web/*`, `/api/*`, `/static/*`, `/favicon.ico`) to avoid unintentionally matching those paths as namespaces.

### Page Routes

| Method | Path                | Handler         | Description                                            |
| ------ | ------------------- | --------------- | ------------------------------------------------------ |
| GET    | `/`                 | `BrowsePage`    | Plugin listing with search bar, cards grid, pagination |
| GET    | `/:namespace`       | `NamespacePage` | All plugins from a namespace                           |
| GET    | `/:namespace/:name` | `DetailPage`    | Plugin detail: meta, version list, README              |

### htmx Fragment Routes

| Method | Path                               | Handler                 | Description                                               |
| ------ | ---------------------------------- | ----------------------- | --------------------------------------------------------- |
| GET    | `/web/search`                      | `SearchFragment`        | Search results grid + pagination (querystring `?q=`)      |
| GET    | `/web/plugins`                     | `PluginGridFragment`    | Plugin cards for load-more pagination (`?offset=&limit=`) |
| GET    | `/web/versions/:ns/:name/:version` | `VersionReadmeFragment` | README HTML for a specific version                        |

### Existing API Routes (unchanged)

| Method | Path                                    | Description                |
| ------ | --------------------------------------- | -------------------------- |
| GET    | `/api/v1/auth/token`                    | Docker Registry token      |
| POST   | `/api/v1/plugins/:ns/:name/publish`     | Publish plugin             |
| GET    | `/api/v1/plugins`                       | Search/list plugins (JSON) |
| GET    | `/api/v1/plugins/:ns/:name/versions/:v` | Get version (JSON)         |

## Pages and Data Flow

### Browse Page (`/`)

```
GET /  →  BrowsePage handler:
  - Query params: q (search), limit (default 20), offset (default 0)
  - Calls store.PluginList(ctx, q, limit, offset)
  - Joins namespace name for each plugin via store.NamespaceGetByID
  - Renders browse.templ with plugin cards + search bar + pagination
```

Plugin cards display: display_name (fallback to name), description, namespace name, logo_url.
Click navigates to `/:namespace/:name`.

### Namespace Page (`/:namespace`)

```
GET /:namespace  →  NamespacePage handler:
  - Calls store.NamespaceGetByName(namespace)
  - Calls store.PluginListByNamespace(ctx, namespaceID, limit, offset)
  - Renders namespace.templ with namespace name + plugin grid
```

If namespace not found: render 404 page.

### Plugin Detail Page (`/:namespace/:name`)

```
GET /:namespace/:name  →  DetailPage handler:
  - Calls store.NamespaceGetByName(namespace) → namespaceID
  - Calls store.PluginGetByNamespaceAndName(namespaceID, name)
  - Calls store.PluginVersionListByPlugin(ctx, pluginID) (new method, ordered by creation DESC)
  - Takes first result as latest version
  - Renders detail.templ with sidebar (versions) + main (meta + latest version README)
```

Initial load shows latest version (first in list). README rendered as HTML from `readme_html` field.

New store methods required:

- `PluginVersionListByPlugin(ctx, pluginID) ([]PluginVersionRecord, error)` — ordered by `id DESC` (creation order)
- `PluginListByNamespace(ctx, namespaceID, query, limit, offset) ([]PluginRecord, int, error)` — filter by namespace
- `NamespaceGetByID(ctx, id)` — needed for browse page card rendering

## htmx Interactions

### Live Search

```
Search bar (browse page):
  <input type="search" name="q"
         hx-get="/web/search"
         hx-trigger="keyup changed delay:300ms"
         hx-target="#plugin-grid"
         hx-swap="innerHTML">
```

Server returns `search_bar.templ` + `plugin_card.templ` × N + `pagination.templ` as HTML fragment.
300ms debounce prevents thrashing. In-flight requests cancelled via htmx default behavior.

### Load More Pagination

```
Load more button (bottom of browse/namespace grids):
  <button hx-get="/web/plugins?offset=N&limit=20&q=..."
          hx-trigger="click"
          hx-target="this"
          hx-swap="outerHTML">Load More</button>

Server returns: plugin_card.templ × N, then either:
  - Another load-more button (if more results exist)
  - Empty response if no more results (button disappears)
```

### Version Switching

```
Version links (detail page sidebar):
  <a hx-get="/web/versions/:ns/:name/:version"
     hx-target="#readme-content"
     hx-swap="innerHTML"
     hx-trigger="click">v1.2.3</a>

Server returns: readme_render.templ with version meta header + README HTML.
Active version highlighted via CSS class swap (htmx class-tools or server-rendered active indicator).
```

## Error Handling

| Scenario                          | Response                                                                  |
| --------------------------------- | ------------------------------------------------------------------------- |
| Namespace not found (page)        | 404 page: "Namespace `foo` not found"                                     |
| Plugin not found (page)           | 404 page: "Plugin `foo/bar` not found"                                    |
| Version not found (htmx fragment) | `<div class="alert">Version not found</div>` (200 OK, inline error)       |
| Search yields no results          | Empty state: "No plugins found matching `query`" with illustration        |
| Database error                    | Generic error page: "Something went wrong. Please try again later." (500) |
| Invalid query params              | Clamp `limit` 1-50, clamp `offset` >= 0, ignore non-numeric values        |
| Empty store (no plugins at all)   | Empty state: "No plugins published yet" with brief description            |

## Layout and Styling

**Layout**: Top navbar + centered content (max-width container, daisyUI `container` class).

Navbar contains:

- Logo/brand text (links to `/`)
- Search bar (form submits to browse page as progressive enhancement fallback)

Breadcrumbs on detail page: Home / namespace / plugin

daisyUI components used:

- `navbar` — top navigation
- `card` — plugin cards with image (logo), title, description, badge (namespace)
- `input` — search input styled as daisyUI form-control
- `btn` — load more button
- `badge` — version tags, namespace labels
- `breadcrumbs` — detail page navigation
- `alert` — error states
- `loading` — loading spinner (htmx `hx-indicator`)
- `divider` — section separators
- `join` — version selector horizontal button group

## Progressive Enhancement

All htmx interactions have fallback behavior for non-JS clients:

- Search form submits to `/` with `?q=` (full page reload)
- Load more button is a link to `/?offset=N` (full page reload)
- Version links are links to the detail page with `?version=v1.2.3` (full page reload with that version pre-selected)

## Testing Strategy

Table-driven tests co-located with source files, minimum 3 cases per table.

### Page Handler Tests (`handlers_test.go`)

Test cases per handler:

- Happy path: renders expected HTML, status 200
- Not found: namespace/plugin missing, status 404, contains error message
- DB error: store returns error, status 500, contains generic error

### htmx Fragment Tests (`htmx_test.go`)

Test cases per handler:

- Happy path: search returns matching cards, version fragment renders README
- No results: search returns empty state HTML
- Pagination: load more returns next slice, last page returns no button
- Error: version not found returns inline alert

### Mock Approach

Inject `store.Adapter` interface into web handlers. Tests use a mock adapter (generated or hand-written) that returns predefined records or errors. The store package already has `Adapter` as a concrete struct; extract an interface for testability.

## New Store Methods Required

Add to `pkg/store/store.go`:

```go
// PluginVersionListByPlugin returns all versions for a plugin, newest first.
func (a *Adapter) PluginVersionListByPlugin(ctx context.Context, pluginID int) ([]PluginVersionRecord, error)

// PluginListByNamespace returns plugins in a namespace with optional search and pagination.
func (a *Adapter) PluginListByNamespace(ctx context.Context, namespaceID int, query string, limit, offset int) ([]PluginRecord, int, error)

// NamespaceGetByID retrieves a namespace by ID.
func (a *Adapter) NamespaceGetByID(ctx context.Context, id int) (*NamespaceRecord, error)
```

## Implementation Order

1. Add templ dependency and Fiber integration (`render.go`)
2. Create `layout.templ` base template (HTML shell, navbar, CDN links)
3. Create `plugin_card.templ` component
4. Create `browse.templ` page + `BrowsePage` handler + `PluginGridFragment` + `SearchFragment` handlers
5. Create `detail.templ` page + `DetailPage` handler + `VersionReadmeFragment` handler
6. Create `namespace.templ` page + `NamespacePage` handler
7. Add pagination, search bar, version list components
8. Add new store methods (`PluginVersionListByPlugin`, `PluginListByNamespace`, `NamespaceGetByID`)
9. Wire routes in `cmd/server/main.go`
10. Add `build:templ` task to taskfile
11. Write tests for all handlers and htmx fragments
