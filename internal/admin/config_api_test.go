package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-pact/openpact/internal/storage"
)

func newHandlers(t *testing.T) *ConfigHandlers {
	t.Helper()
	return NewConfigHandlers(storage.NewTestDB(t))
}

func TestAdvancedSettings_GetDefaults(t *testing.T) {
	h := newHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/config/advanced", nil)
	rec := httptest.NewRecorder()
	h.HandleAdvanced(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got AdvancedSettings
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want info", got.Logging.Level)
	}
	if got.RateLimit.Burst != 20 {
		t.Errorf("RateLimit.Burst = %d, want 20", got.RateLimit.Burst)
	}
	if got.Server.HealthAddr != ":8081" {
		t.Errorf("Server.HealthAddr = %q, want :8081", got.Server.HealthAddr)
	}
	if got.Starlark.MaxMemoryMB != 128 {
		t.Errorf("Starlark.MaxMemoryMB = %d, want 128", got.Starlark.MaxMemoryMB)
	}
}

func TestAdvancedSettings_PutAndGetRoundTrip(t *testing.T) {
	h := newHandlers(t)

	body := `{
		"logging":   {"level": "debug", "json": true},
		"rate_limit": {"rate": 5, "burst": 50},
		"server":    {"health_addr": "127.0.0.1:9000"},
		"starlark":  {"enabled": false, "max_execution_ms": 1000, "max_memory_mb": 64}
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/advanced", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.HandleAdvanced(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Round-trip GET should return what we wrote.
	req = httptest.NewRequest(http.MethodGet, "/api/config/advanced", nil)
	rec = httptest.NewRecorder()
	h.HandleAdvanced(rec, req)
	var got AdvancedSettings
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Logging.Level != "debug" || !got.Logging.JSON {
		t.Errorf("Logging round-trip lost: %+v", got.Logging)
	}
	if got.RateLimit.Rate != 5 || got.RateLimit.Burst != 50 {
		t.Errorf("RateLimit round-trip lost: %+v", got.RateLimit)
	}
	if got.Starlark.Enabled {
		t.Errorf("Starlark.Enabled should be false")
	}
}

func TestAdvancedSettings_BadMethod(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/config/advanced", nil)
	rec := httptest.NewRecorder()
	h.HandleAdvanced(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestAdvancedSettings_BadJSON(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodPut, "/api/config/advanced", bytes.NewBufferString("{not json"))
	rec := httptest.NewRecorder()
	h.HandleAdvanced(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestIntegrations_RoundTrip(t *testing.T) {
	h := newHandlers(t)

	body := `{
		"calendars": [{"name": "Work", "url": "https://example/c.ics"}],
		"vault": {"path": "/home/u/vault", "git_repo": "", "auto_sync": true},
		"github": {"enabled": true}
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/integrations", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.HandleIntegrations(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/config/integrations", nil)
	rec = httptest.NewRecorder()
	h.HandleIntegrations(rec, req)

	var got IntegrationsSettings
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Calendars) != 1 || got.Calendars[0].Name != "Work" {
		t.Errorf("Calendars round-trip lost: %+v", got.Calendars)
	}
	if got.Vault.Path != "/home/u/vault" || !got.Vault.AutoSync {
		t.Errorf("Vault round-trip lost: %+v", got.Vault)
	}
	if !got.GitHub.Enabled {
		t.Errorf("GitHub.Enabled round-trip lost: %+v", got.GitHub)
	}
}

func TestIntegrations_EmptyReturnsEmptyList(t *testing.T) {
	h := newHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/config/integrations", nil)
	rec := httptest.NewRecorder()
	h.HandleIntegrations(rec, req)

	var got IntegrationsSettings
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Calendars == nil {
		t.Error("empty calendars should be an empty array, not nil")
	}
}
