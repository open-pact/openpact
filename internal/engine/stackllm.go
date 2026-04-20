// Package engine composes stackllm primitives into a single object ("Stack")
// that the orchestrator and admin server share.
//
// There is no Engine interface here any more: stackllm runs in-process, so
// orchestrator code talks to *profile.Manager / session.SessionStore /
// agent.Agent directly. The Stack exposes an http.Handler that the admin
// server mounts under /api/engine/ to drive provider login, model
// selection, SSE chat and session retrieval from the web UI.
package engine

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/open-pact/openpact/internal/mcp"
	"github.com/stack-bound/stackllm/auth"
	"github.com/stack-bound/stackllm/config"
	"github.com/stack-bound/stackllm/profile"
	"github.com/stack-bound/stackllm/session"
	"github.com/stack-bound/stackllm/tools"
	"github.com/stack-bound/stackllm/web"
)

// Config holds the settings the Stack needs at construction time.
// Everything else (which providers are authenticated, which model is the
// default) is persisted inside stackllm's auth/config stores and is
// mutated through the web handler at runtime.
type Config struct {
	// WorkspacePath is the root of the OpenPact workspace. Auth, config
	// and the SQLite database are written under <WorkspacePath>/secure/data/.
	WorkspacePath string

	// DBPath overrides the default SQLite path. Empty means use
	// <WorkspacePath>/secure/data/stackllm.db.
	DBPath string

	// Tools is the MCP server whose registered tools should be exposed to
	// the agent. Required for a functioning stack — passing nil builds a
	// Stack with no tools at all, which is only useful in tests.
	Tools *mcp.Server
}

// Stack owns the shared stackllm instances used by the orchestrator and the
// admin web UI. All fields are safe for concurrent use.
type Stack struct {
	// Manager drives provider login/logout, model listing and default
	// persistence. Shared with the web handler.
	Manager *profile.Manager

	// Sessions persists conversation history to SQLite. Shared with both
	// the orchestrator (for channel-scoped sessions) and the web handler
	// (for admin-UI chat).
	Sessions session.SessionStore

	// Tools holds the native Go tool registry the agent invokes. Built
	// from the MCP server's registered tools by an adapter (see tools.go).
	Tools *tools.Registry

	// Handler is the stackllm web.ManagedHandler. The admin server mounts
	// it under /api/engine/ so the browser can drive login, model
	// selection and SSE chat without any custom backend wiring.
	Handler http.Handler

	// systemPrompt is read by the orchestrator when it builds per-turn
	// agents — it's prepended to the session messages as a RoleSystem
	// block so the model sees SOUL/USER/MEMORY context. Not used by the
	// web handler path (the admin chat runs without injected context).
	mu           sync.RWMutex
	systemPrompt string

	// sqlDB is retained so Close can release the underlying SQLite
	// handle. Not exported — callers should call Close(), not reach in.
	sqlDB *sql.DB
}

// New constructs a Stack. It creates the data directory if needed, opens
// the SQLite session store, builds a tool registry from the MCP server,
// and wires the web.ManagedHandler.
func New(cfg Config) (*Stack, error) {
	if cfg.WorkspacePath == "" {
		return nil, fmt.Errorf("engine: WorkspacePath is required")
	}

	dataDir := filepath.Join(cfg.WorkspacePath, "secure", "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("engine: create data dir: %w", err)
	}

	authStore := &auth.FileStore{Path: filepath.Join(dataDir, "stackllm_auth.json")}
	configStore := &config.Store{Path: filepath.Join(dataDir, "stackllm_config.json")}

	mgr := profile.New(
		profile.WithAuthStore(authStore),
		profile.WithConfigStore(configStore),
	)

	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "stackllm.db")
	}
	sessionStore, err := session.OpenSQLiteStore(session.SQLiteConfig{Path: dbPath})
	if err != nil {
		return nil, fmt.Errorf("engine: open session store: %w", err)
	}

	registry := tools.NewRegistry()
	if cfg.Tools != nil {
		RegisterMCPTools(registry, cfg.Tools)
	}

	s := &Stack{
		Manager:  mgr,
		Sessions: sessionStore,
		Tools:    registry,
	}

	// web.ManagedHandler builds a fresh agent per request, so agent-level
	// options (tools, max steps) live on the handler and follow every
	// call. It does not carry system prompts — orchestrator paths
	// prepend their own system message when building agents for bot
	// turns (see orchestrator.handleChatMessage).
	s.Handler = web.NewManagedHandler(mgr, sessionStore,
		web.WithAgentOptions(
			// A more generous default than stackllm's 20 — tool-heavy
			// conversations (fetch → parse → write → follow-up) run out
			// of steps quickly on 20.
			// agent.WithMaxSteps(40),
		),
	)

	return s, nil
}

// Close releases the underlying SQLite handle if the store owns one.
// Safe to call multiple times.
func (s *Stack) Close() error {
	if s == nil {
		return nil
	}
	if sqliteStore, ok := s.Sessions.(interface{ Close() error }); ok {
		return sqliteStore.Close()
	}
	if s.sqlDB != nil {
		return s.sqlDB.Close()
	}
	return nil
}

// SystemPrompt returns the orchestrator-set system prompt (SOUL/USER/MEMORY
// context). Empty when none has been set yet. Safe to call concurrently.
func (s *Stack) SystemPrompt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.systemPrompt
}

// SetSystemPrompt updates the system prompt the orchestrator prepends to
// bot-turn conversations. Called by the context loader whenever a memory
// file changes. Safe to call concurrently.
func (s *Stack) SetSystemPrompt(prompt string) {
	s.mu.Lock()
	s.systemPrompt = prompt
	s.mu.Unlock()
}

// DefaultModel reports the currently-persisted default model (if any).
// It is a thin wrapper around profile.Manager.Default for callers that
// don't want to plumb a context through every layer.
func (s *Stack) DefaultModel(ctx context.Context) (profile.ModelInfo, bool, error) {
	return s.Manager.Default(ctx)
}
