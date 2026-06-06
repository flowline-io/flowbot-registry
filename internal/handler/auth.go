// Package handler contains HTTP request handlers for the registry API.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/flowline-io/flowbot-registry/internal/service"
)

// AuthTokenRequest represents query parameters for the token endpoint.
type AuthTokenRequest struct {
	Service  string `query:"service"`
	Scope    string `query:"scope"`
	ClientID string `query:"client_id"`
}

// AuthTokenHandler handles GET /api/v1/auth/token per Docker Registry v2 token auth.
// Requires User access token in Authorization header for push scopes.
func AuthTokenHandler(authSvc *service.AuthService, userSvc *service.UserService) fiber.Handler {
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

		// Extract user identity from User access token
		userID := 0
		authHeader := c.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token := authHeader[7:]
			claims, err := userSvc.ValidateAccessToken(token)
			if err == nil {
				userID = claims.UserID
			} else {
				slog.Warn("auth token: invalid user token", "error", err, "remote", c.IP())
			}
		}

		clientID := getClientID(c)

		result, err := authSvc.IssueJWT(c.Context(), req.Service, clientID, req.Scope, userID)
		if err != nil {
			slog.Warn("auth token: issue failed",
				"error", err, "service", req.Service, "scope", req.Scope,
				"client_id", clientID, "remote", c.IP(),
			)
			if errors.Is(err, service.ErrForbidden) {
				return c.Status(http.StatusForbidden).JSON(fiber.Map{
					"error": err.Error(),
				})
			}
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		slog.Info("auth token: issued",
			"service", req.Service, "scope", req.Scope,
			"client_id", clientID, "user_id", userID,
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
