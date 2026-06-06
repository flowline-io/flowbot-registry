package web

import (
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

// errorPage renders a full-page error response.
func errorPage(c fiber.Ctx, status int, message string) error {
	c.Status(status)
	return Render(c, templates.LayoutError(message))
}

// RegisterWebRoutes registers all web UI routes on the Fiber app.
func RegisterWebRoutes(app *fiber.App, s store.StoreQuerier) {
	app.Get("/", BrowsePage(s))
}
