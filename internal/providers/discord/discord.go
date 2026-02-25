// Package discord provides Discord bot integration for OpenPact.
package discord

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

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
		{
			Name:        "mode-simple",
			Description: "Set response detail mode to simple (text only)",
		},
		{
			Name:        "mode-thinking",
			Description: "Set response detail mode to show thinking blocks",
		},
		{
			Name:        "mode-tools",
			Description: "Set response detail mode to show tool call details",
		},
		{
			Name:        "mode-full",
			Description: "Set response detail mode to show thinking and tool calls",
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

	// Show typing indicator while waiting for the AI to respond.
	// Discord typing indicators last ~10 seconds, so we re-send every 8s
	// in a background goroutine until the handler returns.
	stopTyping := make(chan struct{})
	go func() {
		// Send initial typing indicator immediately
		if err := s.ChannelTyping(m.ChannelID); err != nil {
			log.Printf("Error sending typing indicator: %v", err)
		}
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopTyping:
				return
			case <-ticker.C:
				if err := s.ChannelTyping(m.ChannelID); err != nil {
					log.Printf("Error sending typing indicator: %v", err)
				}
			}
		}
	}()

	// Call the message handler with provider name
	response, err := handler("discord", m.ChannelID, m.Author.ID, m.Content)
	close(stopTyping)
	if err != nil {
		log.Printf("Error handling message: %v", err)
		return
	}

	// Send response if not empty
	if response != nil && response.Text != "" {
		if err := b.sendRichResponse(s, m.ChannelID, response); err != nil {
			log.Printf("Error sending response: %v", err)
		}
	}
}

// sendRichResponse sends a ChatResponse to Discord with optional embeds for
// thinking blocks and tool calls.
func (b *Bot) sendRichResponse(s *discordgo.Session, channelID string, resp *chat.ChatResponse) error {
	// Build embeds from thinking and tool call data
	var embeds []*discordgo.MessageEmbed

	if resp.Thinking != "" {
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title:       "Thinking",
			Description: truncate(resp.Thinking, 1000),
			Color:       0xa78bfa, // purple
		})
	}

	for _, tc := range resp.ToolCalls {
		desc := ""
		if tc.Input != "" {
			desc += "**Input:** " + truncate(tc.Input, 500)
		}
		if tc.Output != "" {
			if desc != "" {
				desc += "\n"
			}
			desc += "**Output:** " + truncate(tc.Output, 500)
		}
		if desc == "" {
			desc = "(no details)"
		}

		title := "Tool: " + tc.Name
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title:       truncate(title, 256),
			Description: truncate(desc, 4096),
			Color:       0xf59e0b, // orange
		})
	}

	// Discord limits: 2000 chars per message content, 10 embeds per message.
	// Split text into chunks if needed.
	textChunks := splitText(resp.Text, 2000)

	if len(textChunks) == 0 {
		textChunks = []string{""}
	}

	// Send first chunk with as many embeds as fit (max 10)
	firstEmbeds := embeds
	var overflowEmbeds []*discordgo.MessageEmbed
	if len(firstEmbeds) > 10 {
		overflowEmbeds = firstEmbeds[10:]
		firstEmbeds = firstEmbeds[:10]
	}

	// Send the first message with embeds
	_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: textChunks[0],
		Embeds:  firstEmbeds,
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Send overflow embeds in follow-up messages (10 per message)
	for len(overflowEmbeds) > 0 {
		batch := overflowEmbeds
		if len(batch) > 10 {
			batch = batch[:10]
			overflowEmbeds = overflowEmbeds[10:]
		} else {
			overflowEmbeds = nil
		}
		_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Embeds: batch,
		})
		if err != nil {
			return fmt.Errorf("failed to send overflow embeds: %w", err)
		}
	}

	// Send remaining text chunks as plain messages
	for _, chunk := range textChunks[1:] {
		if _, err := s.ChannelMessageSend(channelID, chunk); err != nil {
			return fmt.Errorf("failed to send text chunk: %w", err)
		}
	}

	return nil
}

// truncate shortens a string to max characters, adding ellipsis if truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "\u2026"
}

// splitText splits a string into chunks of at most maxLen characters.
func splitText(s string, maxLen int) []string {
	if len(s) <= maxLen {
		return []string{s}
	}
	var chunks []string
	for len(s) > 0 {
		chunk := s
		if len(chunk) > maxLen {
			chunk = chunk[:maxLen]
		}
		s = s[len(chunk):]
		chunks = append(chunks, chunk)
	}
	return chunks
}

// React adds a reaction to a message
func (b *Bot) React(channelID, messageID, emoji string) error {
	return b.session.MessageReactionAdd(channelID, messageID, emoji)
}

// GetSession returns the underlying discordgo session for advanced use
func (b *Bot) GetSession() *discordgo.Session {
	return b.session
}
