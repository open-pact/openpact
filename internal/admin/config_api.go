package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/open-pact/openpact/internal/storage/calendars"
	"github.com/open-pact/openpact/internal/storage/kv"
)

// AdvancedSettings is re-exported from the kv package so the Go wire
// type of GET/PUT /api/config/advanced stays in the admin namespace
// the frontend and tests already consume. All type details live in
// internal/storage/kv/types.go.
type AdvancedSettings = kv.AdvancedSettings

// IntegrationsSettings is the wire shape for /api/config/integrations.
// It composes scalars (vault, github) from op_kv with the list from
// op_calendars. That split is the intentional schema separation — a
// list doesn't belong in a scoped key-value store.
type IntegrationsSettings struct {
	Calendars []CalendarFeed    `json:"calendars"`
	Vault     kv.VaultSettings  `json:"vault"`
	GitHub    kv.GitHubSettings `json:"github"`
}

// CalendarFeed is the wire type for a calendar feed. Backed by the
// calendars package; this alias keeps JSON stable for the admin UI.
type CalendarFeed = calendars.Feed

// ConfigHandlers serves GET/PUT for /api/config/advanced and
// /api/config/integrations against DB-backed stores. The handlers no
// longer hold a mutex of their own — SQLite's busy_timeout and
// transactional writes make the ReplaceScope / Replace calls safe
// under concurrent writers.
type ConfigHandlers struct {
	db        *sql.DB
	calendars *calendars.Store
}

// NewConfigHandlers constructs a ConfigHandlers over the shared DB.
// The calendars store is eagerly built; every other read/write goes
// through package-level helpers on the kv package.
func NewConfigHandlers(db *sql.DB) *ConfigHandlers {
	return &ConfigHandlers{
		db:        db,
		calendars: calendars.NewStore(db),
	}
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
	s, err := kv.LoadAdvancedSettings(r.Context(), h.db)
	if err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, s)
}

func (h *ConfigHandlers) writeAdvanced(w http.ResponseWriter, r *http.Request) {
	var s AdvancedSettings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeJSONErr(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := kv.SaveAdvancedSettings(r.Context(), h.db, s); err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, s)
}

func (h *ConfigHandlers) readIntegrations(w http.ResponseWriter, r *http.Request) {
	scalars, err := kv.LoadIntegrationScalars(r.Context(), h.db)
	if err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	feeds, err := h.calendars.List(r.Context())
	if err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, IntegrationsSettings{
		Calendars: feeds,
		Vault:     scalars.Vault,
		GitHub:    scalars.GitHub,
	})
}

func (h *ConfigHandlers) writeIntegrations(w http.ResponseWriter, r *http.Request) {
	var s IntegrationsSettings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeJSONErr(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	scalars := kv.IntegrationScalars{Vault: s.Vault, GitHub: s.GitHub}
	if err := kv.SaveIntegrationScalars(r.Context(), h.db, scalars); err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.calendars.Replace(r.Context(), s.Calendars); err != nil {
		writeJSONErr(w, http.StatusInternalServerError, err)
		return
	}
	// GET shape parity: re-read the calendars so the response matches
	// the canonical list order (not the caller-supplied order, which
	// is usually the same but not guaranteed).
	feeds, _ := h.calendars.List(r.Context())
	writeOK(w, IntegrationsSettings{Calendars: feeds, Vault: s.Vault, GitHub: s.GitHub})
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
