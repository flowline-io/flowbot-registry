package handler

import (
	"io"
	"net/http"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/json"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registryInfoResponse mirrors the JSON response from GET /api/v1/registry.
type registryInfoResponse struct {
	URL string `json:"url"`
}

func TestRegistryInfoHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		registryURL string
	}{
		{
			name:        "happy path: returns configured registry URL",
			registryURL: "http://localhost:5000",
		},
		{
			name:        "custom registry URL",
			registryURL: "ghcr.io",
		},
		{
			name:        "empty registry URL",
			registryURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp()
			app.Get("/api/v1/registry", RegistryInfoHandler(tt.registryURL))

			req, _ := http.NewRequest(http.MethodGet, "/api/v1/registry", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var result registryInfoResponse
			err = json.Unmarshal(body, &result)
			require.NoError(t, err)

			assert.Equal(t, tt.registryURL, result.URL)
		})
	}
}

func TestRegistryInfoRouteRegistration(t *testing.T) {
	t.Parallel()

	app := newTestApp()
	app.Get("/api/v1/registry", RegistryInfoHandler("http://example.com"))

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/registry", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "http://example.com")
}

func newTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		AppName: "flowbot-registry-test",
	})
}
