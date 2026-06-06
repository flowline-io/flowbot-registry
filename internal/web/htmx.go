package web

import (
	"context"
	"io"
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
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
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
			return errorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
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

// VersionReadmeFragment handles GET /web/versions/:namespace/:name/:version.
func VersionReadmeFragment(s store.StoreQuerier) fiber.Handler {
	return func(c fiber.Ctx) error {
		namespace := c.Params("namespace")
		name := c.Params("name")
		version := c.Params("version")

		ns, err := s.NamespaceGetByName(c.Context(), namespace)
		if err != nil {
			slog.Warn("version fragment: namespace not found", "namespace", namespace, "error", err)
			return Render(c, components.AlertError("Version not found"))
		}

		p, err := s.PluginGetByNamespaceAndName(c.Context(), ns.ID, name)
		if err != nil {
			slog.Warn("version fragment: plugin not found", "namespace", namespace, "name", name, "error", err)
			return Render(c, components.AlertError("Version not found"))
		}

		pv, err := s.PluginVersionGetByPluginAndVersion(c.Context(), p.ID, version)
		if err != nil {
			slog.Warn("version fragment: version not found", "version", version, "error", err)
			return Render(c, components.AlertError("Version not found"))
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

// htmxComponentList renders a list of components sequentially.
func htmxComponentList(list []templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		for _, c := range list {
			if err := c.Render(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
}

// browseFragment renders the plugin grid fragment used by SearchFragment.
func browseFragment(plugins []store.PluginRecord, namespaceNames map[int]string, total, limit, offset int, query string) templ.Component {
	if len(plugins) == 0 {
		return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
			_, err := io.WriteString(w, `<div class="flex flex-col items-center justify-center py-8 text-center"><p class="text-base-content/60">No plugins matching "`+query+`". Try a different search term.</p></div>`)
			return err
		})
	}

	var parts []templ.Component
	for _, p := range plugins {
		parts = append(parts, components.PluginCard(namespaceNames[p.NamespaceID], p.Name, p.DisplayName, p.Description, p.LogoURL))
	}
	if offset+limit < total {
		parts = append(parts, components.LoadMoreButton(offset, limit, total, query))
	}
	return htmxComponentList(parts)
}
