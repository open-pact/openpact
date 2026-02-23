package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/open-pact/openpact/internal/engine"
)

// SessionAPI defines the interface for session management that the admin handlers need.
type SessionAPI interface {
	CreateSession() (*engine.Session, error)
	ListSessions() ([]engine.Session, error)
	GetSession(id string) (*engine.Session, error)
	DeleteSession(id string) error
	GetMessages(sessionID string, limit int) ([]engine.MessageInfo, error)
	Send(ctx context.Context, sessionID string, messages []engine.Message) (<-chan engine.Response, error)
}

// SessionHandlers handles session-related admin API endpoints.
type SessionHandlers struct {
	api SessionAPI
}

// NewSessionHandlers creates a new SessionHandlers.
func NewSessionHandlers(api SessionAPI) *SessionHandlers {
	return &SessionHandlers{api: api}
}

// ListSessions handles GET /api/sessions
func (h *SessionHandlers) ListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	sessions, err := h.api.ListSessions()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, sessions)
}

// CreateSession handles POST /api/sessions
func (h *SessionHandlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	session, err := h.api.CreateSession()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, session)
}

// GetSession handles GET /api/sessions/:id
func (h *SessionHandlers) GetSession(w http.ResponseWriter, r *http.Request, id string) {
	session, err := h.api.GetSession(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	writeJSON(w, http.StatusOK, session)
}

// DeleteSession handles DELETE /api/sessions/:id
func (h *SessionHandlers) DeleteSession(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.api.DeleteSession(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GetMessages handles GET /api/sessions/:id/messages
func (h *SessionHandlers) GetMessages(w http.ResponseWriter, r *http.Request, sessionID string) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	messages, err := h.api.GetMessages(sessionID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, messages)
}

// Chat handles WebSocket chat at /api/sessions/:id/chat
func (h *SessionHandlers) Chat(w http.ResponseWriter, r *http.Request, sessionID string) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex
	sendMsg := func(msg chatMessage) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("WebSocket write error: %v", err)
		}
	}

	// Send connected message
	sendMsg(chatMessage{Type: "connected", SessionID: sessionID})

	for {
		var msg chatMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		if msg.Type != "message" || msg.Content == "" {
			continue
		}

		log.Printf("[admin] Message in session %s: %s", sessionID, msg.Content)

		// Send message to engine
		ctx := context.Background()
		responses, err := h.api.Send(ctx, sessionID, []engine.Message{
			{Role: "user", Content: msg.Content},
		})
		if err != nil {
			sendMsg(chatMessage{Type: "error", Content: fmt.Sprintf("Engine error: %v", err)})
			continue
		}

		// Stream response chunks
		firstContent := true
		for resp := range responses {
			if resp.Thinking != "" {
				if firstContent {
					log.Printf("[admin] AI response started for session %s", sessionID)
					firstContent = false
				}
				sendMsg(chatMessage{Type: "thinking", Content: resp.Thinking})
			}
			if resp.Content != "" {
				if firstContent {
					log.Printf("[admin] AI response started for session %s", sessionID)
					firstContent = false
				}
				sendMsg(chatMessage{Type: "text", Content: resp.Content})
			}
			if resp.Done {
				sendMsg(chatMessage{Type: "done"})
			}
		}
	}
}

// chatMessage is the WebSocket message format for chat.
type chatMessage struct {
	Type      string `json:"type"` // "message", "text", "thinking", "done", "error", "connected"
	Content   string `json:"content,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// HandleSessions routes /api/sessions requests.
func (h *SessionHandlers) HandleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListSessions(w, r)
	case http.MethodPost:
		h.CreateSession(w, r)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

// HandleSessionByID routes /api/sessions/:id requests.
func (h *SessionHandlers) HandleSessionByID(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Extract session ID and action from path
	// /api/sessions/:id/messages, /api/sessions/:id/switch, /api/sessions/:id/chat, /api/sessions/:id
	trimmed := strings.TrimPrefix(path, "/api/sessions/")
	parts := strings.SplitN(trimmed, "/", 2)
	sessionID := parts[0]

	if sessionID == "" {
		http.Error(w, `{"error":"session_id required"}`, http.StatusBadRequest)
		return
	}

	// Check for sub-resource
	if len(parts) == 2 {
		switch parts[1] {
		case "messages":
			if r.Method == http.MethodGet {
				h.GetMessages(w, r, sessionID)
				return
			}
			http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
			return
		case "chat":
			h.Chat(w, r, sessionID)
			return
		}
	}

	// Base session operations
	switch r.Method {
	case http.MethodGet:
		h.GetSession(w, r, sessionID)
	case http.MethodDelete:
		h.DeleteSession(w, r, sessionID)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}
