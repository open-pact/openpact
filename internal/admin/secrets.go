package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// SecretHandlers handles secret management API endpoints.
type SecretHandlers struct {
	store    *SecretStore
	onChange func() // callback to notify orchestrator of changes
}

// NewSecretHandlers creates a new SecretHandlers.
func NewSecretHandlers(store *SecretStore, onChange func()) *SecretHandlers {
	return &SecretHandlers{store: store, onChange: onChange}
}

type secretListResponse struct {
	Secrets []SecretEntry `json:"secrets"`
}

type createSecretRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type updateSecretRequest struct {
	Value string `json:"value"`
}

// ListSecrets handles GET /api/secrets.
func (h *SecretHandlers) ListSecrets(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.List()
	if err != nil {
		http.Error(w, `{"error":"internal","message":"Failed to list secrets"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(secretListResponse{Secrets: entries})
}

// CreateSecret handles POST /api/secrets.
func (h *SecretHandlers) CreateSecret(w http.ResponseWriter, r *http.Request) {
	var req createSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request","message":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	err := h.store.Create(req.Name, req.Value)
	if err != nil {
		if errors.Is(err, ErrSecretExists) {
			http.Error(w, `{"error":"conflict","message":"Secret already exists"}`, http.StatusConflict)
			return
		}
		if errors.Is(err, ErrInvalidName) {
			http.Error(w, `{"error":"bad_request","message":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrInvalidValue) {
			http.Error(w, `{"error":"bad_request","message":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"internal","message":"Failed to create secret"}`, http.StatusInternalServerError)
		return
	}

	if h.onChange != nil {
		h.onChange()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "name": req.Name})
}

// UpdateSecret handles PUT /api/secrets/:name.
func (h *SecretHandlers) UpdateSecret(w http.ResponseWriter, r *http.Request) {
	name := extractSecretName(r.URL.Path)
	if name == "" {
		http.Error(w, `{"error":"bad_request","message":"Secret name required"}`, http.StatusBadRequest)
		return
	}

	var req updateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request","message":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	err := h.store.Update(name, req.Value)
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			http.Error(w, `{"error":"not_found","message":"Secret not found"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrInvalidValue) {
			http.Error(w, `{"error":"bad_request","message":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"internal","message":"Failed to update secret"}`, http.StatusInternalServerError)
		return
	}

	if h.onChange != nil {
		h.onChange()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated", "name": name})
}

// DeleteSecret handles DELETE /api/secrets/:name.
func (h *SecretHandlers) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	name := extractSecretName(r.URL.Path)
	if name == "" {
		http.Error(w, `{"error":"bad_request","message":"Secret name required"}`, http.StatusBadRequest)
		return
	}

	err := h.store.Delete(name)
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			http.Error(w, `{"error":"not_found","message":"Secret not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal","message":"Failed to delete secret"}`, http.StatusInternalServerError)
		return
	}

	if h.onChange != nil {
		h.onChange()
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractSecretName extracts the secret name from /api/secrets/:name path.
func extractSecretName(path string) string {
	prefix := "/api/secrets/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	name := strings.TrimPrefix(path, prefix)
	// Remove any trailing slashes
	name = strings.TrimRight(name, "/")
	return name
}
