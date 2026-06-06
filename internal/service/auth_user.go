package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/flowline-io/flowbot-registry/pkg/jwt"
	"github.com/flowline-io/flowbot-registry/pkg/store"
	"golang.org/x/crypto/bcrypt"
)

// ErrUnauthorized is returned when credentials are invalid.
var ErrUnauthorized = errors.New("unauthorized")

// ErrConflict is returned when a resource already exists.
var ErrConflict = errors.New("conflict")

// bcryptCost is the cost parameter for bcrypt password hashing.
const bcryptCost = 12

// minPasswordLength is the minimum allowed password length.
const minPasswordLength = 8

// UserService provides user authentication operations (register, login, token refresh).
type UserService struct {
	store             *store.Adapter
	jwtSvc            *jwt.UserTokenService
	refreshExpiration time.Duration
}

// NewUserService creates a new UserService.
func NewUserService(a *store.Adapter, jwtSvc *jwt.UserTokenService, refreshExpiration time.Duration) *UserService {
	return &UserService{
		store:             a,
		jwtSvc:            jwtSvc,
		refreshExpiration: refreshExpiration,
	}
}

// AuthResult contains the result of a successful authentication.
type AuthResult struct {
	User         store.UserRecord `json:"user"`
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresAt    time.Time        `json:"expires_at"`
}

// Register creates a new user, namespace, and issues tokens.
func (s *UserService) Register(ctx context.Context, email string, password string) (*AuthResult, error) {
	if len(password) < minPasswordLength {
		return nil, fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLength)
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.store.UserCreate(ctx, email, hash)
	if err != nil {
		slog.Error("register: user create failed", "email", email, "error", err)
		return nil, fmt.Errorf("%w: %w", ErrConflict, err)
	}

	// Auto-create namespace for the user
	_, err = s.store.NamespaceCreate(ctx, emailToNamespace(email), "user", user.ID)
	if err != nil {
		slog.Warn("register: namespace create failed (non-fatal)", "email", email, "error", err)
	}

	return s.issueTokens(ctx, user)
}

// Login validates credentials and issues tokens.
func (s *UserService) Login(ctx context.Context, email string, password string) (*AuthResult, error) {
	user, err := s.store.UserGetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%w: invalid email or password", ErrUnauthorized)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if !checkPassword(user.PasswordHash, password) {
		return nil, fmt.Errorf("%w: invalid email or password", ErrUnauthorized)
	}

	return s.issueTokens(ctx, user)
}

// RefreshToken validates a refresh token and issues a new token pair.
// The old refresh token is deleted (rotation) to prevent reuse.
func (s *UserService) RefreshToken(ctx context.Context, rawRefreshToken string) (*AuthResult, error) {
	if rawRefreshToken == "" {
		return nil, fmt.Errorf("%w: refresh token required", ErrInvalidInput)
	}

	tokenHash := jwt.HashToken(rawRefreshToken)

	rt, err := s.store.RefreshTokenGetByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}

	if time.Now().After(rt.ExpiresAt) {
		_ = s.store.RefreshTokenDeleteByID(ctx, rt.ID)
		return nil, fmt.Errorf("%w: refresh token expired", ErrUnauthorized)
	}

	// Delete old token (rotation)
	if err := s.store.RefreshTokenDeleteByID(ctx, rt.ID); err != nil {
		slog.Warn("refresh: failed to delete old token", "error", err)
	}

	user, err := s.store.UserGetByID(ctx, rt.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return s.issueTokens(ctx, user)
}

// ValidateAccessToken parses and validates a user access token JWT.
func (s *UserService) ValidateAccessToken(tokenStr string) (*jwt.AccessTokenClaims, error) {
	if tokenStr == "" {
		return nil, fmt.Errorf("%w: token required", ErrInvalidInput)
	}
	return s.jwtSvc.ParseAccessToken(tokenStr)
}

func (s *UserService) issueTokens(ctx context.Context, user *store.UserRecord) (*AuthResult, error) {
	accessToken, expiresAt, err := s.jwtSvc.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.jwtSvc.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	refreshHash := jwt.HashToken(refreshToken)
	refreshExpiresAt := time.Now().Add(s.refreshExpiration)
	if _, err := s.store.RefreshTokenCreate(ctx, user.ID, refreshHash, refreshExpiresAt); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &AuthResult{
		User:         *user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// emailToNamespace extracts the local part of an email as the namespace name.
func emailToNamespace(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}

// hashPassword returns the bcrypt hash of a password.
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(bytes), nil
}

// checkPassword compares a password against a bcrypt hash.
func checkPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
