package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// POST /api/setup must issue a refresh cookie so the wizard's step 2 +
// step 3 calls can authenticate. Before this change the wizard ran
// entirely unauthenticated and /api/engine/* had to be whitelisted
// path-wide, which exposed /chat and /sessions/* during the setup
// window.
func TestSetup_IssuesRefreshCookie(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	body := `{"username":"admin","password":"verysecurepassword1","confirm_password":"verysecurepassword1"}`
	req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("setup returned %d: %s", rec.Code, rec.Body.String())
	}

	var refresh *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh" {
			refresh = c
			break
		}
	}
	if refresh == nil {
		t.Fatal("expected a refresh cookie on successful setup")
	}
	if refresh.Value == "" {
		t.Error("refresh cookie value is empty")
	}
	if !refresh.HttpOnly {
		t.Error("refresh cookie should be HttpOnly")
	}
}

// POST /api/setup/provider must require a default model server-side.
// Previously it accepted the request with zero validation, so a buggy
// or malicious client could flip the flag without any provider
// actually authenticated.
func TestProvider_RequiresDefaultModelServerSide(t *testing.T) {
	server := setupTestServer(t)

	// Force the default-model check to return false so we exercise
	// the new guard. The real orchestrator's callback queries the
	// stackllm profile manager, but we don't need a full stack here.
	server.SetDefaultModelCheck(func() bool { return false })
	handler := server.Handler()

	// Step 1 and step 2.
	token := createAccountAndGetToken(t, handler)
	auth := "Bearer " + token

	body := `{"agent_name":"Bot","personality":"balanced","user_name":"User","timezone":"UTC"}`
	req := httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
	req.Header.Set("Authorization", auth)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile step setup: %d: %s", rec.Code, rec.Body.String())
	}

	// Step 3 must now reject because no default model is set.
	req = httptest.NewRequest("POST", "/api/setup/provider", nil)
	req.Header.Set("Authorization", auth)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when default model missing, got %d: %s", rec.Code, rec.Body.String())
	}
	var data map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&data)
	if data["error"] != "default_model_required" {
		t.Errorf("unexpected error code: %q", data["error"])
	}
}

// The setup wizard now uses a narrow /api/engine/* whitelist. Paths
// the wizard does not need — /chat, /sessions/*, /providers/*/logout —
// must NOT be reachable during the setup window.
func TestSetupWhitelist_BlocksUnrelatedEngineEndpoints(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	// Create account so we're past the account step but still
	// mid-setup (profile + provider flags are false).
	body := `{"username":"admin","password":"verysecurepassword1","confirm_password":"verysecurepassword1"}`
	req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup step 1: %d", rec.Code)
	}

	blocked := []struct {
		method, path string
	}{
		{"POST", "/api/engine/chat"},
		{"GET", "/api/engine/sessions"},
		{"GET", "/api/engine/sessions/abc"},
		{"POST", "/api/engine/providers/openai/logout"},
	}
	for _, tc := range blocked {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		// The setup middleware returns 503 for non-whitelisted
		// paths while setup is incomplete.
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: expected 503, got %d: %s",
				tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

// Whitelisted /api/engine/* paths (provider list, login, model list,
// default) get past the setup 503 but STILL require authentication —
// the whitelist governs the setup gate, not withAuth. Without a
// token the request should 401, not 200/404.
func TestSetupWhitelist_WhitelistedEnginePathsStillRequireAuth(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	// Account created but no token attached on the request.
	body := `{"username":"admin","password":"verysecurepassword1","confirm_password":"verysecurepassword1"}`
	req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	whitelisted := []string{
		"/api/engine/providers",
		"/api/engine/models",
		"/api/engine/default",
	}
	for _, p := range whitelisted {
		req := httptest.NewRequest("GET", p, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without token: expected 401, got %d: %s",
				p, rec.Code, rec.Body.String())
		}
	}
}

// Positive case: with a valid bearer token the whitelisted engine
// paths pass both the setup middleware and withAuth. They reach the
// mount and return 503 from "engine handler not wired" in the test
// server — which is proof the request wasn't rejected earlier.
func TestSetupWhitelist_WhitelistedEnginePathsPassWithAuth(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	token := createAccountAndGetToken(t, handler)

	req := httptest.NewRequest("GET", "/api/engine/providers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Test server has no engine handler wired — the mount returns a
	// 503 "engine handler not wired" which is our sentinel that the
	// request reached the handler rather than being bounced by auth
	// or the setup middleware.
	if rec.Code != http.StatusServiceUnavailable ||
		!strings.Contains(rec.Body.String(), "engine handler not wired") {
		t.Errorf("expected engine-mount 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

// isSetupEngineEndpoint is the whitelist predicate — guard it with a
// direct unit test so a future tweak to the setup middleware can't
// silently widen or narrow the reachable surface.
func TestIsSetupEngineEndpoint_WhitelistShape(t *testing.T) {
	cases := []struct {
		path    string
		allowed bool
	}{
		// Allowed: provider login/status
		{"/api/engine/providers", true},
		{"/api/engine/providers/openai/login", true},
		{"/api/engine/providers/gemini/login", true},
		{"/api/engine/providers/ollama/login", true},
		{"/api/engine/providers/openai/oauth/login", true},
		{"/api/engine/providers/openai/oauth/status", true},
		{"/api/engine/providers/copilot/status", true},
		// Allowed: models + default
		{"/api/engine/models", true},
		{"/api/engine/models/openai", true},
		{"/api/engine/default", true},
		// Disallowed: everything else under /api/engine/
		{"/api/engine/chat", false},
		{"/api/engine/sessions", false},
		{"/api/engine/sessions/abc", false},
		{"/api/engine/providers/openai/logout", false},
		// Disallowed: outside /api/engine/ entirely
		{"/api/scripts", false},
		{"/random", false},
	}
	for _, c := range cases {
		if got := isSetupEngineEndpoint(c.path); got != c.allowed {
			t.Errorf("isSetupEngineEndpoint(%q) = %v, want %v", c.path, got, c.allowed)
		}
	}
}
