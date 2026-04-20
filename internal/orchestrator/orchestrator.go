// Package orchestrator coordinates all OpenPact components.
// It manages lifecycle, message routing, and context injection.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/chat"
	"github.com/open-pact/openpact/internal/config"
	opcontext "github.com/open-pact/openpact/internal/context"
	"github.com/open-pact/openpact/internal/engine"
	"github.com/open-pact/openpact/internal/mcp"
	"github.com/open-pact/openpact/internal/providers/discord"
	"github.com/open-pact/openpact/internal/providers/slack"
	"github.com/open-pact/openpact/internal/providers/telegram"
	"github.com/open-pact/openpact/internal/scheduler"
	"github.com/stack-bound/stackllm/agent"
	"github.com/stack-bound/stackllm/conversation"
	"github.com/stack-bound/stackllm/profile"
	"github.com/stack-bound/stackllm/session"
)

// Orchestrator coordinates all OpenPact components
type Orchestrator struct {
	cfg *config.Config

	// Components
	contextLoader *opcontext.Loader
	mcpServer     *mcp.Server
	stack         *engine.Stack
	scriptStore   *admin.ScriptStore // Script approval store (optional)
	providerStore *admin.ProviderStore
	scheduler     *scheduler.Scheduler

	// Dynamic provider management
	providerMu     sync.RWMutex
	providers      map[string]chat.Provider
	providerStatus map[string]admin.ProviderStatusInfo

	// Per-channel session tracking: "provider:channelID" -> stackllm session UUID
	channelSessions map[string]string
	sessionMu       sync.RWMutex

	// Per-session mutex, so concurrent messages to the same channel serialize
	// through the agent loop. Acquired in handleChatMessage.
	sessionLocksMu sync.Mutex
	sessionLocks   map[string]*sync.Mutex

	// Per-channel detail mode: "provider:channelID" -> mode (simple/thinking/tools/full)
	channelModes map[string]string
	modeMu       sync.RWMutex

	// State
	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc
}

// channelSessionsFile is the JSON file that persists per-channel session mappings.
type channelSessionsFile struct {
	Sessions map[string]string `json:"sessions"`
}

// channelModesFile is the JSON file that persists per-channel detail mode settings.
type channelModesFile struct {
	Modes map[string]string `json:"modes"`
}

// sessionKey builds the key for per-channel session lookup.
func sessionKey(provider, channelID string) string {
	return provider + ":" + channelID
}

