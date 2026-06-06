// Package handler contains HTTP request handlers for the registry API.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/flowline-io/flowbot-registry/internal/service"
	"github.com/gofiber/fiber/v3"
)

// RegisterRequest represents the request body for user registration.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest represents the request body for user login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshRequest represents the request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RegisterHandler handles POST /api/v1/auth/register.
func RegisterHandler(svc *service.UserService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req RegisterRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		if req.Email == "" || req.Password == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "email and password are required",
			})
		}

		result, err := svc.Register(c.Context(), req.Email, req.Password)
		if err != nil {
			return handleAuthError(c, err)
		}

		slog.Info("register: user created", "email", req.Email, "user_id", result.User.ID)

		return c.Status(http.StatusCreated).JSON(result)
	}
}

// LoginHandler handles POST /api/v1/auth/login.
func LoginHandler(svc *service.UserService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req LoginRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		if req.Email == "" || req.Password == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "email and password are required",
			})
		}

		result, err := svc.Login(c.Context(), req.Email, req.Password)
		if err != nil {
			return handleAuthError(c, err)
		}

		slog.Info("login: successful", "email", req.Email)

		return c.JSON(result)
	}
}

// RefreshHandler handles POST /api/v1/auth/refresh.
func RefreshHandler(svc *service.UserService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req RefreshRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		if req.RefreshToken == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "refresh_token is required",
			})
		}

		result, err := svc.RefreshToken(c.Context(), req.RefreshToken)
		if err != nil {
			return handleAuthError(c, err)
		}

		slog.Info("refresh: token refreshed", "user_id", result.User.ID)

		return c.JSON(result)
	}
}

func handleAuthError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrUnauthorized):
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	case errors.Is(err, service.ErrConflict):
		return c.Status(http.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidInput):
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	default:
		slog.Error("auth error", "error", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
