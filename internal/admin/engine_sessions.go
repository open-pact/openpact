package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/stack-bound/stackllm/session"
)

// sessionLister is the capability the admin server needs to page sessions.
// The stackllm SQLiteStore satisfies it (via session.SessionPaginator); the
// in-memory store used in tests satisfies it too.
type sessionLister interface {
	ListPage(ctx context.Context, opts session.ListOptions) (session.ListResult, error)
}

// EngineSessionsHandler serves GET /api/engine/sessions with pagination.
// It sits alongside the stackllm ManagedHandler (which exposes the
// per-session /sessions/{id} routes) so the admin UI has a real, server-
// backed list instead of mirroring IDs in localStorage.
type EngineSessionsHandler struct {
	store func() session.SessionStore
}

// NewEngineSessionsHandler wires a handler that reads the session store
// lazily via the provided accessor. The accessor pattern is used because
// the stack is built by the orchestrator after the admin server is
// constructed — by the time a request lands, the accessor returns the live
// store. It returns nil before wiring, and the handler reports 503 in that
// case so the UI gets a clean error instead of a panic.
func NewEngineSessionsHandler(store func() session.SessionStore) *EngineSessionsHandler {
	return &EngineSessionsHandler{store: store}
}

// sessionSummary is the JSON shape returned in the list. It's deliberately
// narrow: no Messages (stackllm's List doesn't hydrate them), no State map
// (internal), no Blocks. Just what the UI needs to render a row.
type sessionSummary struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Model     string     `json:"model,omitempty"`
	Updated   time.Time  `json:"updated"`
	Created   time.Time  `json:"created"`
	LastUsage *tokenInfo `json:"last_usage,omitempty"`
}

type tokenInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type sessionsPage struct {
	Sessions []sessionSummary `json:"sessions"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
}

// List serves GET /api/engine/sessions?limit=&offset=. It returns a paged
// slice plus the total count so the UI can render "page X of Y" without a
// second round-trip.
func (h *EngineSessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	store := h.store()
	if store == nil {
		http.Error(w, `{"error":"engine_not_ready","message":"session store not wired yet"}`, http.StatusServiceUnavailable)
		return
	}

	pager, ok := store.(sessionLister)
	if !ok {
		// InMemoryStore and SQLiteStore both implement SessionPaginator in
		// stackllm v0.4.1+. A store that doesn't is a bug we want surfaced.
		http.Error(w, `{"error":"not_supported","message":"session store does not support pagination"}`, http.StatusNotImplemented)
		return
	}

	limit := parsePositiveInt(r.URL.Query().Get("limit"), session.DefaultListLimit)
	offset := parsePositiveInt(r.URL.Query().Get("offset"), 0)

	// Cap page size so a malicious or buggy client can't ask the store for
	// tens of thousands of rows in one query.
	const maxPageSize = 200
	if limit > maxPageSize {
		limit = maxPageSize
	}

	result, err := pager.ListPage(r.Context(), session.ListOptions{Limit: limit, Offset: offset})
	if err != nil {
		http.Error(w, `{"error":"list_failed","message":"`+jsonEscape(err.Error())+`"}`, http.StatusInternalServerError)
		return
	}

	payload := sessionsPage{
		Sessions: make([]sessionSummary, 0, len(result.Sessions)),
		Total:    result.Total,
		Limit:    limit,
		Offset:   offset,
	}
	for _, s := range result.Sessions {
		payload.Sessions = append(payload.Sessions, toSummary(s))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(payload)
}

func toSummary(s *session.Session) sessionSummary {
	out := sessionSummary{
		ID:      s.ID,
		Name:    s.Name,
		Model:   s.Model,
		Updated: s.Updated,
		Created: s.Created,
	}
	if s.LastUsage != nil {
		out.LastUsage = &tokenInfo{
			PromptTokens:     s.LastUsage.PromptTokens,
			CompletionTokens: s.LastUsage.CompletionTokens,
			TotalTokens:      s.LastUsage.TotalTokens,
		}
	}
	return out
}

// parsePositiveInt returns the parsed value, clamped to >= 0. A missing or
// unparseable query parameter falls back to fallback.
func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

// jsonEscape escapes a string for inclusion in a manually-composed JSON
// error body. Only used on the error path where we want to avoid pulling a
// full error-wrapping struct just to surface a message.
func jsonEscape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(b[1 : len(b)-1])
}

