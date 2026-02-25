// Package telegram provides Telegram bot integration for OpenPact.
package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/open-pact/openpact/internal/chat"
)

var _ chat.Provider = (*Bot)(nil)

// Config holds Telegram bot configuration.
type Config struct {
	Token        string
	AllowedUsers []string
}

// Bot represents a Telegram bot.
type Bot struct {
	api            *tgbotapi.BotAPI
	handler        chat.MessageHandler
	commandHandler chat.CommandHandler
	allowedUsers   map[string]bool
	stopCh         chan struct{}
	mu             sync.RWMutex
}

// New creates a new Telegram bot.
func New(cfg Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	allowed := make(map[string]bool)
	for _, u := range cfg.AllowedUsers {
		allowed[u] = true
	}

	return &Bot{
		api:          api,
		allowedUsers: allowed,
		stopCh:       make(chan struct{}),
	}, nil
}

// Name returns the provider identifier.
func (b *Bot) Name() string { return "telegram" }

// SetMessageHandler registers the callback for incoming user messages.
func (b *Bot) SetMessageHandler(h chat.MessageHandler) {
	b.mu.Lock()
	b.handler = h
	b.mu.Unlock()
}

// SetCommandHandler registers the callback for incoming commands.
func (b *Bot) SetCommandHandler(h chat.CommandHandler) {
	b.mu.Lock()
	b.commandHandler = h
	b.mu.Unlock()
}

// Start connects to Telegram and begins listening for updates.
func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	log.Printf("Telegram bot connected as @%s", b.api.Self.UserName)

	go func() {
		for {
			select {
			case update := <-updates:
				if update.Message == nil {
					continue
				}
				b.handleUpdate(update)
			case <-b.stopCh:
				return
			}
		}
	}()

	return nil
}

// Stop gracefully disconnects from Telegram.
func (b *Bot) Stop() error {
	close(b.stopCh)
	b.api.StopReceivingUpdates()
	return nil
}

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	msg := update.Message
	userID := strconv.FormatInt(msg.From.ID, 10)
	chatID := strconv.FormatInt(msg.Chat.ID, 10)

	b.mu.RLock()
	if len(b.allowedUsers) > 0 && !b.allowedUsers[userID] && !b.allowedUsers[msg.From.UserName] {
		b.mu.RUnlock()
		return
	}

	if msg.IsCommand() {
		handler := b.commandHandler
		b.mu.RUnlock()
		if handler == nil {
			return
		}
		response, err := handler("telegram", chatID, userID, msg.Command(), msg.CommandArguments())
		if err != nil {
			response = fmt.Sprintf("Error: %v", err)
		}
		if response != "" {
			b.sendReply(msg.Chat.ID, response)
		}
		return
	}

	handler := b.handler
	b.mu.RUnlock()
	if handler == nil {
		return
	}

	response, err := handler("telegram", chatID, userID, msg.Text)
	if err != nil {
		log.Printf("Error handling Telegram message: %v", err)
		return
	}
	if response != nil && response.Text != "" {
		b.sendReply(msg.Chat.ID, response.Text)
	}
}

func (b *Bot) sendReply(chatID int64, content string) {
	// Telegram 4096-char limit — split if needed
	for len(content) > 0 {
		chunk := content
		if len(chunk) > 4096 {
			chunk = chunk[:4096]
		}
		content = content[len(chunk):]
		reply := tgbotapi.NewMessage(chatID, chunk)
		if _, err := b.api.Send(reply); err != nil {
			log.Printf("Error sending Telegram message: %v", err)
		}
	}
}

// SendMessage sends a message to a Telegram chat.
func (b *Bot) SendMessage(target, content string) error {
	target = strings.TrimPrefix(target, "user:")
	target = strings.TrimPrefix(target, "channel:")
	chatID, err := strconv.ParseInt(target, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Telegram chat ID %q: %w", target, err)
	}
	b.sendReply(chatID, content)
	return nil
}
