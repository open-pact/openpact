// Package admin provides the web-based administration interface for OpenPact.
package admin

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token expired")
	ErrInvalidTokenType = errors.New("invalid token type")
)

// TokenType distinguishes between access and refresh tokens.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims represents the JWT claims for OpenPact tokens.
type Claims struct {
	Username  string    `json:"sub"`
	TokenType TokenType `json:"type"`
	jwt.RegisteredClaims
}

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	Secret        []byte
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
	Issuer        string
}

// JWTManager handles JWT token creation and validation.
type JWTManager struct {
	config JWTConfig
}

// NewJWTManager creates a new JWT manager with the given configuration.
func NewJWTManager(config JWTConfig) *JWTManager {
	return &JWTManager{config: config}
}

// CreateAccessToken creates a short-lived access token.
func (m *JWTManager) CreateAccessToken(username string) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.config.AccessExpiry)
	claims := Claims{
		Username:  username,
		TokenType: AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    m.config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.config.Secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// CreateRefreshToken creates a long-lived refresh token.
func (m *JWTManager) CreateRefreshToken(username string) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.config.RefreshExpiry)
	claims := Claims{
		Username:  username,
		TokenType: RefreshToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    m.config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.config.Secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// ValidateAccessToken validates an access token and returns the claims.
func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	return m.validateToken(tokenString, AccessToken)
}

// ValidateRefreshToken validates a refresh token and returns the claims.
func (m *JWTManager) ValidateRefreshToken(tokenString string) (*Claims, error) {
	return m.validateToken(tokenString, RefreshToken)
}

func (m *JWTManager) validateToken(tokenString string, expectedType TokenType) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.config.Secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.TokenType != expectedType {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// GetOrCreateJWTSecret loads the JWT secret from disk or creates a new one.
// Environment variable ADMIN_JWT_SECRET takes priority if set.
func GetOrCreateJWTSecret(dataDir string) ([]byte, error) {
	// Env var takes priority (useful for Docker/K8s)
	if secret := os.Getenv("ADMIN_JWT_SECRET"); secret != "" {
		return []byte(secret), nil
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Try to load existing secret
	path := filepath.Join(dataDir, "jwt_secret")
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	// Generate new 256-bit secret
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(secret)

	// Save with restrictive permissions
	if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
		return nil, fmt.Errorf("failed to save secret: %w", err)
	}

	return []byte(encoded), nil
}

// AuthMiddleware creates middleware that validates access tokens.
func (m *JWTManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := m.ValidateAccessToken(parts[1])
		if err != nil {
			if errors.Is(err, ErrExpiredToken) {
				http.Error(w, "Token expired", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add username to request context
		r = r.WithContext(WithUsername(r.Context(), claims.Username))
		next.ServeHTTP(w, r)
	})
}

// ShouldUseSecureCookies determines if secure cookies should be used.
// Returns false for localhost development, true for production.
func ShouldUseSecureCookies(bind string) bool {
	if strings.HasPrefix(bind, "localhost") || strings.HasPrefix(bind, "127.0.0.1") {
		return false
	}
	return true
}

// SetRefreshCookie sets the refresh token as an HTTP-only cookie.
func SetRefreshCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh",
		Value:    token,
		Path:     "/api/session",
		MaxAge:   3 * 24 * 60 * 60, // 3 days
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearRefreshCookie clears the refresh token cookie.
func ClearRefreshCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh",
		Value:    "",
		Path:     "/api/session",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}
