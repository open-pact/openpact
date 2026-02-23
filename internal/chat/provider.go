// Package chat defines the generic chat provider interface for multi-platform messaging.
package chat

// MessageHandler is called when a chat message is received from a user.
// The provider name is included so the orchestrator knows the source.
type MessageHandler func(provider, channelID, userID, content string) (response string, err error)

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
