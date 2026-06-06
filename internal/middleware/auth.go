// Package middleware provides Fiber middleware for authentication and authorization.
package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/flowline-io/flowbot-registry/internal/service"
)

// AuthRequired validates the Bearer token from the Authorization header.
// On success it injects user_id and email into Locals. On failure it returns 401.
func AuthRequired(userSvc *service.UserService) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			slog.Warn("auth: missing or malformed Authorization header", "remote", c.IP())
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		token := authHeader[7:]
		claims, err := userSvc.ValidateAccessToken(token)
		if err != nil {
			slog.Warn("auth: invalid access token", "error", err, "remote", c.IP())
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)

		return c.Next()
	}
}

// RequireNamespace ensures the authenticated user owns the namespace specified in the URL.
// It must be used after AuthRequired middleware.
// The lookup function retrieves the namespace owner's user ID.
// Returns nil user ID for unowned namespaces, or an error if not found.
func RequireNamespace(lookup func(ctx context.Context, name string) (ownerID *int, err error)) fiber.Handler {
	return func(c fiber.Ctx) error {
		uid, ok := c.Locals("user_id").(int)
		if !ok {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		nsName := c.Params("namespace")

		ownerID, err := lookup(c.Context(), nsName)
		if err != nil {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
		}

		if ownerID == nil || *ownerID != uid {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
		}

		return c.Next()
	}
}
