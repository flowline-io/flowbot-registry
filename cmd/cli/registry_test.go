package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/json"
	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvOrFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		envKey   string
		envVal   string
		fallback string
		want     string
	}{
		{
			name:     "happy path: env var set returns env value",
			envKey:   "TEST_ENV_VAR",
			envVal:   "custom-value",
			fallback: "default",
			want:     "custom-value",
		},
		{
			name:     "env var empty returns fallback",
			envKey:   "TEST_MISSING_VAR",
			envVal:   "",
			fallback: "ghcr.io",
			want:     "ghcr.io",
		},
		{
			name:     "both empty returns fallback",
			envKey:   "TEST_ANOTHER_MISSING",
			envVal:   "",
			fallback: "http://localhost:5000",
			want:     "http://localhost:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(tt.envKey)
			if tt.envVal != "" {
				os.Setenv(tt.envKey, tt.envVal)
				t.Cleanup(func() { os.Unsetenv(tt.envKey) })
			}

			got := envOrFlag(tt.envKey, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchRegistryURLFromStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantURL string
		wantErr bool
	}{
		{
			name: "happy path: store returns registry URL",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"url":"http://registry:5000"}`))
			},
			wantURL: "http://registry:5000",
			wantErr: false,
		},
		{
			name: "error: store returns 500",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
		{
			name: "error: store returns empty URL",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"url":""}`))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			t.Cleanup(srv.Close)

			storeClient := &http.Client{}
			got, err := fetchRegistryURLFromStore(storeClient, srv.URL)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, got)
		})
	}
}

func TestRegistryInfoResponse_JSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		wantURL string
		wantErr bool
	}{
		{
			name:    "happy path: valid JSON",
			json:    `{"url":"ghcr.io"}`,
			wantURL: "ghcr.io",
			wantErr: false,
		},
		{
			name:    "valid JSON with extra fields",
			json:    `{"url":"http://local:5000","extra":"ignored"}`,
			wantURL: "http://local:5000",
			wantErr: false,
		},
		{
			name:    "error: invalid JSON",
			json:    `{"url":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp registryInfoResponse
			err := json.Unmarshal([]byte(tt.json), &resp)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, resp.URL)
		})
	}
}

func TestStripRegistryScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "happy path: strips http scheme",
			input: "http://localhost:5000",
			want:  "localhost:5000",
		},
		{
			name:  "strips https scheme",
			input: "https://ghcr.io",
			want:  "ghcr.io",
		},
		{
			name:  "no scheme unchanged",
			input: "ghcr.io",
			want:  "ghcr.io",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := oci.StripScheme(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
