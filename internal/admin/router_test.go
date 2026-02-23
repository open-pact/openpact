package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupTestServer(t *testing.T) *Server {
	tmpDir := t.TempDir()
	config := Config{
		Bind:          "localhost:8080",
		DataDir:       tmpDir,
		ScriptsDir:    tmpDir + "/scripts",
		WorkspacePath: tmpDir,
		DevMode:       true,
		AccessExpiry:  DefaultConfig().AccessExpiry,
		RefreshExpiry: DefaultConfig().RefreshExpiry,
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return server
}

// completeSetup runs both account creation and profile setup via the API.
func completeSetup(t *testing.T, handler http.Handler) {
	t.Helper()

	// Step 1: Create account
	body := `{"username": "admin", "password": "verysecurepassword1", "confirm_password": "verysecurepassword1"}`
	req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Account setup failed: %d: %s", rec.Code, rec.Body.String())
	}

	// Step 2: Complete profile
	body = `{"agent_name": "TestBot", "personality": "balanced", "user_name": "Tester", "timezone": "UTC"}`
	req = httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Profile setup failed: %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServer_SetupFlow(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	// Check setup required
	req := httptest.NewRequest("GET", "/api/setup/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var statusResp SetupStatusResponse
	json.NewDecoder(rec.Body).Decode(&statusResp)
	if !statusResp.SetupRequired {
		t.Error("Expected setup_required to be true")
	}
	if statusResp.SetupStep != "account" {
		t.Errorf("Expected setup_step 'account', got '%s'", statusResp.SetupStep)
	}

	// Non-setup endpoints should be blocked
	req = httptest.NewRequest("GET", "/api/scripts", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 when setup required, got %d", rec.Code)
	}

	// Complete full setup (account + profile)
	completeSetup(t, handler)

	// Setup should no longer be required
	req = httptest.NewRequest("GET", "/api/setup/status", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	json.NewDecoder(rec.Body).Decode(&statusResp)
	if statusResp.SetupRequired {
		t.Error("Expected setup_required to be false after setup")
	}
	if statusResp.SetupStep != "complete" {
		t.Errorf("Expected setup_step 'complete', got '%s'", statusResp.SetupStep)
	}
}

func TestServer_SetupFlow_ProfileStep(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	// Create account only
	body := `{"username": "admin", "password": "verysecurepassword1", "confirm_password": "verysecurepassword1"}`
	req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Status should show profile step
	req = httptest.NewRequest("GET", "/api/setup/status", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var statusResp SetupStatusResponse
	json.NewDecoder(rec.Body).Decode(&statusResp)
	if !statusResp.SetupRequired {
		t.Error("Expected setup_required to be true during profile step")
	}
	if statusResp.SetupStep != "profile" {
		t.Errorf("Expected setup_step 'profile', got '%s'", statusResp.SetupStep)
	}

	// Non-setup endpoints should still be blocked
	req = httptest.NewRequest("GET", "/api/scripts", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 when profile required, got %d", rec.Code)
	}

	// Verify SOUL.md is written after profile completion
	body = `{"agent_name": "Atlas", "personality": "friendly", "user_name": "Matt", "timezone": "Europe/London"}`
	req = httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	soulData, err := os.ReadFile(filepath.Join(server.config.WorkspacePath, "SOUL.md"))
	if err != nil {
		t.Fatalf("SOUL.md should exist: %v", err)
	}
	if !bytes.Contains(soulData, []byte("Atlas")) {
		t.Error("SOUL.md should contain agent name")
	}
}

func TestServer_AuthFlow(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	// Complete full setup
	completeSetup(t, handler)

	// Login
	body := `{"username": "admin", "password": "verysecurepassword1"}`
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Login failed: %d: %s", rec.Code, rec.Body.String())
	}

	// Get refresh cookie
	var refreshCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh" {
			refreshCookie = c
			break
		}
	}

	if refreshCookie == nil {
		t.Fatal("No refresh cookie returned")
	}

	// Get access token
	req = httptest.NewRequest("GET", "/api/session", nil)
	req.AddCookie(refreshCookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Session failed: %d", rec.Code)
	}

	var sessionResp SessionResponse
	json.NewDecoder(rec.Body).Decode(&sessionResp)

	if sessionResp.AccessToken == "" {
		t.Fatal("No access token returned")
	}

	// Access protected endpoint
	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+sessionResp.AccessToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Access without token should fail
	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without token, got %d", rec.Code)
	}
}

