package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/flowline-io/flowbot-registry/pkg/json"
)

// fetchOCIAuthToken obtains a Docker v2 Bearer token from the store's auth endpoint
// using the user's access token for authentication.
func fetchOCIAuthToken(storeURL, accessToken, scope, service string) (authn.Authenticator, error) {
	tokenURL := fmt.Sprintf("%s/api/v1/auth/token?service=%s&scope=%s",
		storeURL,
		url.QueryEscape(service),
		url.QueryEscape(scope),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create auth request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth token returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read auth token body: %w", err)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode auth token: %w", err)
	}

	return &authn.Bearer{Token: result.Token}, nil
}

// getAccessToken loads the access token from the stored CLI config, falling back to the apiKey.
func getAccessToken(apiKey string) string {
	if apiKey != "" {
		return apiKey
	}
	cfg, err := loadConfig()
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.AccessToken
}

// resolveOCIAuth creates an OCI authenticator for the given namespace and name.
// Returns nil if no access token is available.
func resolveOCIAuth(storeURL, accessToken, namespace, name string) authn.Authenticator {
	if accessToken == "" {
		return nil
	}
	scope := fmt.Sprintf("repository:%s/%s:pull,push", namespace, name)
	auth, err := fetchOCIAuthToken(storeURL, accessToken, scope, "flowbot-registry")
	if err != nil {
		slog.Warn("publish: failed to get OCI auth token, trying without", "error", err)
		return nil
	}
	return auth
}
