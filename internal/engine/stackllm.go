// Package engine composes stackllm primitives into a single object ("Stack")
// that the orchestrator and admin server share.
//
// There is no Engine interface here any more: stackllm runs in-process, so
// orchestrator code talks to *profile.Manager / session.SessionStore /
// agent.Agent directly. The Stack exposes an http.Handler that the admin
// server mounts under /api/engine/ to drive provider login, model
// selection, SSE chat and session retrieval from the web UI.
//
// The *sql.DB handle is owned by the caller (cmd/openpact's main) and
// shared with every OpenPact storage package. engine.New wraps that DB
// into a session.SQLiteStore via NewSQLiteStore (caller-owned), so the
// session store and op_* tables live side-by-side in the same file.
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
	"github.com/stack-bound/stackllm/agent"
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
	// DB is the shared SQLite handle. Required — the caller is expected
	// to open it (via internal/storage.Open), run OpenPact migrations,
	// and pass it in. Stackllm session tables share this file under the
	// `stackllm_` prefix; OpenPact state is under `op_`.
	DB *sql.DB

	// WorkspacePath is the root of the OpenPact workspace. Stackllm
	// auth/config files live under <WorkspacePath>/secure/data/
	// (stackllm_auth.json, stackllm_config.json) and are file-based by
	// stackllm's own design.
	WorkspacePath string

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

	// DB is the shared SQLite connection. Exposed so storage packages and
	// admin handlers can run their own queries against the same file.
	// Not owned by the Stack — the caller opened it and is responsible
	// for closing it.
	DB *sql.DB

	// systemPrompt is read by the orchestrator when it builds per-turn
	// agents — it's prepended to the session messages as a RoleSystem
	// block so the model sees SOUL/USER/MEMORY context. Not used by the
	// web handler path (the admin chat runs without injected context).
	mu           sync.RWMutex
	systemPrompt string
}

// New constructs a Stack. It requires an already-open *sql.DB (shared
// with the rest of OpenPact via internal/storage.Open). Stackllm's
// auth/config stores under <WorkspacePath>/secure/data/ are still
// file-based and are opened here.
func New(cfg Config) (*Stack, error) {
	if cfg.WorkspacePath == "" {
		return nil, fmt.Errorf("engine: WorkspacePath is required")
	}
	if cfg.DB == nil {
		return nil, fmt.Errorf("engine: DB is required")
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

	// NewSQLiteStore does not own the DB — the caller opened it and
	// controls its lifecycle. Bootstrap still runs (journal_mode=WAL
	// pragma + stackllm_* schema migrations).
	sessionStore, err := session.NewSQLiteStore(cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("engine: attach session store: %w", err)
	}

	registry := tools.NewRegistry()
	if cfg.Tools != nil {
		RegisterMCPTools(registry, cfg.Tools)
	}

	s := &Stack{
		Manager:  mgr,
		Sessions: sessionStore,
		Tools:    registry,
		DB:       cfg.DB,
	}

	// web.ManagedHandler builds a fresh agent per request; agent-level
	// options (tools, max steps) live on the handler and follow every
	// call. Admin-UI chat gets the same tool registry the
	// Discord/Slack/Telegram paths do. Orchestrator paths still prepend
	// their own SOUL/USER/MEMORY system message per turn.
	s.Handler = web.NewManagedHandler(mgr, sessionStore,
		web.WithAgentOptions(
			agent.WithTools(registry),
		),
	)

	return s, nil
}

// Close is a no-op in the caller-owned-DB model: the session store
// wraps the caller's DB and does not own it; the *sql.DB itself is
// closed by whoever called storage.Open. Retained for API stability
// with earlier versions that did own the DB.
func (s *Stack) Close() error {
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
