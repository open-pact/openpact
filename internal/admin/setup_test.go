package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-pact/openpact/internal/storage/kv"
)

func TestSetupHandler_Status(t *testing.T) {
	fx := newTestSetupFixture(t)

	t.Run("setup required when no users", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/setup/status", nil)
		rec := httptest.NewRecorder()

		fx.handler.Status(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var resp SetupStatusResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if !resp.SetupRequired {
			t.Error("Expected setup_required to be true")
		}
		if resp.SetupStep != "account" {
			t.Errorf("Expected setup_step 'account', got '%s'", resp.SetupStep)
		}
	})

	t.Run("profile step when user exists but no profile", func(t *testing.T) {
		fx.users.Create(context.Background(), "admin", "password1234567890")

		req := httptest.NewRequest("GET", "/api/setup/status", nil)
		rec := httptest.NewRecorder()

		fx.handler.Status(rec, req)

		var resp SetupStatusResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if !resp.SetupRequired {
			t.Error("Expected setup_required to be true")
		}
		if resp.SetupStep != "profile" {
			t.Errorf("Expected setup_step 'profile', got '%s'", resp.SetupStep)
		}
	})

	t.Run("setup complete when user and profile exist", func(t *testing.T) {
		if err := kv.SaveSetupState(context.Background(), fx.db, SetupState{ProfileComplete: true, ProviderComplete: true}); err != nil {
			t.Fatalf("SaveSetupState: %v", err)
		}

		req := httptest.NewRequest("GET", "/api/setup/status", nil)
		rec := httptest.NewRecorder()

		fx.handler.Status(rec, req)

		var resp SetupStatusResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp.SetupRequired {
			t.Error("Expected setup_required to be false")
		}
		if resp.SetupStep != "complete" {
			t.Errorf("Expected setup_step 'complete', got '%s'", resp.SetupStep)
		}
	})
}

