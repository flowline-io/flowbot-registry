package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserTokenService_GenerateAndParseAccessToken(t *testing.T) {
	keyPath := generateTestKey(t)
	svc, err := NewUserTokenService(keyPath, "test-issuer", time.Hour)
	require.NoError(t, err)

	tests := []struct {
		name   string
		userID int
		email  string
	}{
		{
			name:   "happy path: valid user",
			userID: 1,
			email:  "alice@example.com",
		},
		{
			name:   "another user",
			userID: 42,
			email:  "bob@test.org",
		},
		{
			name:   "user with zero ID",
			userID: 0,
			email:  "zero@test.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, expiresAt, err := svc.GenerateAccessToken(tt.userID, tt.email)
			require.NoError(t, err)
			assert.NotEmpty(t, token)
			assert.True(t, expiresAt.After(time.Now()))
			assert.True(t, expiresAt.Before(time.Now().Add(2*time.Hour)))

			claims, err := svc.ParseAccessToken(token)
			require.NoError(t, err)
			assert.Equal(t, tt.userID, claims.UserID)
			assert.Equal(t, tt.email, claims.Email)
			assert.NotEmpty(t, claims.JTI)
		})
	}
}

func TestUserTokenService_ParseExpiredToken(t *testing.T) {
	keyPath := generateTestKey(t)
	svc, err := NewUserTokenService(keyPath, "test-issuer", 1*time.Millisecond)
	require.NoError(t, err)

	token, _, err := svc.GenerateAccessToken(1, "test@test.com")
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, err = svc.ParseAccessToken(token)
	assert.Error(t, err)
}

func TestUserTokenService_ParseTamperedToken(t *testing.T) {
	keyPath := generateTestKey(t)
	svc, err := NewUserTokenService(keyPath, "test-issuer", time.Hour)
	require.NoError(t, err)

	token, _, err := svc.GenerateAccessToken(1, "test@test.com")
	require.NoError(t, err)

	tampered := token + "tampered"

	_, err = svc.ParseAccessToken(tampered)
	assert.Error(t, err)
}

func TestUserTokenService_GenerateRefreshToken(t *testing.T) {
	keyPath := generateTestKey(t)
	svc, err := NewUserTokenService(keyPath, "test-issuer", time.Hour)
	require.NoError(t, err)

	t.Run("generates different tokens on each call", func(t *testing.T) {
		token1, err := svc.GenerateRefreshToken()
		require.NoError(t, err)
		token2, err := svc.GenerateRefreshToken()
		require.NoError(t, err)
		assert.NotEqual(t, token1, token2)
	})

	t.Run("generates 64-character hex string", func(t *testing.T) {
		token, err := svc.GenerateRefreshToken()
		require.NoError(t, err)
		assert.Len(t, token, 64)
	})
}

func TestNewUserTokenService(t *testing.T) {
	tests := []struct {
		name      string
		keyPath   string
		wantError bool
	}{
		{
			name:      "valid key",
			keyPath:   "", // set below
			wantError: false,
		},
		{
			name:      "nonexistent key file",
			keyPath:   "/nonexistent/key.pem",
			wantError: true,
		},
		{
			name:      "invalid key file",
			keyPath:   "", // set below
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
				kp = t.TempDir() + "/invalid.pem"
			}

			svc, err := NewUserTokenService(kp, "test-issuer", time.Hour)
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
