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
