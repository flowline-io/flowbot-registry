package handler

import (
	"github.com/flowline-io/flowbot-registry/internal/service"
	"github.com/gofiber/fiber/v3"
)

// RegisterRoutes registers all HTTP API routes.
func RegisterRoutes(app *fiber.App, authSvc *service.AuthService, pluginSvc *service.PluginService, userSvc *service.UserService, registryURL string) {
	api := app.Group("/api/v1")

	api.Get("/auth/token", AuthTokenHandler(authSvc, userSvc))
	api.Get("/registry", RegistryInfoHandler(registryURL))
	api.Post("/plugins/:namespace/:name/publish", PublishHandler(pluginSvc))
	api.Get("/plugins", ListPluginsHandler(pluginSvc))
	api.Get("/plugins/:namespace/:name/versions/:version", GetVersionHandler(pluginSvc))
}
