package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// AdvancedSettings covers the knobs that don't fit cleanly into the other
// admin views — logging, rate limiting, health bind address, Starlark
// limits. Persisted as JSON to secure/data/advanced_settings.json so
// secure/config.yaml can stay minimal.
type AdvancedSettings struct {
	Logging struct {
		Level string `json:"level"` // debug, info, warn, error
		JSON  bool   `json:"json"`  // emit structured JSON logs
	} `json:"logging"`

	RateLimit struct {
		Rate  float64 `json:"rate"`  // requests per second
		Burst int     `json:"burst"` // max burst size
	} `json:"rate_limit"`

	Server struct {
		HealthAddr string `json:"health_addr"` // ":8081" etc.
	} `json:"server"`

	Starlark struct {
		Enabled        bool  `json:"enabled"`
		MaxExecutionMs int64 `json:"max_execution_ms"`
		MaxMemoryMB    int   `json:"max_memory_mb"`
	} `json:"starlark"`
}

// IntegrationsSettings bundles the non-LLM, non-chat integrations: calendar
// feeds and the Obsidian vault. Stored as JSON alongside AdvancedSettings.
type IntegrationsSettings struct {
	Calendars []CalendarFeed `json:"calendars"`
	Vault     VaultFeed      `json:"vault"`
}

// CalendarFeed mirrors config.CalendarConfig but over the wire as JSON.
type CalendarFeed struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// VaultFeed mirrors config.VaultConfig but over the wire as JSON.
type VaultFeed struct {
	Path     string `json:"path"`
	GitRepo  string `json:"git_repo"`
	AutoSync bool   `json:"auto_sync"`
}

// ConfigHandlers serves GET/PUT for /api/config/advanced and
// /api/config/integrations. Uses a file lock so concurrent writers don't
// corrupt the JSON payload.
type ConfigHandlers struct {
	dataDir string
	mu      sync.Mutex
}

// NewConfigHandlers constructs a ConfigHandlers pointing at the
// secure/data/ directory.
func NewConfigHandlers(dataDir string) *ConfigHandlers {
	return &ConfigHandlers{dataDir: dataDir}
}

func (h *ConfigHandlers) advancedPath() string {
	return filepath.Join(h.dataDir, "advanced_settings.json")
}

func (h *ConfigHandlers) integrationsPath() string {
	return filepath.Join(h.dataDir, "integrations.json")
}

// HandleAdvanced routes GET + PUT for /api/config/advanced.
func (h *ConfigHandlers) HandleAdvanced(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.readAdvanced(w, r)
	case http.MethodPut:
		h.writeAdvanced(w, r)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

// HandleIntegrations routes GET + PUT for /api/config/integrations.
func (h *ConfigHandlers) HandleIntegrations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.readIntegrations(w, r)
	case http.MethodPut:
		h.writeIntegrations(w, r)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *ConfigHandlers) readAdvanced(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var s AdvancedSettings
	if err := readJSONFile(h.advancedPath(), &s); err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	// Fill in sensible defaults on first read.
	if s.Logging.Level == "" {
		s.Logging.Level = "info"
	}
	if s.RateLimit.Rate == 0 {
		s.RateLimit.Rate = 10
	}
	if s.RateLimit.Burst == 0 {
		s.RateLimit.Burst = 20
	}
	if s.Server.HealthAddr == "" {
		s.Server.HealthAddr = ":8081"
	}
	if s.Starlark.MaxExecutionMs == 0 {
		s.Starlark.MaxExecutionMs = 30000
	}
	if s.Starlark.MaxMemoryMB == 0 {
		s.Starlark.MaxMemoryMB = 128
	}

	writeOK(w, s)
}

func (h *ConfigHandlers) writeAdvanced(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var s AdvancedSettings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeJSONErr(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := writeJSONFile(h.advancedPath(), s); err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, s)
}

func (h *ConfigHandlers) readIntegrations(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var s IntegrationsSettings
	if err := readJSONFile(h.integrationsPath(), &s); err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	if s.Calendars == nil {
		s.Calendars = []CalendarFeed{}
	}
	writeOK(w, s)
}

func (h *ConfigHandlers) writeIntegrations(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var s IntegrationsSettings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeJSONErr(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := writeJSONFile(h.integrationsPath(), s); err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, s)
}

func readJSONFile(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func writeJSONFile(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONErr(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// writeJSON writes an arbitrary JSON body with the given status (exported
// for other admin handlers that don't have a local writer).
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