func TestSetupHandler_Setup(t *testing.T) {
	t.Run("successful setup", func(t *testing.T) {
		fx := newTestSetupFixture(t)

		body := `{"username": "admin", "password": "mysecurepassword16", "confirm_password": "mysecurepassword16"}`
		req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Setup(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp SetupResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if !resp.Success {
			t.Error("Expected success to be true")
		}

		if !fx.users.HasUsers(context.Background()) {
			t.Error("Expected user to be created")
		}
	})

	t.Run("setup already complete", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "existing", "password1234567890")

		body := `{"username": "admin", "password": "mysecurepassword16", "confirm_password": "mysecurepassword16"}`
		req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Setup(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", rec.Code)
		}
	})

	t.Run("password mismatch", func(t *testing.T) {
		fx := newTestSetupFixture(t)

		body := `{"username": "admin", "password": "mysecurepassword16", "confirm_password": "differentpassword"}`
		req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Setup(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}

		var resp map[string]string
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp["message"] != "Passwords do not match" {
			t.Errorf("Expected 'Passwords do not match', got '%s'", resp["message"])
		}
	})

	t.Run("weak password", func(t *testing.T) {
		fx := newTestSetupFixture(t)

		body := `{"username": "admin", "password": "weak", "confirm_password": "weak"}`
		req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Setup(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		fx := newTestSetupFixture(t)

		body := `{"username": "", "password": "mysecurepassword16", "confirm_password": "mysecurepassword16"}`
		req := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Setup(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})
}

func TestSetupHandler_Profile(t *testing.T) {
	t.Run("successful profile setup", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "admin", "password1234567890")

		body := `{"agent_name": "Atlas", "personality": "friendly", "user_name": "Matt", "timezone": "Europe/London"}`
		req := httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Profile(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp SetupResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if !resp.Success {
			t.Error("Expected success to be true")
		}

		// Verify SOUL.md was written
		soulData, err := os.ReadFile(filepath.Join(fx.aiDataDir, "SOUL.md"))
		if err != nil {
			t.Fatalf("Failed to read SOUL.md: %v", err)
		}
		soulContent := string(soulData)
		if !bytes.Contains(soulData, []byte("Atlas")) {
			t.Errorf("SOUL.md should contain agent name 'Atlas', got: %s", soulContent)
		}
		if !bytes.Contains(soulData, []byte("Warm, conversational")) {
			t.Errorf("SOUL.md should contain personality vibe, got: %s", soulContent)
		}

		// Verify USER.md was written
		userData, err := os.ReadFile(filepath.Join(fx.aiDataDir, "USER.md"))
		if err != nil {
			t.Fatalf("Failed to read USER.md: %v", err)
		}
		userContent := string(userData)
		if !bytes.Contains(userData, []byte("Matt")) {
			t.Errorf("USER.md should contain user name 'Matt', got: %s", userContent)
		}
		if !bytes.Contains(userData, []byte("Europe/London")) {
			t.Errorf("USER.md should contain timezone, got: %s", userContent)
		}

		// Verify setup state was persisted to the DB.
		state, err := kv.LoadSetupState(context.Background(), fx.db)
		if err != nil {
			t.Fatalf("LoadSetupState: %v", err)
		}
		if !state.ProfileComplete {
			t.Error("Expected profile_complete to be true")
		}
	})

	t.Run("requires account first", func(t *testing.T) {
		fx := newTestSetupFixture(t)

		body := `{"agent_name": "Atlas", "personality": "friendly", "user_name": "Matt", "timezone": "UTC"}`
		req := httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Profile(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("already complete", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "admin", "password1234567890")

		if err := kv.SaveSetupState(context.Background(), fx.db, SetupState{ProfileComplete: true}); err != nil {
			t.Fatalf("SaveSetupState: %v", err)
		}

		body := `{"agent_name": "Atlas", "personality": "friendly", "user_name": "Matt", "timezone": "UTC"}`
		req := httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Profile(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", rec.Code)
		}
	})

	t.Run("invalid personality", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "admin", "password1234567890")

		body := `{"agent_name": "Atlas", "personality": "nonexistent", "user_name": "Matt", "timezone": "UTC"}`
		req := httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Profile(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("empty agent name", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "admin", "password1234567890")

		body := `{"agent_name": "", "personality": "friendly", "user_name": "Matt", "timezone": "UTC"}`
		req := httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Profile(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("empty user name", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "admin", "password1234567890")

		body := `{"agent_name": "Atlas", "personality": "friendly", "user_name": "", "timezone": "UTC"}`
		req := httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Profile(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("defaults timezone to UTC", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "admin", "password1234567890")

		body := `{"agent_name": "Atlas", "personality": "calm", "user_name": "Matt", "timezone": ""}`
		req := httptest.NewRequest("POST", "/api/setup/profile", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		fx.handler.Profile(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		userData, _ := os.ReadFile(filepath.Join(fx.aiDataDir, "USER.md"))
		if !bytes.Contains(userData, []byte("UTC")) {
			t.Error("USER.md should default timezone to UTC")
		}
	})

	t.Run("rejects GET method", func(t *testing.T) {
		fx := newTestSetupFixture(t)

		req := httptest.NewRequest("GET", "/api/setup/profile", nil)
		rec := httptest.NewRecorder()

		fx.handler.Profile(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rec.Code)
		}
	})
}

func TestRequireSetupMiddleware(t *testing.T) {
	t.Run("blocks requests when setup required", func(t *testing.T) {
		fx := newTestSetupFixture(t)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		protected := RequireSetupMiddleware(fx.users, fx.db)(handler)

		req := httptest.NewRequest("GET", "/api/scripts", nil)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got %d", rec.Code)
		}
	})

	t.Run("allows setup endpoints when setup required", func(t *testing.T) {
		fx := newTestSetupFixture(t)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		protected := RequireSetupMiddleware(fx.users, fx.db)(handler)

		// /api/setup should be allowed
		req := httptest.NewRequest("POST", "/api/setup", nil)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 for /api/setup, got %d", rec.Code)
		}

		// /api/setup/status should be allowed
		req = httptest.NewRequest("GET", "/api/setup/status", nil)
		rec = httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 for /api/setup/status, got %d", rec.Code)
		}

		// /api/setup/profile should be allowed
		req = httptest.NewRequest("POST", "/api/setup/profile", nil)
		rec = httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 for /api/setup/profile, got %d", rec.Code)
		}
	})

	t.Run("blocks when profile incomplete", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "admin", "password1234567890")

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		protected := RequireSetupMiddleware(fx.users, fx.db)(handler)

		req := httptest.NewRequest("GET", "/api/scripts", nil)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got %d", rec.Code)
		}

		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["setup_step"] != "profile" {
			t.Errorf("Expected setup_step 'profile', got '%v'", resp["setup_step"])
		}
	})

	t.Run("allows requests when setup complete", func(t *testing.T) {
		fx := newTestSetupFixture(t)
		fx.users.Create(context.Background(), "admin", "password1234567890")

		if err := kv.SaveSetupState(context.Background(), fx.db, SetupState{ProfileComplete: true, ProviderComplete: true}); err != nil {
			t.Fatalf("SaveSetupState: %v", err)
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		protected := RequireSetupMiddleware(fx.users, fx.db)(handler)

		req := httptest.NewRequest("GET", "/api/scripts", nil)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}
