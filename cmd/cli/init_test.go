package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInitName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNs     string
		wantName   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "valid namespace/name",
			input:    "community/my-plugin",
			wantNs:   "community",
			wantName: "my-plugin",
			wantErr:  false,
		},
		{
			name:     "valid org with hyphen",
			input:    "my-org/github-stars",
			wantNs:   "my-org",
			wantName: "github-stars",
			wantErr:  false,
		},
		{
			name:       "bare name without slash fails",
			input:      "my-plugin",
			wantErr:    true,
			wantErrMsg: "must be in format",
		},
		{
			name:       "empty namespace fails",
			input:      "/my-plugin",
			wantErr:    true,
			wantErrMsg: "must not be empty",
		},
		{
			name:       "empty name fails",
			input:      "community/",
			wantErr:    true,
			wantErrMsg: "must not be empty",
		},
		{
			name:       "empty string fails",
			input:      "",
			wantErr:    true,
			wantErrMsg: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, name, err := parseInitName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantNs, ns)
				assert.Equal(t, tt.wantName, name)
			}
		})
	}
}

func TestInitFileCreation(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "my-plugin")
	require.NoError(t, runInit(targetDir, "community", "my-plugin", "grpc"))

	for _, path := range []string{"plugin.yaml", "go.mod", "main.go", filepath.Join("cmd", "server", "main.go")} {
		_, err := os.Stat(filepath.Join(targetDir, path))
		require.NoError(t, err, "expected file %s to exist", path)
	}
}

func TestInitFileCreationWasm(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "my-wasm")
	require.NoError(t, runInit(targetDir, "community", "my-wasm", "wasm"))

	for _, path := range []string{"plugin.yaml", "go.mod", "main.go"} {
		_, err := os.Stat(filepath.Join(targetDir, path))
		require.NoError(t, err, "expected file %s to exist", path)
	}
	_, err := os.Stat(filepath.Join(targetDir, "cmd", "server", "main.go"))
	require.True(t, os.IsNotExist(err))
}
