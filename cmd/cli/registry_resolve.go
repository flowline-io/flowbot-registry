package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/flowline-io/flowbot-registry/pkg/json"
	"github.com/spf13/cobra"
)

// registryInfoResponse represents the JSON response from GET /api/v1/registry.
type registryInfoResponse struct {
	URL string `json:"url"`
}

// fetchRegistryURLFromStore queries the store API for the configured OCI registry URL.
func fetchRegistryURLFromStore(client *http.Client, storeURL string) (string, error) {
	apiURL := fmt.Sprintf("%s/api/v1/registry", strings.TrimRight(storeURL, "/"))

	slog.Debug("fetching registry URL from store", "url", apiURL)

	resp, err := client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("store API returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var info registryInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if info.URL == "" {
		return "", fmt.Errorf("store returned empty registry URL")
	}

	return info.URL, nil
}

// resolveRegistryURLFromStore overrides registryURL with the store-configured
// value when the user did not explicitly set --registry or the env var.
func resolveRegistryURLFromStore(cmd *cobra.Command, storeURL string, currentRegistryURL string) string {
	if cmd.Flags().Changed("registry") {
		slog.Debug("using explicitly set registry URL", "url", currentRegistryURL)
		return currentRegistryURL
	}

	if url, err := fetchRegistryURLFromStore(httpClient, storeURL); err == nil {
		slog.Info("resolved registry URL from store", "url", url)
		return url
	}

	slog.Debug("could not resolve registry URL from store, using default", "url", currentRegistryURL)
	return currentRegistryURL
}
