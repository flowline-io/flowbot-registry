package handler

import (
	"context"
	"time"

	"github.com/flowline-io/flowbot-registry/internal/middleware"
	"github.com/flowline-io/flowbot-registry/internal/service"
	"github.com/flowline-io/flowbot-registry/pkg/store"
	"github.com/gofiber/fiber/v3"
)

// RegisterRoutes registers all HTTP API routes.
func RegisterRoutes(app *fiber.App, authSvc *service.AuthService, userSvc *service.UserService, pluginSvc *service.PluginService, storeAdapter *store.Adapter, registryURL string) {
	api := app.Group("/api/v1")

	// Public: Registry info, plugin listing
	api.Get("/registry", RegistryInfoHandler(registryURL))
	api.Get("/plugins", ListPluginsHandler(pluginSvc))
	api.Get("/plugins/:namespace/:name/versions/:version", GetVersionHandler(pluginSvc))

	// Public: User auth (login has rate limiter)
	limiter := middleware.LoginRateLimit(10, time.Minute)
	api.Post("/auth/register", RegisterHandler(userSvc))
	api.Post("/auth/login", limiter, LoginHandler(userSvc))
	api.Post("/auth/refresh", RefreshHandler(userSvc))

	// Authenticated
	auth := api.Group("", middleware.AuthRequired(userSvc))

	// Docker v2 token (requires User access token)
	auth.Get("/auth/token", AuthTokenHandler(authSvc, userSvc))

	// Plugin publish (requires namespace ownership)
	nsLookup := func(ctx context.Context, name string) (*int, error) {
		ns, err := storeAdapter.NamespaceGetByName(ctx, name)
		if err != nil {
			return nil, err
		}
		return ns.UserID, nil
	}
	auth.Post("/plugins/:namespace/:name/publish", middleware.RequireNamespace(nsLookup), PublishHandler(pluginSvc))
}
