package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-pact/openpact/internal/storage"
	"github.com/open-pact/openpact/internal/storage/users"
)

func newTestJWTManager() *JWTManager {
	return NewJWTManager(JWTConfig{
		Secret:        []byte("test-secret-key-12345678901234567890"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
		Issuer:        "openpact-test",
	})
}

func newTestUserStore(t *testing.T) *users.Store {
	t.Helper()
	return users.NewStore(storage.NewTestDB(t))
}

func TestSessionHandler_Login(t *testing.T) {
	userStore := newTestUserStore(t)
	userStore.Create(context.Background(), "admin", "password1234567890")
	jwt := newTestJWTManager()
	handler := NewSessionHandler(userStore, jwt, false)

	t.Run("successful login", func(t *testing.T) {
		body := `{"username": "admin", "password": "password1234567890"}`
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Check refresh cookie was set
		cookies := rec.Result().Cookies()
		var refreshCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "refresh" {
				refreshCookie = c
				break
			}
		}

		if refreshCookie == nil {
			t.Error("Expected refresh cookie to be set")
		}
	})

	t.Run("invalid credentials", func(t *testing.T) {
		body := `{"username": "admin", "password": "wrong"}`
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString("{not json"))
		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})
}

func TestSessionHandler_Session(t *testing.T) {
	userStore := newTestUserStore(t)
	userStore.Create(context.Background(), "admin", "password1234567890")
	jwt := newTestJWTManager()
	handler := NewSessionHandler(userStore, jwt, false)

	t.Run("valid refresh token", func(t *testing.T) {
		refreshToken, _, _ := jwt.CreateRefreshToken("admin")

		req := httptest.NewRequest("GET", "/api/session", nil)
		req.AddCookie(&http.Cookie{
			Name:  "refresh",
			Value: refreshToken,
		})
		rec := httptest.NewRecorder()

		handler.Session(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp SessionResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp.AccessToken == "" {
			t.Error("Expected access_token to be returned")
		}

		if resp.Username != "admin" {
			t.Errorf("Expected username 'admin', got '%s'", resp.Username)
		}

		// Verify the access token is valid
		claims, err := jwt.ValidateAccessToken(resp.AccessToken)
		if err != nil {
			t.Errorf("Access token should be valid: %v", err)
		}

		if claims.Username != "admin" {
			t.Errorf("Expected claims username 'admin', got '%s'", claims.Username)
		}
	})

	t.Run("no refresh cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/session", nil)
		rec := httptest.NewRecorder()

		handler.Session(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/session", nil)
		req.AddCookie(&http.Cookie{
			Name:  "refresh",
			Value: "invalid-token",
		})
		rec := httptest.NewRecorder()

		handler.Session(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("expired refresh token", func(t *testing.T) {
		// Create JWT manager with already-expired refresh tokens
		expiredJWT := NewJWTManager(JWTConfig{
			Secret:        []byte("test-secret-key-12345678901234567890"),
			AccessExpiry:  15 * time.Minute,
			RefreshExpiry: -1 * time.Hour, // Already expired
			Issuer:        "openpact-test",
		})

		refreshToken, _, _ := expiredJWT.CreateRefreshToken("admin")

		req := httptest.NewRequest("GET", "/api/session", nil)
		req.AddCookie(&http.Cookie{
			Name:  "refresh",
			Value: refreshToken,
		})
		rec := httptest.NewRecorder()

		handler.Session(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})
}

func TestSessionHandler_Logout(t *testing.T) {
	userStore := newTestUserStore(t)
	jwt := newTestJWTManager()
	handler := NewSessionHandler(userStore, jwt, false)

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", rec.Code)
	}

	// Check cookie is cleared
	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh" {
			refreshCookie = c
			break
		}
	}

	if refreshCookie == nil {
		t.Error("Expected refresh cookie to be set (for clearing)")
	}

	if refreshCookie.MaxAge != -1 {
		t.Errorf("Expected MaxAge -1 to clear cookie, got %d", refreshCookie.MaxAge)
	}
}

func TestSessionHandler_Me(t *testing.T) {
	userStore := newTestUserStore(t)
	jwt := newTestJWTManager()
	handler := NewSessionHandler(userStore, jwt, false)

	t.Run("authenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		req = req.WithContext(WithUsername(req.Context(), "admin"))
		rec := httptest.NewRecorder()

		handler.Me(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var resp map[string]string
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp["username"] != "admin" {
			t.Errorf("Expected username 'admin', got '%s'", resp["username"])
		}

		if resp["role"] != "admin" {
			t.Errorf("Expected role 'admin', got '%s'", resp["role"])
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		rec := httptest.NewRecorder()

		handler.Me(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})
}

func TestFullAuthFlow(t *testing.T) {
	userStore := newTestUserStore(t)
	userStore.Create(context.Background(), "admin", "password1234567890")
	jwt := newTestJWTManager()
	sessionHandler := NewSessionHandler(userStore, jwt, false)

	// Step 1: Login
	loginBody := `{"username": "admin", "password": "password1234567890"}`
	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(loginBody))
	loginRec := httptest.NewRecorder()

	sessionHandler.Login(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Login failed: %d", loginRec.Code)
	}

	// Get refresh cookie
	var refreshCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "refresh" {
			refreshCookie = c
			break
		}
	}

	if refreshCookie == nil {
		t.Fatal("No refresh cookie returned")
	}

	// Step 2: Get access token via /api/session
	sessionReq := httptest.NewRequest("GET", "/api/session", nil)
	sessionReq.AddCookie(refreshCookie)
	sessionRec := httptest.NewRecorder()

	sessionHandler.Session(sessionRec, sessionReq)

	if sessionRec.Code != http.StatusOK {
		t.Fatalf("Session failed: %d: %s", sessionRec.Code, sessionRec.Body.String())
	}

	var sessionResp SessionResponse
	json.NewDecoder(sessionRec.Body).Decode(&sessionResp)

	if sessionResp.AccessToken == "" {
		t.Fatal("No access token returned")
	}

	// Step 3: Use access token to access protected endpoint
	claims, err := jwt.ValidateAccessToken(sessionResp.AccessToken)
	if err != nil {
		t.Fatalf("Access token invalid: %v", err)
	}

	if claims.Username != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", claims.Username)
	}

	// Step 4: Logout
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutRec := httptest.NewRecorder()

	sessionHandler.Logout(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Errorf("Logout failed: %d", logoutRec.Code)
	}
}
