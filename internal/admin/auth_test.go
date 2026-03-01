package admin

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJWTManager_CreateAndValidateAccessToken(t *testing.T) {
	manager := NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-key-12345678901234567890"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})

	token, expiresAt, err := manager.CreateAccessToken("testuser")
	if err != nil {
		t.Fatalf("CreateAccessToken failed: %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	if expiresAt.Before(time.Now()) {
		t.Error("Expected expiry to be in the future")
	}

	// Validate the token
	claims, err := manager.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if claims.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", claims.Username)
	}

	if claims.TokenType != AccessToken {
		t.Errorf("Expected token type 'access', got '%s'", claims.TokenType)
	}
}

func TestJWTManager_CreateAndValidateRefreshToken(t *testing.T) {
	manager := NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-key-12345678901234567890"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})

	token, expiresAt, err := manager.CreateRefreshToken("testuser")
	if err != nil {
		t.Fatalf("CreateRefreshToken failed: %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	// Refresh token should expire in ~72 hours
	expectedExpiry := time.Now().Add(72 * time.Hour)
	if expiresAt.Before(expectedExpiry.Add(-1*time.Minute)) || expiresAt.After(expectedExpiry.Add(1*time.Minute)) {
		t.Errorf("Expected expiry around 72 hours, got %v", expiresAt)
	}

	// Validate the token
	claims, err := manager.ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken failed: %v", err)
	}

	if claims.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", claims.Username)
	}

	if claims.TokenType != RefreshToken {
		t.Errorf("Expected token type 'refresh', got '%s'", claims.TokenType)
	}
}

func TestJWTManager_TokenTypeMismatch(t *testing.T) {
	manager := NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-key-12345678901234567890"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})

	// Create an access token
	accessToken, _, _ := manager.CreateAccessToken("testuser")

	// Try to validate it as a refresh token - should fail
	_, err := manager.ValidateRefreshToken(accessToken)
	if err != ErrInvalidTokenType {
		t.Errorf("Expected ErrInvalidTokenType, got %v", err)
	}

	// Create a refresh token
	refreshToken, _, _ := manager.CreateRefreshToken("testuser")

	// Try to validate it as an access token - should fail
	_, err = manager.ValidateAccessToken(refreshToken)
	if err != ErrInvalidTokenType {
		t.Errorf("Expected ErrInvalidTokenType, got %v", err)
	}
}

func TestJWTManager_InvalidToken(t *testing.T) {
	manager := NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-key-12345678901234567890"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})

	_, err := manager.ValidateAccessToken("invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	manager := NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-key-12345678901234567890"),
		AccessExpiry:  -1 * time.Hour, // Already expired
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})

	token, _, _ := manager.CreateAccessToken("testuser")

	_, err := manager.ValidateAccessToken(token)
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got %v", err)
	}
}

func TestJWTManager_WrongSecret(t *testing.T) {
	manager1 := NewJWTManager(JWTConfig{
		Secret:        []byte("secret-one-12345678901234567890"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})

	manager2 := NewJWTManager(JWTConfig{
		Secret:        []byte("secret-two-12345678901234567890"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})

	token, _, _ := manager1.CreateAccessToken("testuser")

	_, err := manager2.ValidateAccessToken(token)
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

func TestGetOrCreateJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()

	// First call should create a new secret
	secret1, err := GetOrCreateJWTSecret(tmpDir)
	if err != nil {
		t.Fatalf("GetOrCreateJWTSecret failed: %v", err)
	}

	if len(secret1) == 0 {
		t.Error("Expected non-empty secret")
	}

	// Second call should return the same secret
	secret2, err := GetOrCreateJWTSecret(tmpDir)
	if err != nil {
		t.Fatalf("GetOrCreateJWTSecret failed on second call: %v", err)
	}

	if string(secret1) != string(secret2) {
		t.Error("Expected same secret on second call")
	}

	// Verify file permissions
	info, err := os.Stat(filepath.Join(tmpDir, "jwt_secret"))
	if err != nil {
		t.Fatalf("Failed to stat jwt_secret file: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestGetOrCreateJWTSecret_EnvOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Set environment variable
	os.Setenv("ADMIN_JWT_SECRET", "env-secret-override")
	defer os.Unsetenv("ADMIN_JWT_SECRET")

	secret, err := GetOrCreateJWTSecret(tmpDir)
	if err != nil {
		t.Fatalf("GetOrCreateJWTSecret failed: %v", err)
	}

	if string(secret) != "env-secret-override" {
		t.Errorf("Expected 'env-secret-override', got '%s'", string(secret))
	}

	// Should not have created a file
	_, err = os.Stat(filepath.Join(tmpDir, "jwt_secret"))
	if !os.IsNotExist(err) {
		t.Error("Should not have created jwt_secret file when env var is set")
	}
}

func TestAuthMiddleware(t *testing.T) {
	manager := NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-key-12345678901234567890"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})

	// Create a test handler that checks the username in context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, ok := UsernameFromContext(r.Context())
		if !ok {
			t.Error("Expected username in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(username))
	})

	protected := manager.AuthMiddleware(handler)

	t.Run("valid token", func(t *testing.T) {
		token, _, _ := manager.CreateAccessToken("testuser")

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		if rec.Body.String() != "testuser" {
			t.Errorf("Expected body 'testuser', got '%s'", rec.Body.String())
		}
	})

	t.Run("missing header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("invalid header format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})
}

func TestShouldUseSecureCookies(t *testing.T) {
	tests := []struct {
		bind     string
		expected bool
	}{
		{"localhost:8080", false},
		{"127.0.0.1:8080", false},
		{"0.0.0.0:8080", true},
		{"192.168.1.1:8080", true},
	}

	for _, tt := range tests {
		result := ShouldUseSecureCookies(tt.bind)
		if result != tt.expected {
			t.Errorf("ShouldUseSecureCookies(%q) = %v, expected %v",
				tt.bind, result, tt.expected)
		}
	}
}

func TestSetRefreshCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	SetRefreshCookie(rec, "test-token", true)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "refresh" {
		t.Errorf("Expected cookie name 'refresh', got '%s'", cookie.Name)
	}
	if cookie.Value != "test-token" {
		t.Errorf("Expected cookie value 'test-token', got '%s'", cookie.Value)
	}
	if cookie.Path != "/api/session" {
		t.Errorf("Expected cookie path '/api/session', got '%s'", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Error("Expected HttpOnly to be true")
	}
	if !cookie.Secure {
		t.Error("Expected Secure to be true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Expected SameSite Strict, got %v", cookie.SameSite)
	}
}

func TestClearRefreshCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearRefreshCookie(rec, true)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "refresh" {
		t.Errorf("Expected cookie name 'refresh', got '%s'", cookie.Name)
	}
	if cookie.Value != "" {
		t.Errorf("Expected empty cookie value, got '%s'", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Errorf("Expected MaxAge -1, got %d", cookie.MaxAge)
	}
}