// New creates a new Orchestrator with the given config
func New(cfg *config.Config, providerStore *admin.ProviderStore) (*Orchestrator, error) {
	o := &Orchestrator{
		cfg:             cfg,
		providerStore:   providerStore,
		channelSessions: make(map[string]string),
		channelModes:    make(map[string]string),
		sessionLocks:    make(map[string]*sync.Mutex),
		providers:       make(map[string]chat.Provider),
		providerStatus:  make(map[string]admin.ProviderStatusInfo),
	}

	// Initialize context loader (reads from AI-accessible data dir)
	o.contextLoader = opcontext.NewLoader(cfg.Workspace.AIDataDir())

	// Seed workspace with template context files if they don't exist
	seedContextTemplates(cfg.Workspace.AIDataDir())

	// Seed provider store from YAML config (one-time migration)
	if providerStore != nil {
		seedProviders := make(map[string]admin.ProviderConfig)
		if cfg.Discord.Enabled {
			seedProviders["discord"] = admin.ProviderConfig{
				Enabled:      true,
				AllowedUsers: cfg.Discord.AllowedUsers,
				AllowedChans: cfg.Discord.AllowedChans,
			}
		}
		if cfg.Telegram.Enabled {
			seedProviders["telegram"] = admin.ProviderConfig{
				Enabled:      true,
				AllowedUsers: cfg.Telegram.AllowedUsers,
			}
		}
		if cfg.Slack.Enabled {
			seedProviders["slack"] = admin.ProviderConfig{
				Enabled:      true,
				AllowedUsers: cfg.Slack.AllowedUsers,
				AllowedChans: cfg.Slack.AllowedChans,
			}
		}
		if len(seedProviders) > 0 {
			if err := providerStore.SeedFromConfig(seedProviders); err != nil {
				log.Printf("Warning: failed to seed provider store: %v", err)
			}
		}
	}

	// Initialize MCP server (used only for registering tool handlers — the
	// stackllm engine adapter copies each tool into a native Go registry).
	o.mcpServer = mcp.NewServer(nil, nil)

	// Build registration config for MCP tools
	regCfg := mcp.RegistrationConfig{
		WorkspacePath: cfg.Workspace.Path,
		AIDataDir:     cfg.Workspace.AIDataDir(),
		ReloadContext: o.ReloadContext,
		Chat:          o,
		Models:        o,
		Allowlist:     cfg.Admin.Allowlist,
	}

	// Calendar config
	if len(cfg.Calendars) > 0 {
		regCfg.Calendars = make([]mcp.CalendarConfig, len(cfg.Calendars))
		for i, c := range cfg.Calendars {
			regCfg.Calendars[i] = mcp.CalendarConfig{Name: c.Name, URL: c.URL}
		}
	}

	// Vault config
	if cfg.Vault.Path != "" {
		regCfg.Vault = &mcp.VaultConfig{
			Path:     cfg.Vault.Path,
			GitRepo:  cfg.Vault.GitRepo,
			AutoSync: cfg.Vault.AutoSync,
		}
	}

	// GitHub config
	if cfg.GitHub.Enabled {
		token := os.Getenv("GITHUB_TOKEN")
		if token != "" {
			regCfg.GitHub = &mcp.GitHubConfig{Token: token}
		} else {
			log.Println("GitHub enabled but GITHUB_TOKEN not set, skipping")
		}
	}

	// Starlark script config
	if cfg.Starlark.Enabled {
		regCfg.Script = &mcp.ScriptRegistrationConfig{
			ScriptsDir:     cfg.Workspace.ScriptsDir(),
			MaxExecutionMs: cfg.Starlark.MaxExecutionMs,
		}
	}

	// Initialize scheduler
	scheduleStore := admin.NewScheduleStore(cfg.Workspace.DataDir())
	schedCfg := scheduler.Config{
		ScriptsDir:     cfg.Workspace.ScriptsDir(),
		MaxExecutionMs: cfg.Starlark.MaxExecutionMs,
	}
	// Load secrets for scheduler's script execution
	secretStore := admin.NewSecretStore(cfg.Workspace.DataDir())
	if secrets, err := secretStore.All(); err == nil {
		schedCfg.Secrets = secrets
	}
	// Script approval store
	if cfg.Admin.Enabled {
		scriptStore, err := admin.NewScriptStore(cfg.Workspace.ScriptsDir(), cfg.Workspace.DataDir(), cfg.Admin.Allowlist)
		if err == nil {
			schedCfg.ScriptStore = scriptStore
		}
	}
	o.scheduler = scheduler.New(scheduleStore, schedCfg)
	regCfg.Scheduler = o

	// Register all tools on the MCP server — this is the registry the
	// stackllm adapter will read from when building the tool registry.
	mcp.RegisterAllTools(o.mcpServer, regCfg)

	// Store script store reference for admin API
	if regCfg.Script != nil && cfg.Admin.Enabled {
		scriptStore, err := admin.NewScriptStore(cfg.Workspace.ScriptsDir(), cfg.Workspace.DataDir(), cfg.Admin.Allowlist)
		if err != nil {
			log.Printf("Warning: failed to create script store for admin: %v", err)
		} else {
			o.scriptStore = scriptStore
		}
	}

	// Build the stackllm stack — this copies every MCP tool into a native
	// Go registry, opens the SQLite session store, and wires the
	// web.ManagedHandler that the admin server will mount at /api/engine/.
	stack, err := engine.New(engine.Config{
		WorkspacePath: cfg.Workspace.Path,
		DBPath:        cfg.Engine.DBPath,
		Tools:         o.mcpServer,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build engine stack: %w", err)
	}
	o.stack = stack

	// Load system prompt from SOUL/USER/MEMORY
	systemPrompt, err := o.contextLoader.Load()
	if err != nil {
		log.Printf("Warning: failed to load context: %v", err)
	}
	if systemPrompt != "" {
		stack.SetSystemPrompt(systemPrompt)
	}

	return o, nil
}

// Stack returns the stackllm stack (engine.Stack). Exposed so the admin
// server can mount stack.Handler and call into stack.Manager for the
// advanced-settings views.
func (o *Orchestrator) Stack() *engine.Stack {
	return o.stack
}

// StartProvider starts a single chat provider by name using config from the store.
func (o *Orchestrator) StartProvider(name string) error {
	o.providerMu.Lock()
	if _, running := o.providers[name]; running {
		o.providerMu.Unlock()
		return fmt.Errorf("provider %s is already running", name)
	}
	o.providerStatus[name] = admin.ProviderStatusInfo{State: "starting"}
	o.providerMu.Unlock()

	cfg, err := o.providerStore.Get(name)
	if err != nil {
		o.setProviderError(name, fmt.Sprintf("config not found: %v", err))
		return fmt.Errorf("failed to get config for %s: %w", name, err)
	}

	provider, err := o.createProvider(name, cfg)
	if err != nil {
		o.setProviderError(name, err.Error())
		return err
	}

	provider.SetMessageHandler(o.handleChatMessage)
	provider.SetCommandHandler(o.handleChatCommand)

	if err := provider.Start(); err != nil {
		o.setProviderError(name, err.Error())
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	o.providerMu.Lock()
	o.providers[name] = provider
	o.providerStatus[name] = admin.ProviderStatusInfo{State: "connected"}
	o.providerMu.Unlock()

	log.Printf("Chat provider started: %s", name)
	return nil
}

// StopProvider stops a running chat provider.
func (o *Orchestrator) StopProvider(name string) error {
	o.providerMu.Lock()
	provider, ok := o.providers[name]
	if !ok {
		o.providerMu.Unlock()
		return fmt.Errorf("provider %s is not running", name)
	}
	delete(o.providers, name)
	o.providerMu.Unlock()

	err := provider.Stop()

	o.providerMu.Lock()
	if err != nil {
		o.providerStatus[name] = admin.ProviderStatusInfo{State: "error", Error: err.Error()}
	} else {
		o.providerStatus[name] = admin.ProviderStatusInfo{State: "stopped"}
	}
	o.providerMu.Unlock()

	log.Printf("Chat provider stopped: %s", name)
	return err
}

// RestartProvider stops then starts a provider.
func (o *Orchestrator) RestartProvider(name string) error {
	o.providerMu.RLock()
	_, isRunning := o.providers[name]
	o.providerMu.RUnlock()

	if isRunning {
		if err := o.StopProvider(name); err != nil {
			log.Printf("Warning: error stopping %s during restart: %v", name, err)
		}
	}

	return o.StartProvider(name)
}

// GetProviderStatus returns the status of a single provider.
func (o *Orchestrator) GetProviderStatus(name string) (admin.ProviderStatusInfo, error) {
	o.providerMu.RLock()
	defer o.providerMu.RUnlock()

	status, ok := o.providerStatus[name]
	if !ok {
		return admin.ProviderStatusInfo{State: "stopped"}, nil
	}
	return status, nil
}

// ListProviderStatuses returns status for all known providers.
func (o *Orchestrator) ListProviderStatuses() map[string]admin.ProviderStatusInfo {
	o.providerMu.RLock()
	defer o.providerMu.RUnlock()

	result := make(map[string]admin.ProviderStatusInfo, len(o.providerStatus))
	for k, v := range o.providerStatus {
		result[k] = v
	}
	return result
}

// GetActiveProviderNames returns names of currently running providers (implements mcp.ChatProviderLookup).
func (o *Orchestrator) GetActiveProviderNames() []string {
	o.providerMu.RLock()
	defer o.providerMu.RUnlock()

	names := make([]string, 0, len(o.providers))
	for name := range o.providers {
		names = append(names, name)
	}
	return names
}

// SendViaProvider sends a message through a specific provider (implements mcp.ChatProviderLookup).
func (o *Orchestrator) SendViaProvider(provider, target, content string) error {
	o.providerMu.RLock()
	p, ok := o.providers[provider]
	o.providerMu.RUnlock()

	if !ok {
		return fmt.Errorf("provider %s is not running", provider)
	}
	return p.SendMessage(target, content)
}

func (o *Orchestrator) setProviderError(name, errMsg string) {
	o.providerMu.Lock()
	o.providerStatus[name] = admin.ProviderStatusInfo{State: "error", Error: errMsg}
	o.providerMu.Unlock()
}

func (o *Orchestrator) createProvider(name string, cfg admin.ProviderConfig) (chat.Provider, error) {
	switch name {
	case "discord":
		token := o.providerStore.ResolveToken("discord", "token")
		if token == "" {
			return nil, fmt.Errorf("discord token not available (set via UI or DISCORD_TOKEN env var)")
		}
		return discord.New(discord.Config{
			Token:        token,
			AllowedUsers: cfg.AllowedUsers,
			AllowedChans: cfg.AllowedChans,
		})
	case "telegram":
		token := o.providerStore.ResolveToken("telegram", "token")
		if token == "" {
			return nil, fmt.Errorf("telegram token not available (set via UI or TELEGRAM_BOT_TOKEN env var)")
		}
		return telegram.New(telegram.Config{
			Token:        token,
			AllowedUsers: cfg.AllowedUsers,
		})
	case "slack":
		botToken := o.providerStore.ResolveToken("slack", "bot_token")
		appToken := o.providerStore.ResolveToken("slack", "app_token")
		if botToken == "" || appToken == "" {
			return nil, fmt.Errorf("slack tokens not available (set via UI or SLACK_BOT_TOKEN/SLACK_APP_TOKEN env vars)")
		}
		return slack.New(slack.Config{
			BotToken:     botToken,
			AppToken:     appToken,
			AllowedUsers: cfg.AllowedUsers,
			AllowedChans: cfg.AllowedChans,
		})
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

// Start begins the orchestrator
func (o *Orchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	if o.running {
		o.mu.Unlock()
		return fmt.Errorf("orchestrator already running")
	}
	o.running = true

	ctx, o.cancel = context.WithCancel(ctx)
	o.mu.Unlock()

	log.Println("OpenPact orchestrator starting...")

	// Load persisted sessions and modes from disk
	o.loadChannelSessions()
	o.loadChannelModes()

	// Wire scheduler APIs now that the stack is ready
	o.scheduler.SetEngineAPI(o)
	o.scheduler.SetChatAPI(o)

	// Start scheduler
	if err := o.scheduler.Start(ctx); err != nil {
		log.Printf("Warning: failed to start scheduler: %v", err)
	}

	// Start enabled providers from store (failures are non-fatal)
	o.startEnabledProviders()

	log.Println("OpenPact orchestrator started successfully")

	// Wait for context cancellation
	<-ctx.Done()

	return o.shutdown()
}

// startEnabledProviders starts all providers that are enabled in the store.
func (o *Orchestrator) startEnabledProviders() {
	if o.providerStore == nil {
		return
	}

	configs, err := o.providerStore.List()
	if err != nil {
		log.Printf("Warning: failed to list providers: %v", err)
		return
	}

	for _, cfg := range configs {
		if !cfg.Enabled {
			o.providerMu.Lock()
			o.providerStatus[cfg.Name] = admin.ProviderStatusInfo{State: "stopped"}
			o.providerMu.Unlock()
			continue
		}

		if err := o.StartProvider(cfg.Name); err != nil {
			log.Printf("Warning: failed to start provider %s: %v", cfg.Name, err)
		}
	}
}

// Stop gracefully stops the orchestrator
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cancel != nil {
		o.cancel()
	}
}

// shutdown cleans up all components
func (o *Orchestrator) shutdown() error {
	log.Println("OpenPact orchestrator shutting down...")

	var errs []error

	if o.scheduler != nil {
		o.scheduler.Stop()
	}

	o.providerMu.Lock()
	for name, p := range o.providers {
		if err := p.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("%s stop: %w", name, err))
		}
	}
	o.providers = make(map[string]chat.Provider)
	o.providerMu.Unlock()

	if o.stack != nil {
		if err := o.stack.Close(); err != nil {
			errs = append(errs, fmt.Errorf("engine close: %w", err))
		}
	}

	if o.mcpServer != nil {
		o.mcpServer.Stop()
	}

	o.mu.Lock()
	o.running = false
	o.mu.Unlock()

	log.Println("OpenPact orchestrator stopped")

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// sessionLock returns a per-session mutex, creating one if needed.
func (o *Orchestrator) sessionLock(sessionID string) *sync.Mutex {
	o.sessionLocksMu.Lock()
	defer o.sessionLocksMu.Unlock()
	m, ok := o.sessionLocks[sessionID]
	if !ok {
		m = &sync.Mutex{}
		o.sessionLocks[sessionID] = m
	}
	return m
}

// handleChatMessage processes incoming chat messages from any provider.
//
// Per the migration plan, this path:
//  1. Looks up (or creates) a stackllm session keyed by (provider, channelID).
//  2. Acquires the per-session mutex so concurrent messages serialize.
//  3. Loads session messages from SQLite, prepends the SOUL/USER/MEMORY
//     system prompt if the history is empty, and appends the user turn.
//  4. Builds a fresh provider from the persisted default via
//     profile.Manager.LoadDefault and runs an agent.Agent to completion.
//  5. Persists the updated messages and returns the assembled response.
func (o *Orchestrator) handleChatMessage(provider, channelID, userID, content string) (*chat.ChatResponse, error) {
	log.Printf("[%s] Message from %s in %s: %s", provider, userID, channelID, content)

	sessionID, err := o.ensureChannelSession(provider, channelID)
	if err != nil {
		return nil, err
	}

	// Look up the channel's detail mode
	mode := o.GetChannelMode(provider, channelID)
	wantThinking := mode == chat.ModeThinking || mode == chat.ModeFull
	wantTools := mode == chat.ModeTools || mode == chat.ModeFull

	lock := o.sessionLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	ctx := context.Background()

	sess, err := o.stack.Sessions.Load(ctx, sessionID)
	if err != nil {
		sess = session.New()
		sess.ID = sessionID
	}

	// If the session is empty, prepend the system prompt so the model sees
	// SOUL/USER/MEMORY context. We check here (not at send time) because
	// the system message should live at the head of the history forever,
	// but we only want one copy.
	if len(sess.Messages) == 0 {
		if prompt := o.stack.SystemPrompt(); prompt != "" {
			sess.AppendMessage(conversation.Message{
				Role: conversation.RoleSystem,
				Blocks: []conversation.Block{
					{Type: conversation.BlockText, Text: prompt},
				},
			})
		}
	}

	// Prepend source context so the AI knows the origin.
	contextPrefix := fmt.Sprintf("[via %s, channel:%s, user:%s]\n", provider, channelID, userID)
	sess.AppendMessage(conversation.Message{
		Role: conversation.RoleUser,
		Blocks: []conversation.Block{
			{Type: conversation.BlockText, Text: contextPrefix + content},
		},
	})

	info, ok, err := o.stack.Manager.Default(ctx)
	if err != nil {
		return nil, fmt.Errorf("load default model: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("no default model set — open the admin UI and pick one at /engine")
	}
	sess.Model = info.String()

	prov, err := o.stack.Manager.LoadProviderForModel(ctx, info)
	if err != nil {
		return nil, fmt.Errorf("load provider for %s: %w", info.String(), err)
	}

	a := agent.New(prov,
		agent.WithModel(info.Model),
		agent.WithTools(o.stack.Tools),
	)

	events, err := a.Run(ctx, sess.Messages)
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	result, finalMessages, streamErr := accumulateChatResponse(events, wantThinking, wantTools)
	if len(finalMessages) > 0 {
		sess.Messages = finalMessages
	}
	if streamErr != nil {
		if persistErr := o.stack.Sessions.Save(context.Background(), sess); persistErr != nil {
			log.Printf("Warning: failed to save session after error: %v", persistErr)
		}
		return nil, streamErr
	}

	if err := o.stack.Sessions.Save(context.Background(), sess); err != nil {
		log.Printf("Warning: failed to save session %s: %v", sessionID, err)
	}
	return result, nil
}

// accumulateChatResponse drains an agent event stream into a chat
// response. A single agent turn can emit multiple text blocks around
// tool-use ("Looking that up…" → tool_use → "Here's what I found.")
// so completed text/thinking blocks are separated by a blank line
// rather than replacing each other. Factored out of sendChatMessage to
// be unit-testable — see orchestrator_accumulate_test.go.
//
// Return value: the assembled response, the authoritative message slice
// the agent built (empty if the stream never emitted EventComplete or
// EventError with messages), and any terminal error reported via
// EventError.
func accumulateChatResponse(events <-chan agent.Event, wantThinking, wantTools bool) (*chat.ChatResponse, []conversation.Message, error) {
	var (
		textBuilder     strings.Builder
		thinkingBuilder strings.Builder
		curText         strings.Builder
		curThinking     strings.Builder
		toolCalls       []chat.ToolCallInfo
		messages        []conversation.Message
	)

	appendBlock := func(dst, cur *strings.Builder, final string) {
		cur.Reset()
		if dst.Len() > 0 {
			dst.WriteString("\n\n")
		}
		dst.WriteString(final)
	}

	for ev := range events {
		switch ev.Type {
		case agent.EventBlockDelta:
			if ev.BlockType == conversation.BlockText {
				curText.WriteString(ev.Content)
			} else if ev.BlockType == conversation.BlockThinking && wantThinking {
				curThinking.WriteString(ev.Content)
			}
		case agent.EventBlockEnd:
			if ev.Block == nil {
				continue
			}
			switch ev.Block.Type {
			case conversation.BlockText:
				appendBlock(&textBuilder, &curText, ev.Block.Text)
			case conversation.BlockThinking:
				if wantThinking {
					appendBlock(&thinkingBuilder, &curThinking, ev.Block.Text)
				}
			case conversation.BlockToolUse:
				if wantTools {
					toolCalls = append(toolCalls, chat.ToolCallInfo{
						Name:  ev.Block.ToolName,
						Input: ev.Block.ToolArgsJSON,
					})
				}
			}
		case agent.EventToolResult:
			if !wantTools || ev.ToolCall == nil {
				continue
			}
			for i := len(toolCalls) - 1; i >= 0; i-- {
				if toolCalls[i].Output == "" && toolCalls[i].Name == ev.ToolCall.Name {
					toolCalls[i].Output = ev.ToolResult
					break
				}
			}
		case agent.EventComplete:
			messages = append([]conversation.Message(nil), ev.Messages...)
		case agent.EventError:
			if len(ev.Messages) > 0 {
				messages = append([]conversation.Message(nil), ev.Messages...)
			}
			return nil, messages, ev.Err
		}
	}

	// Flush any block that closed without a BlockEnd — defensive against
	// an early stream termination leaving deltas buffered.
	if curText.Len() > 0 {
		appendBlock(&textBuilder, &curText, curText.String())
	}
	if wantThinking && curThinking.Len() > 0 {
		appendBlock(&thinkingBuilder, &curThinking, curThinking.String())
	}

	result := &chat.ChatResponse{Text: textBuilder.String()}
	if wantThinking {
		result.Thinking = strings.ReplaceAll(thinkingBuilder.String(), `\n`, "")
	}
	if wantTools && len(toolCalls) > 0 {
		result.ToolCalls = toolCalls
	}
	return result, messages, nil
}

// ensureChannelSession returns the session ID for a channel, creating a
// new stackllm session if none is mapped yet. New sessions are named with
// their chat-provider origin so the admin UI list shows "Discord: channel"
// alongside direct-chat sessions rather than a bare UUID.
func (o *Orchestrator) ensureChannelSession(provider, channelID string) (string, error) {
	if sid := o.GetChannelSession(provider, channelID); sid != "" {
		return sid, nil
	}
	sess := session.New()
	sess.Name = channelSessionName(provider, channelID)
	if err := o.stack.Sessions.Save(context.Background(), sess); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	o.SetChannelSession(provider, channelID, sess.ID)
	log.Printf("Created new session %s (%q) for %s:%s", sess.ID, sess.Name, provider, channelID)
	return sess.ID, nil
}

// channelSessionName produces a human-readable session name for a chat-
// provider session so the admin UI list is browsable. Keeps the channel
// identifier verbatim — chat-provider IDs are opaque and we don't have a
// cheap way to resolve them to display names here.
func channelSessionName(provider, channelID string) string {
	providerLabel := strings.ToUpper(provider[:1]) + provider[1:]
	if channelID == "" {
		return providerLabel
	}
	return fmt.Sprintf("%s: %s", providerLabel, channelID)
}

// handleChatCommand processes slash/bot commands from any provider.
func (o *Orchestrator) handleChatCommand(provider, channelID, userID, command, args string) (string, error) {
	log.Printf("[%s] Command from %s in %s: /%s %s", provider, userID, channelID, command, args)

	switch command {
	case "new":
		sess := session.New()
		sess.Name = channelSessionName(provider, channelID)
		if err := o.stack.Sessions.Save(context.Background(), sess); err != nil {
			return "", fmt.Errorf("failed to create session: %w", err)
		}
		o.SetChannelSession(provider, channelID, sess.ID)
		return fmt.Sprintf("New session started: `%s`", sess.ID), nil

	case "sessions":
		sessions, err := o.stack.Sessions.List(context.Background())
		if err != nil {
			return "", fmt.Errorf("failed to list sessions: %w", err)
		}
		if len(sessions) == 0 {
			return "No sessions found.", nil
		}
		activeID := o.GetChannelSession(provider, channelID)
		var b strings.Builder
		b.WriteString("**Sessions:**\n")
		for _, s := range sessions {
			marker := ""
			if s.ID == activeID {
				marker = " **(active in this channel)**"
			}
			name := s.Name
			if name == "" {
				name = "(untitled)"
			}
			fmt.Fprintf(&b, "- `%s` — %s%s\n", s.ID, name, marker)
		}
		return b.String(), nil

	case "switch":
		if args == "" {
			return "Usage: /switch <session_id>", nil
		}
		sess, err := o.stack.Sessions.Load(context.Background(), args)
		if err != nil {
			return fmt.Sprintf("Session not found: %s", args), nil
		}
		o.SetChannelSession(provider, channelID, sess.ID)
		return fmt.Sprintf("Switched to session: `%s`", sess.ID), nil

	case "context":
		sessionID := o.GetChannelSession(provider, channelID)
		if sessionID == "" {
			return "No active session in this channel. Send a message or use /new first.", nil
		}
		usage, err := o.GetContextUsage(sessionID)
		if err != nil {
			return "", fmt.Errorf("failed to get context usage: %w", err)
		}
		return formatContextUsage(sessionID, usage), nil

	case "mode-simple":
		o.SetChannelMode(provider, channelID, chat.ModeSimple)
		return "Detail mode set to **simple** — responses will show text only.", nil

	case "mode-thinking":
		o.SetChannelMode(provider, channelID, chat.ModeThinking)
		return "Detail mode set to **thinking** — responses will include thinking blocks.", nil

	case "mode-tools":
		o.SetChannelMode(provider, channelID, chat.ModeTools)
		return "Detail mode set to **tools** — responses will include tool call details.", nil

	case "mode-full":
		o.SetChannelMode(provider, channelID, chat.ModeFull)
		return "Detail mode set to **full** — responses will include thinking blocks and tool call details.", nil

	default:
		return fmt.Sprintf("Unknown command: %s", command), nil
	}
}

// GetChannelSession returns the active session for a provider:channel pair.
func (o *Orchestrator) GetChannelSession(provider, channelID string) string {
	o.sessionMu.RLock()
	defer o.sessionMu.RUnlock()
	return o.channelSessions[sessionKey(provider, channelID)]
}

// SetChannelSession sets and persists the active session for a provider:channel pair.
func (o *Orchestrator) SetChannelSession(provider, channelID, sessionID string) {
	o.sessionMu.Lock()
	o.channelSessions[sessionKey(provider, channelID)] = sessionID
	o.sessionMu.Unlock()
	o.saveChannelSessions()
}

// loadChannelSessions reads per-channel session mappings from disk.
func (o *Orchestrator) loadChannelSessions() {
	path := o.channelSessionsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var f channelSessionsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return
	}

	o.sessionMu.Lock()
	for k, v := range f.Sessions {
		o.channelSessions[k] = v
	}
	o.sessionMu.Unlock()
	log.Printf("Restored %d channel sessions", len(f.Sessions))
}

// saveChannelSessions persists per-channel session mappings to disk.
func (o *Orchestrator) saveChannelSessions() {
	path := o.channelSessionsPath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Warning: failed to create data dir for channel sessions: %v", err)
		return
	}

	o.sessionMu.RLock()
	sessions := make(map[string]string, len(o.channelSessions))
	for k, v := range o.channelSessions {
		sessions[k] = v
	}
	o.sessionMu.RUnlock()

	data, err := json.Marshal(channelSessionsFile{Sessions: sessions})
	if err != nil {
		log.Printf("Warning: failed to marshal channel sessions: %v", err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("Warning: failed to save channel sessions: %v", err)
	}
}

// channelSessionsPath returns the path to the channel sessions file.
func (o *Orchestrator) channelSessionsPath() string {
	return filepath.Join(o.cfg.Workspace.DataDir(), "channel_sessions.json")
}

// GetChannelMode returns the detail mode for a provider:channel pair.
// Returns "simple" as the default when no mode is set.
func (o *Orchestrator) GetChannelMode(provider, channelID string) string {
	o.modeMu.RLock()
	defer o.modeMu.RUnlock()
	mode := o.channelModes[sessionKey(provider, channelID)]
	if mode == "" {
		return chat.ModeSimple
	}
	return mode
}

// SetChannelMode sets and persists the detail mode for a provider:channel pair.
func (o *Orchestrator) SetChannelMode(provider, channelID, mode string) {
	o.modeMu.Lock()
	o.channelModes[sessionKey(provider, channelID)] = mode
	o.modeMu.Unlock()
	o.saveChannelModes()
}

// ListChannelModes returns all channel mode settings.
func (o *Orchestrator) ListChannelModes() map[string]string {
	o.modeMu.RLock()
	defer o.modeMu.RUnlock()
	result := make(map[string]string, len(o.channelModes))
	for k, v := range o.channelModes {
		result[k] = v
	}
	return result
}

// loadChannelModes reads per-channel mode settings from disk.
func (o *Orchestrator) loadChannelModes() {
	path := o.channelModesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var f channelModesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return
	}

	o.modeMu.Lock()
	for k, v := range f.Modes {
		o.channelModes[k] = v
	}
	o.modeMu.Unlock()
	log.Printf("Restored %d channel modes", len(f.Modes))
}

// saveChannelModes persists per-channel mode settings to disk.
func (o *Orchestrator) saveChannelModes() {
	path := o.channelModesPath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Warning: failed to create data dir for channel modes: %v", err)
		return
	}

	o.modeMu.RLock()
	modes := make(map[string]string, len(o.channelModes))
	for k, v := range o.channelModes {
		modes[k] = v
	}
	o.modeMu.RUnlock()

	data, err := json.Marshal(channelModesFile{Modes: modes})
	if err != nil {
		log.Printf("Warning: failed to marshal channel modes: %v", err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("Warning: failed to save channel modes: %v", err)
	}
}

// channelModesPath returns the path to the channel modes file.
func (o *Orchestrator) channelModesPath() string {
	return filepath.Join(o.cfg.Workspace.DataDir(), "channel_modes.json")
}

// ContextUsage is the data surfaced by the /context chat command.
type ContextUsage struct {
	Model        string `json:"model"`
	MessageCount int    `json:"message_count"`
	ContextLimit int    `json:"context_limit"`
	PromptTokens int    `json:"prompt_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// GetContextUsage computes a lightweight usage summary from the session's
// LastUsage field (populated by the agent loop after each turn) and the
// persisted default model's context window.
func (o *Orchestrator) GetContextUsage(sessionID string) (*ContextUsage, error) {
	sess, err := o.stack.Sessions.Load(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}

	u := &ContextUsage{
		Model:        sess.Model,
		MessageCount: countAssistantMessages(sess),
	}

	if sess.LastUsage != nil {
		u.PromptTokens = sess.LastUsage.PromptTokens
		u.OutputTokens = sess.LastUsage.CompletionTokens
	}

	// Best-effort: pull context window from the current default model.
	if info, ok, err := o.stack.Manager.Default(context.Background()); err == nil && ok {
		u.ContextLimit = info.ContextWindow
	}

	return u, nil
}

func countAssistantMessages(sess *session.Session) int {
	n := 0
	for _, m := range sess.Messages {
		if m.Role == conversation.RoleAssistant {
			n++
		}
	}
	return n
}

// formatTokens formats a token count for display (e.g. 128500 -> "128.5k").
func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// formatContextUsage builds a human-readable context usage summary.
func formatContextUsage(sessionID string, usage *ContextUsage) string {
	var b strings.Builder

	displayID := sessionID
	if len(displayID) > 8 {
		displayID = displayID[:8]
	}

	b.WriteString(fmt.Sprintf("**Context Usage** (session `%s`)\n", displayID))

	if usage.Model != "" {
		b.WriteString(fmt.Sprintf("Model: `%s`\n", usage.Model))
	}

	b.WriteString(fmt.Sprintf("Messages: %d assistant responses\n", usage.MessageCount))

	if usage.ContextLimit > 0 {
		pct := float64(usage.PromptTokens) / float64(usage.ContextLimit) * 100
		b.WriteString(fmt.Sprintf("Current context: %s tokens (%.1f%% of %s)\n",
			formatTokens(usage.PromptTokens), pct, formatTokens(usage.ContextLimit)))
	} else if usage.PromptTokens > 0 {
		b.WriteString(fmt.Sprintf("Current context: %s tokens\n", formatTokens(usage.PromptTokens)))
	}

	if usage.OutputTokens > 0 {
		b.WriteString(fmt.Sprintf("Output tokens: %s\n", formatTokens(usage.OutputTokens)))
	}

	return b.String()
}

// ListModels implements mcp.ModelLookup by delegating to stackllm's profile.Manager.
func (o *Orchestrator) ListModels() ([]mcp.ModelInfo, error) {
	infos, err := o.stack.Manager.ListAllModels(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]mcp.ModelInfo, 0, len(infos))
	for _, m := range infos {
		out = append(out, mcp.ModelInfo{
			ProviderID: m.Provider,
			ModelID:    m.Model,
			Context:    m.ContextWindow,
		})
	}
	return out, nil
}

// GetDefaultModel implements mcp.ModelLookup.
func (o *Orchestrator) GetDefaultModel() (string, string) {
	info, ok, err := o.stack.Manager.Default(context.Background())
	if err != nil || !ok {
		return "", ""
	}
	return info.Provider, info.Model
}

// SetDefaultModel implements mcp.ModelLookup.
func (o *Orchestrator) SetDefaultModel(provider, model string) error {
	info := profile.ModelInfo{Provider: provider, Model: model}
	if err := o.stack.Manager.SetDefaultModel(info); err != nil {
		return err
	}
	log.Printf("Default model set to %s/%s", provider, model)
	return nil
}

// --- Scheduler helpers ---

// RunAgent implements scheduler.EngineAPI — runs an agent prompt to
// completion and returns the final assistant text.
func (o *Orchestrator) RunAgent(ctx context.Context, prompt string) (string, string, error) {
	info, ok, err := o.stack.Manager.Default(ctx)
	if err != nil {
		return "", "", fmt.Errorf("load default model: %w", err)
	}
	if !ok {
		return "", "", fmt.Errorf("no default model set")
	}
	prov, err := o.stack.Manager.LoadProviderForModel(ctx, info)
	if err != nil {
		return "", "", fmt.Errorf("load provider: %w", err)
	}

	sess := session.New()
	sess.Model = info.String()
	sess.Name = "Scheduled: " + truncateForName(prompt, 40)
	if p := o.stack.SystemPrompt(); p != "" {
		sess.AppendMessage(conversation.Message{
			Role:   conversation.RoleSystem,
			Blocks: []conversation.Block{{Type: conversation.BlockText, Text: p}},
		})
	}
	sess.AppendMessage(conversation.Message{
		Role:   conversation.RoleUser,
		Blocks: []conversation.Block{{Type: conversation.BlockText, Text: prompt}},
	})

	a := agent.New(prov, agent.WithModel(info.Model), agent.WithTools(o.stack.Tools))
	events, err := a.Run(ctx, sess.Messages)
	if err != nil {
		return "", "", err
	}

	// Concatenate every text block the agent emits this turn. A single
	// turn can produce multiple text blocks around tool calls
	// ("Looking that up…" → tool_use → "Here's what I found."). The
	// previous implementation reset the builder on each BlockEnd,
	// which kept only the final block — scheduled prompts that hit a
	// tool lost their pre-tool framing text.
	var text strings.Builder
	for ev := range events {
		switch ev.Type {
		case agent.EventBlockEnd:
			if ev.Block != nil && ev.Block.Type == conversation.BlockText {
				if text.Len() > 0 {
					text.WriteString("\n\n")
				}
				text.WriteString(ev.Block.Text)
			}
		case agent.EventComplete:
			sess.Messages = append([]conversation.Message(nil), ev.Messages...)
		case agent.EventError:
			return sess.ID, text.String(), ev.Err
		}
	}
	if err := o.stack.Sessions.Save(context.Background(), sess); err != nil {
		log.Printf("Warning: failed to save scheduler session %s: %v", sess.ID, err)
	}
	return sess.ID, text.String(), nil
}

// --- Schedule management (implements mcp.SchedulerLookup + admin.SchedulerAPI) ---

// List returns all schedules from the store.
func (o *Orchestrator) List() ([]*admin.Schedule, error) {
	return o.scheduler.Store().List()
}

// Get returns a schedule by ID.
func (o *Orchestrator) Get(id string) (*admin.Schedule, error) {
	return o.scheduler.Store().Get(id)
}

// Create creates a new schedule and reloads the scheduler.
func (o *Orchestrator) Create(sched *admin.Schedule) (*admin.Schedule, error) {
	created, err := o.scheduler.Store().Create(sched)
	if err != nil {
		return nil, err
	}
	o.scheduler.Reload()
	return created, nil
}

// Update updates a schedule and reloads the scheduler.
func (o *Orchestrator) Update(id string, updates *admin.Schedule) (*admin.Schedule, error) {
	updated, err := o.scheduler.Store().Update(id, updates)
	if err != nil {
		return nil, err
	}
	o.scheduler.Reload()
	return updated, nil
}

// Delete deletes a schedule and reloads the scheduler.
func (o *Orchestrator) Delete(id string) error {
	if err := o.scheduler.Store().Delete(id); err != nil {
		return err
	}
	o.scheduler.Reload()
	return nil
}

// SetEnabled enables or disables a schedule and reloads.
func (o *Orchestrator) SetEnabled(id string, enabled bool) error {
	if err := o.scheduler.Store().SetEnabled(id, enabled); err != nil {
		return err
	}
	o.scheduler.Reload()
	return nil
}

// RunNow triggers immediate execution of a schedule.
func (o *Orchestrator) RunNow(id string) error {
	return o.scheduler.RunNow(id)
}

// Reload reloads the scheduler's cron entries from the store.
func (o *Orchestrator) Reload() error {
	return o.scheduler.Reload()
}

// Scheduler returns the scheduler instance.
func (o *Orchestrator) Scheduler() *scheduler.Scheduler {
	return o.scheduler
}

// truncateForName trims a free-form string so it's safe to use as a session
// Name column. The store has no length limit but overlong names break the
// sidebar layout, so we cap at n runes and append an ellipsis if cut.
func truncateForName(s string, n int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// ReloadContext reloads context files (SOUL, USER, MEMORY) and updates the
// stack's system prompt.
func (o *Orchestrator) ReloadContext() error {
	systemPrompt, err := o.contextLoader.Load()
	if err != nil {
		return fmt.Errorf("failed to reload context: %w", err)
	}
	o.stack.SetSystemPrompt(systemPrompt)
	log.Println("Context reloaded successfully")
	return nil
}
