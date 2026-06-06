package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// UserTokenService signs and validates user authentication JWT tokens.
type UserTokenService struct {
	privateKey *rsa.PrivateKey
	issuer     string
	expiration time.Duration
}

// AccessTokenClaims represents the claims in a user access token.
type AccessTokenClaims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	JTI    string `json:"jti"`
}

// NewUserTokenService loads an RSA private key from a PEM file.
func NewUserTokenService(keyPath string, issuer string, expiration time.Duration) (*UserTokenService, error) {
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

	return &UserTokenService{
		privateKey: rsaKey,
		issuer:     issuer,
		expiration: expiration,
	}, nil
}

// GenerateAccessToken creates a signed RS256 JWT access token.
func (s *UserTokenService) GenerateAccessToken(userID int, email string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.expiration)

	claims := jwt.MapClaims{
		"iss":     s.issuer,
		"sub":     fmt.Sprintf("%d", userID),
		"exp":     expiresAt.Unix(),
		"iat":     now.Unix(),
		"nbf":     now.Unix(),
		"jti":     uuid.New().String(),
		"user_id": userID,
		"email":   email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// GenerateRefreshToken creates a random 256-bit hex string for use as a refresh token.
func (*UserTokenService) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ParseAccessToken validates an access token and returns its claims.
func (s *UserTokenService) ParseAccessToken(tokenStr string) (*AccessTokenClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return &s.privateKey.PublicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("user_id claim missing or invalid")
	}

	email, ok := claims["email"].(string)
	if !ok {
		return nil, fmt.Errorf("email claim missing or invalid")
	}

	var jti string
	if v, ok := claims["jti"].(string); ok {
		jti = v
	}

	return &AccessTokenClaims{
		UserID: int(userIDFloat),
		Email:  email,
		JTI:    jti,
	}, nil
}

// HashToken returns the SHA256 hex hash of a token string (for refresh token storage).
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
