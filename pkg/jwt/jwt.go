// Package jwt provides RS256 JWT signing per the Docker Registry v2 Token Authentication specification.
package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenService signs and issues JWT tokens for Docker Registry v2 authentication.
type TokenService struct {
	privateKey *rsa.PrivateKey
	issuer     string
	expiration time.Duration
}

// AccessEntry represents a single access entry in the JWT payload.
type AccessEntry struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

// TokenResponse is the response format for the token endpoint.
type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
	IssuedAt  string `json:"issued_at"`
}

// NewTokenService loads the RSA private key from the given path and creates a TokenService.
func NewTokenService(keyPath string, issuer string, expiration time.Duration) (*TokenService, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA private key")
	}

	return &TokenService{
		privateKey: rsaKey,
		issuer:     issuer,
		expiration: expiration,
	}, nil
}

// GenerateToken creates a signed JWT token with the given access entries.
func (s *TokenService) GenerateToken(service string, accesses []AccessEntry, subject string) (*TokenResponse, error) {
	now := time.Now()
	expiresAt := now.Add(s.expiration)

	claims := jwt.MapClaims{
		"iss":    s.issuer,
		"sub":    subject,
		"aud":    service,
		"exp":    expiresAt.Unix(),
		"iat":    now.Unix(),
		"nbf":    now.Unix(),
		"jti":    uuid.New().String(),
		"access": accesses,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &TokenResponse{
		Token:     tokenString,
		ExpiresIn: int64(s.expiration.Seconds()),
		IssuedAt:  now.Format(time.RFC3339),
	}, nil
}
