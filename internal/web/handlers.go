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
	app.Get("/:namespace/:name", DetailPage(s))
}
