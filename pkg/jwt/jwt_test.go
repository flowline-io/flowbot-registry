package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKey(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "private.pem")
	err = os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600)
	require.NoError(t, err)

	return keyPath
}

func TestNewTokenService(t *testing.T) {
	tests := []struct {
		name      string
		keyPath   string
		wantError bool
	}{
		{
			name:      "valid key",
			keyPath:   "", // Set below
			wantError: false,
		},
		{
			name:      "nonexistent key file",
			keyPath:   "/nonexistent/key.pem",
			wantError: true,
		},
		{
			name:      "invalid key file",
			keyPath:   "", // Set below
			wantError: true,
		},
	}

	validKey := generateTestKey(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kp := tt.keyPath
			switch tt.name {
			case "valid key":
				kp = validKey
			case "invalid key file":
				kp = filepath.Join(t.TempDir(), "invalid.pem")
				require.NoError(t, os.WriteFile(kp, []byte("not a key"), 0o600))
			}

			svc, err := NewTokenService(kp, "test-issuer", time.Hour)
			if tt.wantError {
				require.Error(t, err)
				assert.Nil(t, svc)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, svc)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	keyPath := generateTestKey(t)
	svc, err := NewTokenService(keyPath, "test-issuer", time.Hour)
	require.NoError(t, err)

	tests := []struct {
		name     string
		service  string
		accesses []AccessEntry
		subject  string
	}{
		{
			name:    "single repository pull",
			service: "registry.example.com",
			accesses: []AccessEntry{
				{Type: "repository", Name: "community/my-plugin", Actions: []string{"pull"}},
			},
			subject: "user-1",
		},
		{
			name:    "multiple repositories with push",
			service: "registry.example.com",
			accesses: []AccessEntry{
				{Type: "repository", Name: "org/plugin-a", Actions: []string{"pull", "push"}},
				{Type: "repository", Name: "org/plugin-b", Actions: []string{"pull"}},
			},
			subject: "org-owner",
		},
		{
			name:     "empty scopes",
			service:  "registry.example.com",
			accesses: []AccessEntry{},
			subject:  "anonymous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.GenerateToken(tt.service, tt.accesses, tt.subject)
			require.NoError(t, err)
			assert.NotEmpty(t, result.Token)
			assert.Equal(t, int64(3600), result.ExpiresIn)
			assert.NotEmpty(t, result.IssuedAt)
		})
	}
}
