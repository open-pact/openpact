// Package discord provides Discord bot integration for OpenPact.
package discord

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/open-pact/openpact/internal/chat"
)

var _ chat.Provider = (*Bot)(nil)

// Bot represents a Discord bot
type Bot struct {
	session        *discordgo.Session
	handler        chat.MessageHandler
	commandHandler chat.CommandHandler
	allowedUsers   map[string]bool // User IDs allowed to DM
	allowedChans   map[string]bool // Channel IDs allowed
	botUserID string
	mu        sync.RWMutex
}

// Config holds Discord bot configuration
type Config struct {
	Token        string
	AllowedUsers []string
	AllowedChans []string
}

// New creates a new Discord bot
func New(cfg Config) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Build allowed maps for O(1) lookup
	allowedUsers := make(map[string]bool)
	for _, u := range cfg.AllowedUsers {
		allowedUsers[u] = true
	}

	allowedChans := make(map[string]bool)
	for _, c := range cfg.AllowedChans {
		allowedChans[c] = true
	}

	bot := &Bot{
		session:      session,
		allowedUsers: allowedUsers,
		allowedChans: allowedChans,
	}

	// Add handlers
	session.AddHandler(bot.onMessageCreate)
	session.AddHandler(bot.onInteractionCreate)

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentMessageContent |
		discordgo.IntentsGuilds

	return bot, nil
}

// Name returns the provider identifier.
func (b *Bot) Name() string { return "discord" }

// SetMessageHandler sets the message handler callback
func (b *Bot) SetMessageHandler(h chat.MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handler = h
}

// SetCommandHandler sets the slash command handler callback
func (b *Bot) SetCommandHandler(h chat.CommandHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.commandHandler = h
}

// Start connects the bot to Discord and registers slash commands
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord connection: %w", err)
	}

	// Get bot's user ID
	user, err := b.session.User("@me")
	if err != nil {
		return fmt.Errorf("failed to get bot user: %w", err)
	}
	b.botUserID = user.ID

	log.Printf("Discord bot connected as %s#%s", user.Username, user.Discriminator)

	// Register slash commands
	if err := b.registerCommands(); err != nil {
		log.Printf("Warning: failed to register slash commands: %v", err)
	}

	return nil
}

// Stop disconnects the bot from Discord.
// Commands are intentionally NOT unregistered on shutdown — global commands
// can take up to an hour to propagate, so deleting them causes a window
// where commands don't work after a restart. They're overwritten on next startup.
func (b *Bot) Stop() error {
	return b.session.Close()
}

// registerCommands registers slash commands with Discord
func (b *Bot) registerCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "new",
			Description: "Start a new conversation session",
		},
		{
			Name:        "sessions",
			Description: "List recent conversation sessions",
		},
		{
			Name:        "switch",
			Description: "Switch to an existing session",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "session_id",
					Description: "The session ID to switch to",
					Required:    true,
				},
			},
		},
		{
			Name:        "context",
			Description: "Show context window usage for the current session",
		},
	}

	for _, cmd := range commands {
		if _, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, "", cmd); err != nil {
			return fmt.Errorf("failed to register command %s: %w", cmd.Name, err)
		}
		log.Printf("Registered Discord command: /%s", cmd.Name)
	}

	return nil
}


// onInteractionCreate handles slash command interactions
func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()

	b.mu.RLock()
	handler := b.commandHandler
	b.mu.RUnlock()

	if handler == nil {
		return
	}

	// Defer the response (we have 3 seconds to acknowledge)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		log.Printf("Failed to defer interaction response: %v", err)
		return
	}

	// Extract args
	var args string
	if len(data.Options) > 0 {
		args = data.Options[0].StringValue()
	}

	// Get user ID
	userID := ""
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	// Call command handler with provider name
	response, err := handler("discord", i.ChannelID, userID, data.Name, args)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}
	if response == "" {
		response = "Done."
	}

	// Edit the deferred response with the actual content
	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &response,
	})
	if err != nil {
		log.Printf("Failed to edit interaction response: %v", err)
	}
}

// SendMessage sends a message to a channel or user
func (b *Bot) SendMessage(target, content string) error {
	// Handle user: or channel: prefixes
	if strings.HasPrefix(target, "user:") {
		userID := strings.TrimPrefix(target, "user:")
		return b.sendDM(userID, content)
	}

	if strings.HasPrefix(target, "channel:") {
		target = strings.TrimPrefix(target, "channel:")
	}

	_, err := b.session.ChannelMessageSend(target, content)
	return err
}

// sendDM sends a direct message to a user
func (b *Bot) sendDM(userID, content string) error {
	channel, err := b.session.UserChannelCreate(userID)
	if err != nil {
		return fmt.Errorf("failed to create DM channel: %w", err)
	}

	_, err = b.session.ChannelMessageSend(channel.ID, content)
	return err
}

// onMessageCreate handles incoming messages
func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == b.botUserID {
		return
	}

	// Check if user is allowed (empty map = all allowed)
	b.mu.RLock()
	if len(b.allowedUsers) > 0 && !b.allowedUsers[m.Author.ID] {
		b.mu.RUnlock()
		return
	}

	// Check if channel is allowed (empty map = all allowed)
	// DMs are always allowed if user is allowed
	isDM := m.GuildID == ""
	if !isDM && len(b.allowedChans) > 0 && !b.allowedChans[m.ChannelID] {
		b.mu.RUnlock()
		return
	}

	handler := b.handler
	b.mu.RUnlock()

	if handler == nil {
		return
	}

	// Call the message handler with provider name
	response, err := handler("discord", m.ChannelID, m.Author.ID, m.Content)
	if err != nil {
		log.Printf("Error handling message: %v", err)
		return
	}

	// Send response if not empty
	if response != "" {
		if _, err := s.ChannelMessageSend(m.ChannelID, response); err != nil {
			log.Printf("Error sending response: %v", err)
		}
	}
}

// React adds a reaction to a message
func (b *Bot) React(channelID, messageID, emoji string) error {
	return b.session.MessageReactionAdd(channelID, messageID, emoji)
}

// GetSession returns the underlying discordgo session for advanced use
func (b *Bot) GetSession() *discordgo.Session {
	return b.session
}
