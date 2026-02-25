// Package chat defines the generic chat provider interface for multi-platform messaging.
package chat

// Detail mode constants control what gets included in chat responses.
const (
	ModeSimple   = "simple"   // Text only (default)
	ModeThinking = "thinking" // Text + thinking blocks
	ModeTools    = "tools"    // Text + tool call blocks
	ModeFull     = "full"     // Text + thinking + tool calls
)

// ValidMode returns true if the given mode string is a recognized detail mode.
func ValidMode(mode string) bool {
	switch mode {
	case ModeSimple, ModeThinking, ModeTools, ModeFull:
		return true
	}
	return false
}

// ToolCallInfo holds information about a single tool call from the AI.
type ToolCallInfo struct {
	Name   string // Tool name (e.g. "workspace_read")
	Input  string // Tool input (JSON string or summary)
	Output string // Tool output/result
}

// ChatResponse is the structured response from the AI engine,
// containing text and optionally thinking blocks and tool call details.
type ChatResponse struct {
	Text      string         // Plain text response (always present)
	Thinking  string         // Thinking/reasoning content (if collected)
	ToolCalls []ToolCallInfo // Tool calls made during the response
}

// MessageHandler is called when a chat message is received from a user.
// The provider name is included so the orchestrator knows the source.
type MessageHandler func(provider, channelID, userID, content string) (response *ChatResponse, err error)

// CommandHandler is called when a slash/bot command is received.
type CommandHandler func(provider, channelID, userID, command, args string) (response string, err error)

// Provider defines a chat platform integration.
// Each provider (Discord, Telegram, Slack) implements this interface.
type Provider interface {
	// Name returns the provider identifier (e.g., "discord", "telegram", "slack").
	Name() string

	// Start connects to the chat platform and begins listening for messages.
	Start() error

	// Stop gracefully disconnects from the chat platform.
	Stop() error

	// SetMessageHandler registers the callback for incoming user messages.
	SetMessageHandler(h MessageHandler)

	// SetCommandHandler registers the callback for incoming commands.
	SetCommandHandler(h CommandHandler)

	// SendMessage sends a message to a target (channel ID, user ID, etc.).
	// Target format is provider-specific but should support "user:<id>" prefix for DMs.
	SendMessage(target, content string) error
}
