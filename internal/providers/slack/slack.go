// Package slack provides Slack bot integration for OpenPact using Socket Mode.
package slack

import (
	"fmt"
	"log"
	"strings"
	"sync"

	slacklib "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/open-pact/openpact/internal/chat"
)

var _ chat.Provider = (*Bot)(nil)

// Config holds Slack bot configuration.
type Config struct {
	BotToken     string
	AppToken     string
	AllowedUsers []string
	AllowedChans []string
}

// Bot represents a Slack bot using Socket Mode.
type Bot struct {
	client       *slacklib.Client
	socketClient *socketmode.Client
	handler      chat.MessageHandler
	cmdHandler   chat.CommandHandler
	allowedUsers map[string]bool
	allowedChans map[string]bool
	botUserID    string
	stopCh       chan struct{}
	done         chan struct{}
	mu           sync.RWMutex
}

// New creates a new Slack bot.
func New(cfg Config) (*Bot, error) {
	client := slacklib.New(
		cfg.BotToken,
		slacklib.OptionAppLevelToken(cfg.AppToken),
	)
	socketClient := socketmode.New(client)

	allowed := make(map[string]bool)
	for _, u := range cfg.AllowedUsers {
		allowed[u] = true
	}
	allowedChans := make(map[string]bool)
	for _, c := range cfg.AllowedChans {
		allowedChans[c] = true
	}

	return &Bot{
		client:       client,
		socketClient: socketClient,
		allowedUsers: allowed,
		allowedChans: allowedChans,
	}, nil
}

// Name returns the provider identifier.
func (b *Bot) Name() string { return "slack" }

// SetMessageHandler registers the callback for incoming user messages.
func (b *Bot) SetMessageHandler(h chat.MessageHandler) {
	b.mu.Lock()
	b.handler = h
	b.mu.Unlock()
}

// SetCommandHandler registers the callback for incoming commands.
func (b *Bot) SetCommandHandler(h chat.CommandHandler) {
	b.mu.Lock()
	b.cmdHandler = h
	b.mu.Unlock()
}

// Start connects to Slack via Socket Mode and begins listening.
func (b *Bot) Start() error {
	authResp, err := b.client.AuthTest()
	if err != nil {
		return fmt.Errorf("slack auth test failed: %w", err)
	}
	b.botUserID = authResp.UserID
	log.Printf("Slack bot connected as %s", authResp.User)

	b.stopCh = make(chan struct{})
	b.done = make(chan struct{})

	go b.handleEvents()
	go func() {
		if err := b.socketClient.Run(); err != nil {
			log.Printf("Slack socket mode error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully disconnects from Slack.
func (b *Bot) Stop() error {
	if b.stopCh != nil {
		close(b.stopCh)
		<-b.done
	}
	return nil
}

func (b *Bot) handleEvents() {
	defer close(b.done)
	for {
		select {
		case <-b.stopCh:
			return
		case evt, ok := <-b.socketClient.Events:
			if !ok {
				return
			}
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}
				b.socketClient.Ack(*evt.Request)
				b.handleEventsAPI(eventsAPIEvent)

			case socketmode.EventTypeSlashCommand:
				cmd, ok := evt.Data.(slacklib.SlashCommand)
				if !ok {
					continue
				}
				b.handleSlashCommand(cmd, evt)
			}
		}
	}
}

func (b *Bot) handleEventsAPI(event slackevents.EventsAPIEvent) {
	if event.Type != slackevents.CallbackEvent {
		return
	}

	switch ev := event.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		if ev.User == b.botUserID || ev.SubType != "" {
			return
		}

		b.mu.RLock()
		if len(b.allowedUsers) > 0 && !b.allowedUsers[ev.User] {
			b.mu.RUnlock()
			return
		}
		if len(b.allowedChans) > 0 && !b.allowedChans[ev.Channel] {
			b.mu.RUnlock()
			return
		}
		handler := b.handler
		b.mu.RUnlock()

		if handler == nil {
			return
		}

		response, err := handler("slack", ev.Channel, ev.User, ev.Text)
		if err != nil {
			log.Printf("Error handling Slack message: %v", err)
			return
		}
		if response != "" {
			if _, _, err := b.client.PostMessage(ev.Channel, slacklib.MsgOptionText(response, false)); err != nil {
				log.Printf("Error sending Slack response: %v", err)
			}
		}
	}
}

func (b *Bot) handleSlashCommand(cmd slacklib.SlashCommand, evt socketmode.Event) {
	b.mu.RLock()
	handler := b.cmdHandler
	b.mu.RUnlock()

	if handler == nil {
		b.socketClient.Ack(*evt.Request)
		return
	}

	// Strip "openpact-" prefix (Slack requires unique command names)
	command := strings.TrimPrefix(cmd.Command, "/openpact-")
	command = strings.TrimPrefix(command, "/")

	response, err := handler("slack", cmd.ChannelID, cmd.UserID, command, cmd.Text)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}
	if response == "" {
		response = "Done."
	}

	b.socketClient.Ack(*evt.Request, map[string]interface{}{
		"response_type": "ephemeral",
		"text":          response,
	})
}

// SendMessage sends a message to a Slack channel or user.
func (b *Bot) SendMessage(target, content string) error {
	target = strings.TrimPrefix(target, "user:")
	target = strings.TrimPrefix(target, "channel:")
	_, _, err := b.client.PostMessage(target, slacklib.MsgOptionText(content, false))
	return err
}
