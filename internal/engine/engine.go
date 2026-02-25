// Package engine provides an abstraction layer for AI coding agents.
// Supports OpenCode as the backend.
package engine

import (
	"context"
	"encoding/json"
)

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"` // Message text
}

// ToolCall represents a tool/function call from the AI
type ToolCall struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments"`
}

// Response represents an AI response
type Response struct {
	Content   string              `json:"content"`              // Text response
	Thinking  string              `json:"thinking,omitempty"`   // Thinking/reasoning content
	ToolCalls []ToolCall          `json:"tool_calls"`           // Tool calls to execute
	Parts     []json.RawMessage   `json:"parts,omitempty"`      // Raw non-text/thinking parts (tool, file, snapshot)
	Done      bool                `json:"done"`                 // Whether conversation turn is complete
	SessionID string              `json:"session_id,omitempty"` // Session that generated this response
	// Streaming fields (only set during SSE streaming)
	PartID   string `json:"part_id,omitempty"`   // Stable part ID for in-place updates
	PartType string `json:"part_type,omitempty"` // Part type (reasoning, text, tool, etc.)
	IsUpdate bool   `json:"is_update,omitempty"` // True if this updates a previously-sent part
}

// Session represents an opencode session
type Session struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	ProjectID string `json:"projectID"`
	Directory string `json:"directory"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	Time      struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// MessageInfo represents a message from the opencode API
type MessageInfo struct {
	ID        string        `json:"id"`
	SessionID string        `json:"sessionID"`
	Role      string        `json:"role"`
	Parts     []json.RawMessage `json:"parts"`
	Time      struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// ContextUsage holds token usage and context window information for a session
type ContextUsage struct {
	Model          string  // Model identifier (e.g. "claude-sonnet-4-20250514")
	MessageCount   int     // Number of assistant messages
	CurrentContext int     // Current context size (input tokens from last assistant message)
	TotalOutput    int     // Sum of output tokens across all assistant messages
	TotalReasoning int     // Sum of reasoning tokens across all assistant messages
	CacheRead      int     // Sum of cache read tokens
	CacheWrite     int     // Sum of cache write tokens
	TotalCost      float64 // Sum of cost across all assistant messages
	ContextLimit   int     // Model's context window limit (0 if unknown)
	OutputLimit    int     // Model's output limit (0 if unknown)
}

// ModelInfo describes an available model from a provider.
type ModelInfo struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
	Context    int    `json:"context_limit"`
	Output     int    `json:"output_limit"`
}

// Engine is the interface for AI coding agents
type Engine interface {
	// Start initializes the engine (starts opencode serve)
	Start(ctx context.Context) error

	// Stop gracefully shuts down the engine
	Stop() error

	// Send sends a message to a session and returns the response stream
	Send(ctx context.Context, sessionID string, messages []Message) (<-chan Response, error)

	// SetSystemPrompt sets the system prompt for context injection
	SetSystemPrompt(prompt string)

	// Session management
	CreateSession() (*Session, error)
	ListSessions() ([]Session, error)
	GetSession(id string) (*Session, error)
	DeleteSession(id string) error
	AbortSession(id string) error
	GetMessages(sessionID string, limit int) ([]MessageInfo, error)
	GetContextUsage(sessionID string) (*ContextUsage, error)

	// Model management
	ListModels() ([]ModelInfo, error)
	GetDefaultModel() (provider, model string)
	SetDefaultModel(provider, model string)
}

// Config holds engine configuration
type Config struct {
	Type     string // "opencode"
	Provider string // For OpenCode: provider name
	Model    string // Model to use
	WorkDir  string // Working directory (workspace path, used for MCP config)
	Port     int    // Port for opencode serve (0 = use DefaultPort)
	Hostname string // Hostname for opencode serve (default: 127.0.0.1)
	Password string // Optional OPENCODE_SERVER_PASSWORD
}

// New creates a new engine based on config
func New(cfg Config) (Engine, error) {
	return NewOpenCode(cfg)
}
