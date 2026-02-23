package admin

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ScriptHandlers provides HTTP handlers for script management.
type ScriptHandlers struct {
	store *ScriptStore
}

// NewScriptHandlers creates new script handlers.
func NewScriptHandlers(store *ScriptStore) *ScriptHandlers {
	return &ScriptHandlers{store: store}
}

// ListScripts handles GET /api/scripts
func (h *ScriptHandlers) ListScripts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	scripts, err := h.store.List()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "list_failed",
			"message": "Failed to list scripts",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"scripts": scripts,
	})
}

// GetScript handles GET /api/scripts/:name
func (h *ScriptHandlers) GetScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := strings.TrimPrefix(r.URL.Path, "/api/scripts/")
	name = strings.Split(name, "/")[0] // Handle /api/scripts/:name/approve etc

	script, err := h.store.Get(name, true)
	if err == ErrScriptNotFound {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_found",
			"message": "Script not found",
		})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "get_failed",
			"message": "Failed to get script",
		})
		return
	}

	json.NewEncoder(w).Encode(script)
}

// CreateScript handles POST /api/scripts
func (h *ScriptHandlers) CreateScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_request",
			"message": "Invalid request body",
		})
		return
	}

	if req.Name == "" || req.Source == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_request",
			"message": "Name and source are required",
		})
		return
	}

	username, _ := UsernameFromContext(r.Context())

	script, err := h.store.Create(req.Name, req.Source, username)
	if err == ErrScriptExists {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "script_exists",
			"message": "Script already exists",
		})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "create_failed",
			"message": "Failed to create script",
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(script)
}

// UpdateScript handles PUT /api/scripts/:name
func (h *ScriptHandlers) UpdateScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := strings.TrimPrefix(r.URL.Path, "/api/scripts/")

	var req struct {
		Source string `json:"source"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_request",
			"message": "Invalid request body",
		})
		return
	}

	if req.Source == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_request",
			"message": "Source is required",
		})
		return
	}

	username, _ := UsernameFromContext(r.Context())

	script, err := h.store.Update(name, req.Source, username)
	if err == ErrScriptNotFound {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_found",
			"message": "Script not found",
		})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "update_failed",
			"message": "Failed to update script",
		})
		return
	}

	json.NewEncoder(w).Encode(script)
}

// DeleteScript handles DELETE /api/scripts/:name
func (h *ScriptHandlers) DeleteScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := strings.TrimPrefix(r.URL.Path, "/api/scripts/")

	err := h.store.Delete(name)
	if err == ErrScriptNotFound {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_found",
			"message": "Script not found",
		})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "delete_failed",
			"message": "Failed to delete script",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ApproveScript handles POST /api/scripts/:name/approve
func (h *ScriptHandlers) ApproveScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract name from /api/scripts/:name/approve
	path := strings.TrimPrefix(r.URL.Path, "/api/scripts/")
	name := strings.TrimSuffix(path, "/approve")

	username, _ := UsernameFromContext(r.Context())

	script, err := h.store.Approve(name, username)
	if err == ErrScriptNotFound {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_found",
			"message": "Script not found",
		})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "approve_failed",
			"message": "Failed to approve script",
		})
		return
	}

	json.NewEncoder(w).Encode(script)
}

// RejectScript handles POST /api/scripts/:name/reject
func (h *ScriptHandlers) RejectScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract name from /api/scripts/:name/reject
	path := strings.TrimPrefix(r.URL.Path, "/api/scripts/")
	name := strings.TrimSuffix(path, "/reject")

	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req) // Reason is optional

	username, _ := UsernameFromContext(r.Context())

	script, err := h.store.Reject(name, username, req.Reason)
	if err == ErrScriptNotFound {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_found",
			"message": "Script not found",
		})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "reject_failed",
			"message": "Failed to reject script",
		})
		return
	}

	json.NewEncoder(w).Encode(script)
}
