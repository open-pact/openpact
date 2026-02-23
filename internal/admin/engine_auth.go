package admin

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/open-pact/openpact/internal/auth"
)

// EngineAuthHandlers provides HTTP handlers for engine authentication.
type EngineAuthHandlers struct {
	engineType string
	upgrader   websocket.Upgrader
}

// NewEngineAuthHandlers creates a new EngineAuthHandlers.
func NewEngineAuthHandlers(engineType string) *EngineAuthHandlers {
	return &EngineAuthHandlers{
		engineType: engineType,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Admin UI is same-origin or dev proxy
			},
		},
	}
}

// GetStatus handles GET /api/engine/auth — returns current auth status.
func (h *EngineAuthHandlers) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	status := auth.CheckAuth(h.engineType)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Terminal handles GET /api/engine/auth/terminal — WebSocket PTY bridge.
func (h *EngineAuthHandlers) Terminal(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Mutex for WebSocket writes — gorilla/websocket doesn't support concurrent writes.
	var wsMu sync.Mutex
	writeJSON := func(msg wsMessage) error {
		wsMu.Lock()
		defer wsMu.Unlock()
		return conn.WriteJSON(msg)
	}

	// Wait for start message
	var startMsg wsMessage
	if err := conn.ReadJSON(&startMsg); err != nil {
		log.Printf("Failed to read start message: %v", err)
		return
	}

	if startMsg.Type != "start" {
		writeJSON(wsMessage{Type: "error", Data: "expected start message"})
		return
	}

	engineType := startMsg.Engine
	if engineType == "" {
		engineType = h.engineType
	}

	// Validate engine type
	if engineType != "opencode" {
		writeJSON(wsMessage{Type: "error", Data: "invalid engine type"})
		return
	}

	// Send status update
	writeJSON(wsMessage{Type: "status", Status: "running"})

	// Start terminal session with 5-minute timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	session, err := auth.StartTerminalSession(ctx, engineType)
	if err != nil {
		writeJSON(wsMessage{Type: "error", Data: err.Error()})
		return
	}
	defer session.Close()

	// Read from PTY → send to WebSocket
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := session.Read(buf)
			// Process data before checking error (standard io.Reader pattern:
			// n > 0 bytes may be returned alongside a non-nil error).
			if n > 0 {
				msg := wsMessage{Type: "output", Data: string(buf[:n])}
				if writeErr := writeJSON(msg); writeErr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY read error: %v", err)
				}
				return
			}
		}
	}()

	// Read from WebSocket → write to PTY
	go func() {
		for {
			var msg wsMessage
			if err := conn.ReadJSON(&msg); err != nil {
				// WebSocket closed — cancel context to kill the process
				// so session.Wait() unblocks promptly.
				cancel()
				return
			}

			switch msg.Type {
			case "input":
				session.Write([]byte(msg.Data))
			case "resize":
				if msg.Rows > 0 && msg.Cols > 0 {
					session.Resize(msg.Rows, msg.Cols)
				}
			}
		}
	}()

	// Wait for process to exit
	exitCode := 0
	if err := session.Wait(); err != nil {
		exitCode = 1
	}

	// Wait for the read goroutine to drain remaining PTY output.
	// The PTY fd is still open (defer session.Close() hasn't fired yet),
	// so the read goroutine can finish reading buffered data. It will
	// get EIO once the slave side is fully closed, then close done.
	<-done

	writeJSON(wsMessage{Type: "exit", ExitCode: exitCode})
}

// ClearCredentials handles DELETE /api/engine/auth — clears CLI credential files.
func (h *EngineAuthHandlers) ClearCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Remove CLI-native credential files
	if h.engineType == "opencode" {
		if home, err := os.UserHomeDir(); err == nil {
			os.Remove(filepath.Join(home, ".local", "share", "opencode", "auth.json"))
		}
	}

	status := auth.CheckAuth(h.engineType)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleEngineAuth routes /api/engine/auth requests.
func (h *EngineAuthHandlers) HandleEngineAuth(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if strings.HasSuffix(path, "/terminal") {
		h.Terminal(w, r)
		return
	}

	// Base path: GET for status, DELETE for clear
	switch r.Method {
	case http.MethodGet:
		h.GetStatus(w, r)
	case http.MethodDelete:
		h.ClearCredentials(w, r)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

// wsMessage represents a WebSocket message.
type wsMessage struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	Engine   string `json:"engine,omitempty"`
	Status   string `json:"status,omitempty"`
	Rows     uint16 `json:"rows,omitempty"`
	Cols     uint16 `json:"cols,omitempty"`
	ExitCode int    `json:"code,omitempty"`
}
