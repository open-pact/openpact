package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestSecretHandlers(t *testing.T) (*SecretHandlers, *SecretStore) {
	t.Helper()
	store := NewSecretStore(t.TempDir())
	handlers := NewSecretHandlers(store, nil)
	return handlers, store
}

func TestListSecrets_Empty(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/secrets", nil)
	w := httptest.NewRecorder()

	handlers.ListSecrets(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp secretListResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Secrets) != 0 {
		t.Errorf("Expected empty list, got %d", len(resp.Secrets))
	}
}

func TestListSecrets_WithSecrets(t *testing.T) {
	handlers, store := newTestSecretHandlers(t)

	store.Set("ALPHA_SECRET", "alpha_value")
	store.Set("BETA_SECRET", "beta_value")

	req := httptest.NewRequest(http.MethodGet, "/api/secrets", nil)
	w := httptest.NewRecorder()

	handlers.ListSecrets(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp secretListResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Secrets) != 2 {
		t.Fatalf("Expected 2 secrets, got %d", len(resp.Secrets))
	}

	// Should be sorted
	if resp.Secrets[0].Name != "ALPHA_SECRET" {
		t.Errorf("Expected first ALPHA_SECRET, got %s", resp.Secrets[0].Name)
	}
	if resp.Secrets[1].Name != "BETA_SECRET" {
		t.Errorf("Expected second BETA_SECRET, got %s", resp.Secrets[1].Name)
	}
}

func TestListSecrets_NeverReturnsValues(t *testing.T) {
	handlers, store := newTestSecretHandlers(t)

	store.Set("MY_SECRET", "super_secret_value_12345")
	store.Set("OTHER_SECRET", "another_secret_67890")

	req := httptest.NewRequest(http.MethodGet, "/api/secrets", nil)
	w := httptest.NewRecorder()

	handlers.ListSecrets(w, req)

	body := w.Body.String()

	if strings.Contains(body, "super_secret_value_12345") {
		t.Error("Response body contains secret value 'super_secret_value_12345'")
	}
	if strings.Contains(body, "another_secret_67890") {
		t.Error("Response body contains secret value 'another_secret_67890'")
	}
}

