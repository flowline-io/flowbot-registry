// Package handler contains HTTP request handlers for the registry API.
package handler

import (
	"log/slog"
	"net/http"

	"github.com/flowline-io/flowbot-registry/internal/service"
	"github.com/gofiber/fiber/v3"
)

// AuthTokenRequest represents query parameters for the token endpoint.
type AuthTokenRequest struct {
	Service  string `query:"service"`
	Scope    string `query:"scope"`
	ClientID string `query:"client_id"`
}

// AuthTokenHandler handles GET /api/v1/auth/token per Docker Registry v2 token auth.
func AuthTokenHandler(svc *service.AuthService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req AuthTokenRequest
		if err := c.Bind().Query(&req); err != nil {
			slog.Warn("auth token: invalid query", "error", err, "remote", c.IP())
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid query parameters",
			})
		}

		if req.Service == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "service parameter is required",
			})
		}

		clientID := getClientID(c)

		result, err := svc.IssueJWT(c.Context(), req.Service, clientID, req.Scope)
		if err != nil {
			slog.Warn("auth token: issue failed",
				"error", err, "service", req.Service, "scope", req.Scope,
				"client_id", clientID, "remote", c.IP(),
			)
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		slog.Info("auth token: issued",
			"service", req.Service, "scope", req.Scope,
			"client_id", clientID,
		)

		return c.JSON(result)
	}
}

// getClientID extracts the client identity from the request header.
func getClientID(c fiber.Ctx) string {
	apiKey := c.Get("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	authHeader := c.Get("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}

	return ""
}
