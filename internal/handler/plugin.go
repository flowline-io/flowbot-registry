// Package handler contains HTTP request handlers for the registry API.
package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/flowline-io/flowbot-registry/internal/service"
	"github.com/gofiber/fiber/v3"
)

// PublishRequest represents the request body for plugin publish.
type PublishRequest struct {
	Version   string `json:"version"`
	OciDigest string `json:"oci_digest"`
}

// PublishHandler handles POST /api/v1/plugins/:namespace/:name/publish.
func PublishHandler(svc *service.PluginService) fiber.Handler {
	return func(c fiber.Ctx) error {
		namespace := c.Params("namespace")
		name := c.Params("name")

		var req PublishRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		if req.Version == "" || req.OciDigest == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "version and oci_digest are required",
			})
		}

		slog.Info("publish request",
			"namespace", namespace, "name", name,
			"version", req.Version, "digest", req.OciDigest,
		)

		result, err := svc.Publish(c.Context(), service.PublishRequest{
			Namespace: namespace,
			Name:      name,
			Version:   req.Version,
			OciDigest: req.OciDigest,
		})
		if err != nil {
			return handlePublishError(c, err, namespace, name)
		}

		slog.Info("publish success",
			"namespace", namespace, "name", name,
			"version", req.Version, "plugin_id", result.PluginID,
			"created", result.Created,
		)

		return c.Status(http.StatusCreated).JSON(result)
	}
}

func handlePublishError(c fiber.Ctx, err error, namespace, name string) error {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrForbidden):
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidInput):
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	default:
		slog.Error("publish failed",
			"error", err,
			"namespace", namespace, "name", name,
		)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("internal server error"),
		})
	}
}

// ListPluginsHandler handles GET /api/v1/plugins.
func ListPluginsHandler(svc *service.PluginService) fiber.Handler {
	return func(c fiber.Ctx) error {
		query := c.Query("q", "")
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		offset, _ := strconv.Atoi(c.Query("offset", "0"))

		if limit < 1 || limit > 100 {
			limit = 20
		}

		plugins, total, err := svc.ListPlugins(c.Context(), query, limit, offset)
		if err != nil {
			slog.Error("list plugins failed", "error", err, "query", query)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.JSON(fiber.Map{
			"plugins": plugins,
			"total":   total,
			"limit":   limit,
			"offset":  offset,
		})
	}
}

// GetVersionHandler handles GET /api/v1/plugins/:namespace/:name/versions/:version.
func GetVersionHandler(svc *service.PluginService) fiber.Handler {
	return func(c fiber.Ctx) error {
		namespace := c.Params("namespace")
		name := c.Params("name")
		version := c.Params("version")

		pv, err := svc.GetPluginVersion(c.Context(), namespace, name, version)
		if err != nil {
			if errors.Is(err, service.ErrNotFound) {
				return c.Status(http.StatusNotFound).JSON(fiber.Map{
					"error": "version not found",
				})
			}
			slog.Error("get version failed",
				"error", err,
				"namespace", namespace, "name", name, "version", version,
			)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.JSON(pv)
	}
}