func TestServer_ScriptsCRUD(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	// Setup and login
	setupAndGetToken := func() string {
		// Full setup
		completeSetup(t, handler)

		// Login
		body := `{"username": "admin", "password": "verysecurepassword1"}`
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var refreshCookie *http.Cookie
		for _, c := range rec.Result().Cookies() {
			if c.Name == "refresh" {
				refreshCookie = c
				break
			}
		}

		if refreshCookie == nil {
			t.Fatal("No refresh cookie returned")
		}

		// Get access token
		req = httptest.NewRequest("GET", "/api/session", nil)
		req.AddCookie(refreshCookie)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var sessionResp SessionResponse
		json.NewDecoder(rec.Body).Decode(&sessionResp)
		return sessionResp.AccessToken
	}

	token := setupAndGetToken()

	// Create script
	body := `{"name": "test.star", "source": "def main(): pass"}`
	req := httptest.NewRequest("POST", "/api/scripts", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List scripts
	req = httptest.NewRequest("GET", "/api/scripts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var listResp struct {
		Scripts []*Script `json:"scripts"`
	}
	json.NewDecoder(rec.Body).Decode(&listResp)

	if len(listResp.Scripts) != 1 {
		t.Errorf("Expected 1 script, got %d", len(listResp.Scripts))
	}

	// Get script
	req = httptest.NewRequest("GET", "/api/scripts/test.star", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var script Script
	json.NewDecoder(rec.Body).Decode(&script)

	if script.Name != "test.star" {
		t.Errorf("Expected name 'test.star', got '%s'", script.Name)
	}

	if script.Status != StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", script.Status)
	}

	// Approve script
	req = httptest.NewRequest("POST", "/api/scripts/test.star/approve", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	json.NewDecoder(rec.Body).Decode(&script)
	if script.Status != StatusApproved {
		t.Errorf("Expected status 'approved', got '%s'", script.Status)
	}

	// Update script
	body = `{"source": "def main(): return 'updated'"}`
	req = httptest.NewRequest("PUT", "/api/scripts/test.star", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	json.NewDecoder(rec.Body).Decode(&script)
	if script.Status != StatusPending {
		t.Errorf("Expected status 'pending' after update, got '%s'", script.Status)
	}

	// Delete script
	req = httptest.NewRequest("DELETE", "/api/scripts/test.star", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", rec.Code)
	}

	// Verify deleted
	req = httptest.NewRequest("GET", "/api/scripts/test.star", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rec.Code)
	}
}

func TestServer_RejectScript(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	// Full setup
	completeSetup(t, handler)

	// Login
	body := `{"username": "admin", "password": "verysecurepassword1"}`
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var refreshCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh" {
			refreshCookie = c
			break
		}
	}

	req = httptest.NewRequest("GET", "/api/session", nil)
	req.AddCookie(refreshCookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var sessionResp SessionResponse
	json.NewDecoder(rec.Body).Decode(&sessionResp)
	token := sessionResp.AccessToken

	// Create script
	body = `{"name": "bad.star", "source": "def main(): evil()"}`
	req = httptest.NewRequest("POST", "/api/scripts", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Reject script
	body = `{"reason": "Contains suspicious code"}`
	req = httptest.NewRequest("POST", "/api/scripts/bad.star/reject", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var script Script
	json.NewDecoder(rec.Body).Decode(&script)

	if script.Status != StatusRejected {
		t.Errorf("Expected status 'rejected', got '%s'", script.Status)
	}

	if script.RejectReason != "Contains suspicious code" {
		t.Errorf("Expected reject_reason 'Contains suspicious code', got '%s'", script.RejectReason)
	}
}