func TestCreateSecret_Success(t *testing.T) {
	handlers, store := newTestSecretHandlers(t)

	body := `{"name":"NEW_KEY","value":"new_value"}`
	req := httptest.NewRequest(http.MethodPost, "/api/secrets", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.CreateSecret(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify it was stored
	val, err := store.Get("NEW_KEY")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "new_value" {
		t.Errorf("Expected 'new_value', got '%s'", val)
	}
}

func TestCreateSecret_InvalidName(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	tests := []struct {
		name string
		body string
	}{
		{"lowercase", `{"name":"lowercase","value":"val"}`},
		{"empty", `{"name":"","value":"val"}`},
		{"starts with number", `{"name":"123ABC","value":"val"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/secrets", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			handlers.CreateSecret(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateSecret_EmptyValue(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	body := `{"name":"MY_KEY","value":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/secrets", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.CreateSecret(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestCreateSecret_Duplicate(t *testing.T) {
	handlers, store := newTestSecretHandlers(t)

	store.Create("EXISTING_KEY", "existing_value")

	body := `{"name":"EXISTING_KEY","value":"new_value"}`
	req := httptest.NewRequest(http.MethodPost, "/api/secrets", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.CreateSecret(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSecret_InvalidJSON(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/secrets", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	handlers.CreateSecret(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestUpdateSecret_Success(t *testing.T) {
	handlers, store := newTestSecretHandlers(t)

	store.Create("UPDATE_ME", "original")

	body := `{"value":"updated_value"}`
	req := httptest.NewRequest(http.MethodPut, "/api/secrets/UPDATE_ME", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.UpdateSecret(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	val, _ := store.Get("UPDATE_ME")
	if val != "updated_value" {
		t.Errorf("Expected 'updated_value', got '%s'", val)
	}
}

func TestUpdateSecret_NotFound(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	body := `{"value":"new_value"}`
	req := httptest.NewRequest(http.MethodPut, "/api/secrets/NONEXISTENT", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.UpdateSecret(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

func TestUpdateSecret_EmptyValue(t *testing.T) {
	handlers, store := newTestSecretHandlers(t)

	store.Create("MY_KEY", "original")

	body := `{"value":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/secrets/MY_KEY", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.UpdateSecret(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestUpdateSecret_InvalidJSON(t *testing.T) {
	handlers, store := newTestSecretHandlers(t)

	store.Create("MY_KEY", "original")

	req := httptest.NewRequest(http.MethodPut, "/api/secrets/MY_KEY", bytes.NewBufferString("bad json"))
	w := httptest.NewRecorder()

	handlers.UpdateSecret(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestDeleteSecret_Success(t *testing.T) {
	handlers, store := newTestSecretHandlers(t)

	store.Create("DELETE_ME", "value")

	req := httptest.NewRequest(http.MethodDelete, "/api/secrets/DELETE_ME", nil)
	w := httptest.NewRecorder()

	handlers.DeleteSecret(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", w.Code)
	}

	// Verify deleted
	_, err := store.Get("DELETE_ME")
	if err != ErrSecretNotFound {
		t.Error("Expected secret to be deleted")
	}
}

func TestDeleteSecret_NotFound(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/secrets/NONEXISTENT", nil)
	w := httptest.NewRecorder()

	handlers.DeleteSecret(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

func TestSecretHandlers_OnChangeCallback(t *testing.T) {
	store := NewSecretStore(t.TempDir())
	callCount := 0
	handlers := NewSecretHandlers(store, func() {
		callCount++
	})

	// Create triggers onChange
	body := `{"name":"KEY_ONE","value":"val1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/secrets", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handlers.CreateSecret(w, req)

	if callCount != 1 {
		t.Errorf("Expected 1 onChange call after create, got %d", callCount)
	}

	// Update triggers onChange
	body = `{"value":"val2"}`
	req = httptest.NewRequest(http.MethodPut, "/api/secrets/KEY_ONE", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	handlers.UpdateSecret(w, req)

	if callCount != 2 {
		t.Errorf("Expected 2 onChange calls after update, got %d", callCount)
	}

	// Delete triggers onChange
	req = httptest.NewRequest(http.MethodDelete, "/api/secrets/KEY_ONE", nil)
	w = httptest.NewRecorder()
	handlers.DeleteSecret(w, req)

	if callCount != 3 {
		t.Errorf("Expected 3 onChange calls after delete, got %d", callCount)
	}
}

func TestHandleSecrets_WrongMethod(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	// PUT on /api/secrets (collection endpoint) - test via CreateSecret with wrong intent
	req := httptest.NewRequest(http.MethodPut, "/api/secrets", nil)
	w := httptest.NewRecorder()

	// This would be handled by the router method switch, not the handler directly
	// We test the router-level method dispatch in router_test
	_ = handlers
	_ = w
	_ = req
}

func TestExtractSecretName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/secrets/MY_KEY", "MY_KEY"},
		{"/api/secrets/FOO_BAR_123", "FOO_BAR_123"},
		{"/api/secrets/", ""},
		{"/api/secrets", ""},
		{"/api/other/MY_KEY", ""},
		{"/api/secrets/KEY/", "KEY"},
	}

	for _, tt := range tests {
		got := extractSecretName(tt.path)
		if got != tt.want {
			t.Errorf("extractSecretName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestUpdateSecret_EmptyName(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	body := `{"value":"val"}`
	req := httptest.NewRequest(http.MethodPut, "/api/secrets/", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.UpdateSecret(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestDeleteSecret_EmptyName(t *testing.T) {
	handlers, _ := newTestSecretHandlers(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/secrets/", nil)
	w := httptest.NewRecorder()

	handlers.DeleteSecret(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}
