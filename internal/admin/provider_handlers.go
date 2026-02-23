package admin

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ProviderManagerAPI is the interface the orchestrator implements for provider lifecycle management.
type ProviderManagerAPI interface {
	StartProvider(name string) error
	StopProvider(name string) error
	RestartProvider(name string) error
	GetProviderStatus(name string) (ProviderStatusInfo, error)
	ListProviderStatuses() map[string]ProviderStatusInfo
}

// ProviderStatusInfo mirrors the orchestrator type for use in admin handlers.
type ProviderStatusInfo struct {
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

// ProviderHandlers handles HTTP requests for provider management.
type ProviderHandlers struct {
	store   *ProviderStore
	manager ProviderManagerAPI
}

// NewProviderHandlers creates new provider handlers.
func NewProviderHandlers(store *ProviderStore) *ProviderHandlers {
	return &ProviderHandlers{store: store}
}

// SetManager sets the provider manager (called after orchestrator is created).
func (h *ProviderHandlers) SetManager(manager ProviderManagerAPI) {
	h.manager = manager
}

// providerResponse is the API response for a single provider.
type providerResponse struct {
	Name         string                       `json:"name"`
	Enabled      bool                         `json:"enabled"`
	AllowedUsers []string                     `json:"allowed_users"`
	AllowedChans []string                     `json:"allowed_chans"`
	Status       *ProviderStatusInfo          `json:"status,omitempty"`
	Tokens       map[string]ProviderTokenInfo `json:"tokens"`
}

var allProviderNames = []string{"discord", "telegram", "slack"}

// ListProviders handles GET /api/providers.
func (h *ProviderHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var statuses map[string]ProviderStatusInfo
	if h.manager != nil {
		statuses = h.manager.ListProviderStatuses()
	}

	providers := make([]providerResponse, 0, 3)
	for _, name := range allProviderNames {
		providers = append(providers, h.buildProviderResponse(name, statuses))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"providers": providers})
}

// HandleProviderByName handles /api/providers/:name.
func (h *ProviderHandlers) HandleProviderByName(w http.ResponseWriter, r *http.Request) {
	name := extractProviderName(r.URL.Path)
	if !validProviderNames[name] {
		http.Error(w, `{"error":"invalid provider name"}`, http.StatusBadRequest)
		return
	}

	// Check for action sub-paths
	suffix := strings.TrimPrefix(r.URL.Path, "/api/providers/"+name)
	switch suffix {
	case "/tokens":
		if r.Method == http.MethodPut {
			h.SetTokens(w, r, name)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	case "/start":
		if r.Method == http.MethodPost {
			h.StartProvider(w, r, name)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	case "/stop":
		if r.Method == http.MethodPost {
			h.StopProvider(w, r, name)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	case "/restart":
		if r.Method == http.MethodPost {
			h.RestartProvider(w, r, name)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	case "":
		// Fall through to standard CRUD
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.GetProvider(w, r, name)
	case http.MethodPut:
		h.UpdateProvider(w, r, name)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

// GetProvider handles GET /api/providers/:name.
func (h *ProviderHandlers) GetProvider(w http.ResponseWriter, r *http.Request, name string) {
	var statuses map[string]ProviderStatusInfo
	if h.manager != nil {
		statuses = h.manager.ListProviderStatuses()
	}
	writeJSON(w, http.StatusOK, h.buildProviderResponse(name, statuses))
}

// UpdateProvider handles PUT /api/providers/:name.
func (h *ProviderHandlers) UpdateProvider(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Enabled      *bool    `json:"enabled"`
		AllowedUsers []string `json:"allowed_users"`
		AllowedChans []string `json:"allowed_chans"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Get existing config or create default
	cfg, err := h.store.Get(name)
	if err == ErrProviderNotFound {
		cfg = ProviderConfig{Name: name}
	} else if err != nil {
		http.Error(w, `{"error":"failed to load config"}`, http.StatusInternalServerError)
		return
	}

	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.AllowedUsers != nil {
		cfg.AllowedUsers = req.AllowedUsers
	}
	if req.AllowedChans != nil {
		cfg.AllowedChans = req.AllowedChans
	}

	// Preserve existing tokens — Set() with nil Tokens preserves them
	cfg.Tokens = nil

	if err := h.store.Set(name, cfg); err != nil {
		http.Error(w, `{"error":"failed to save config"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SetTokens handles PUT /api/providers/:name/tokens.
func (h *ProviderHandlers) SetTokens(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Tokens map[string]string `json:"tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if len(req.Tokens) == 0 {
		http.Error(w, `{"error":"tokens required"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.SetTokens(name, req.Tokens); err != nil {
		http.Error(w, `{"error":"failed to save tokens"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// StartProvider handles POST /api/providers/:name/start.
func (h *ProviderHandlers) StartProvider(w http.ResponseWriter, r *http.Request, name string) {
	if h.manager == nil {
		http.Error(w, `{"error":"provider manager not available"}`, http.StatusServiceUnavailable)
		return
	}

	if err := h.manager.StartProvider(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// StopProvider handles POST /api/providers/:name/stop.
func (h *ProviderHandlers) StopProvider(w http.ResponseWriter, r *http.Request, name string) {
	if h.manager == nil {
		http.Error(w, `{"error":"provider manager not available"}`, http.StatusServiceUnavailable)
		return
	}

	if err := h.manager.StopProvider(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RestartProvider handles POST /api/providers/:name/restart.
func (h *ProviderHandlers) RestartProvider(w http.ResponseWriter, r *http.Request, name string) {
	if h.manager == nil {
		http.Error(w, `{"error":"provider manager not available"}`, http.StatusServiceUnavailable)
		return
	}

	if err := h.manager.RestartProvider(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ProviderHandlers) buildProviderResponse(name string, statuses map[string]ProviderStatusInfo) providerResponse {
	resp := providerResponse{
		Name:         name,
		AllowedUsers: []string{},
		AllowedChans: []string{},
		Tokens:       make(map[string]ProviderTokenInfo),
	}

	cfg, err := h.store.Get(name)
	if err == nil {
		resp.Enabled = cfg.Enabled
		if cfg.AllowedUsers != nil {
			resp.AllowedUsers = cfg.AllowedUsers
		}
		if cfg.AllowedChans != nil {
			resp.AllowedChans = cfg.AllowedChans
		}
	}

	// Add token info for each required key
	for _, key := range RequiredTokenKeys(name) {
		resp.Tokens[key] = h.store.TokenInfo(name, key)
	}

	// Add runtime status if available
	if statuses != nil {
		if s, ok := statuses[name]; ok {
			resp.Status = &s
		}
	}

	return resp
}

func extractProviderName(path string) string {
	// /api/providers/discord/tokens -> discord
	path = strings.TrimPrefix(path, "/api/providers/")
	if idx := strings.Index(path, "/"); idx != -1 {
		return path[:idx]
	}
	return path
}

// writeJSON is defined in session_ai.go
