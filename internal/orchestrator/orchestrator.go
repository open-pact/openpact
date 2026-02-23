// Package orchestrator coordinates all OpenPact components.
// It manages lifecycle, message routing, and context injection.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/auth"
	"github.com/open-pact/openpact/internal/chat"
	"github.com/open-pact/openpact/internal/config"
	opcontext "github.com/open-pact/openpact/internal/context"
	"github.com/open-pact/openpact/internal/engine"
	"github.com/open-pact/openpact/internal/mcp"
	"github.com/open-pact/openpact/internal/providers/discord"
	"github.com/open-pact/openpact/internal/providers/slack"
	"github.com/open-pact/openpact/internal/providers/telegram"
)

// Orchestrator coordinates all OpenPact components
type Orchestrator struct {
	cfg *config.Config

	// Components
	contextLoader *opcontext.Loader
	mcpServer     *mcp.Server
	engine        engine.Engine
	scriptStore   *admin.ScriptStore // Script approval store (optional)
	providerStore *admin.ProviderStore

	// Dynamic provider management
	providerMu     sync.RWMutex
	providers      map[string]chat.Provider
	providerStatus map[string]admin.ProviderStatusInfo

	// Communication pipes for MCP
	mcpReader *io.PipeReader
	mcpWriter *io.PipeWriter

	// Per-channel session tracking: "provider:channelID" -> sessionID
	channelSessions map[string]string
	sessionMu       sync.RWMutex

	// State
	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc
}

// channelSessionsFile is the JSON file that persists per-channel session mappings.
type channelSessionsFile struct {
	Sessions map[string]string `json:"sessions"`
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
		providers:       make(map[string]chat.Provider),
		providerStatus:  make(map[string]admin.ProviderStatusInfo),
	}

	// Initialize context loader
	o.contextLoader = opcontext.NewLoader(cfg.Workspace.Path)

	// Seed workspace with template context files if they don't exist
	seedContextTemplates(cfg.Workspace.Path)

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

	// Create MCP pipes
	o.mcpReader, o.mcpWriter = io.Pipe()

	// Initialize MCP server
	mcpResponseReader, mcpResponseWriter := io.Pipe()
	o.mcpServer = mcp.NewServer(o.mcpReader, mcpResponseWriter)

	// Register default MCP tools
	mcp.RegisterDefaultTools(o.mcpServer, cfg.Workspace.Path, o.ReloadContext)

	// Register calendar tools if calendars are configured
	if len(cfg.Calendars) > 0 {
		calConfigs := make([]mcp.CalendarConfig, len(cfg.Calendars))
		for i, c := range cfg.Calendars {
			calConfigs[i] = mcp.CalendarConfig{Name: c.Name, URL: c.URL}
		}
		mcp.RegisterCalendarTools(o.mcpServer, calConfigs)
	}

	// Register vault tools if vault is configured
	if cfg.Vault.Path != "" {
		vaultCfg := mcp.VaultConfig{
			Path:     cfg.Vault.Path,
			GitRepo:  cfg.Vault.GitRepo,
			AutoSync: cfg.Vault.AutoSync,
		}
		mcp.RegisterVaultTools(o.mcpServer, vaultCfg)
	}

	// Register web tools (always available)
	mcp.RegisterWebTools(o.mcpServer)

	// Register GitHub tools if enabled
	if cfg.GitHub.Enabled {
		token := os.Getenv("GITHUB_TOKEN")
		if token != "" {
			githubCfg := mcp.GitHubConfig{Token: token}
			mcp.RegisterGitHubTools(o.mcpServer, githubCfg)
		} else {
			log.Println("GitHub enabled but GITHUB_TOKEN not set, skipping")
		}
	}

	// Register Starlark script tools if enabled
	if cfg.Starlark.Enabled {
		scriptCfg := mcp.ScriptConfig{
			ScriptsDir:     cfg.Workspace.ScriptsDir(),
			MaxExecutionMs: cfg.Starlark.MaxExecutionMs,
		}

		// Load secrets from store
		secretStore := admin.NewSecretStore(cfg.Workspace.DataDir())
		secrets, err := secretStore.All()
		if err != nil {
			log.Printf("Warning: failed to load secrets: %v", err)
			secrets = map[string]string{}
		}
		scriptCfg.Secrets = secrets

		// Initialize script store for approval checking if admin is enabled
		if cfg.Admin.Enabled {
			scriptStore, err := admin.NewScriptStore(cfg.Workspace.ScriptsDir(), cfg.Workspace.DataDir(), cfg.Admin.Allowlist)
			if err != nil {
				log.Printf("Warning: failed to create script store for approvals: %v", err)
			} else {
				o.scriptStore = scriptStore
				scriptCfg.ScriptStore = scriptStore
				log.Println("Script approval checking enabled")
			}
		}

		mcp.RegisterScriptTools(o.mcpServer, scriptCfg)
	}

	// Register chat tools — always registered, providers resolved dynamically
	mcp.RegisterChatTools(o.mcpServer, o)

	// Check engine authentication
	authStatus := auth.CheckAuth(cfg.Engine.Type)
	if !authStatus.Authenticated {
		log.Printf("WARNING: Engine authentication not configured for %s.", cfg.Engine.Type)
		log.Printf("Visit the admin UI to sign in, or run: openpact auth %s", cfg.Engine.Type)
	}

	// Initialize engine
	engineCfg := engine.Config{
		Type:     cfg.Engine.Type,
		Provider: cfg.Engine.Provider,
		Model:    cfg.Engine.Model,
		DataDir:  cfg.Workspace.DataDir(),
		WorkDir:  cfg.Workspace.Path,
		Port:     cfg.Engine.Port,
		Password: cfg.Engine.Password,
	}
	eng, err := engine.New(engineCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine: %w", err)
	}

	// Load and set system prompt
	systemPrompt, err := o.contextLoader.Load()
	if err != nil {
		log.Printf("Warning: failed to load context: %v", err)
	}
	if systemPrompt != "" {
		eng.SetSystemPrompt(systemPrompt)
	}

	o.engine = eng

	// Close unused pipe ends to avoid leaks
	_ = mcpResponseReader

	return o, nil
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
	// Stop if running (ignore error if not running)
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

	// Start MCP server in background
	go func() {
		if err := o.mcpServer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("MCP server error: %v", err)
		}
	}()

	// Start engine (launches opencode serve)
	if err := o.engine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	log.Printf("Engine started: %s", o.cfg.Engine.Type)

	// Load persisted sessions from disk
	o.loadChannelSessions()

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
			// Status is already set to error by StartProvider
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

	// Stop all running chat providers
	o.providerMu.Lock()
	for name, p := range o.providers {
		if err := p.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("%s stop: %w", name, err))
		}
	}
	o.providers = make(map[string]chat.Provider)
	o.providerMu.Unlock()

	// Stop engine
	if o.engine != nil {
		if err := o.engine.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("engine stop: %w", err))
		}
	}

	// Stop MCP server
	if o.mcpServer != nil {
		o.mcpServer.Stop()
	}

	// Close pipes
	if o.mcpWriter != nil {
		o.mcpWriter.Close()
	}
	if o.mcpReader != nil {
		o.mcpReader.Close()
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

// handleChatMessage processes incoming chat messages from any provider.
func (o *Orchestrator) handleChatMessage(provider, channelID, userID, content string) (string, error) {
	log.Printf("[%s] Message from %s in %s: %s", provider, userID, channelID, content)

	// Get or create per-channel session
	sessionID := o.GetChannelSession(provider, channelID)
	if sessionID == "" {
		session, err := o.engine.CreateSession()
		if err != nil {
			return "", fmt.Errorf("failed to create session: %w", err)
		}
		sessionID = session.ID
		o.SetChannelSession(provider, channelID, sessionID)
		log.Printf("Created new session %s for %s:%s", sessionID, provider, channelID)
	}

	// Prepend source context so the AI knows the origin
	contextPrefix := fmt.Sprintf("[via %s, channel:%s, user:%s]\n", provider, channelID, userID)

	messages := []engine.Message{
		{Role: "user", Content: contextPrefix + content},
	}

	ctx := context.Background()
	responses, err := o.engine.Send(ctx, sessionID, messages)
	if err != nil {
		return "", fmt.Errorf("engine error: %w", err)
	}

	var responseText string
	for resp := range responses {
		responseText += resp.Content
	}

	return responseText, nil
}

// handleChatCommand processes slash/bot commands from any provider.
func (o *Orchestrator) handleChatCommand(provider, channelID, userID, command, args string) (string, error) {
	switch command {
	case "new":
		session, err := o.engine.CreateSession()
		if err != nil {
			return "", fmt.Errorf("failed to create session: %w", err)
		}
		o.SetChannelSession(provider, channelID, session.ID)
		title := session.Title
		if title == "" {
			title = "New session"
		}
		return fmt.Sprintf("New session started: `%s` - %s", session.ID, title), nil

	case "sessions":
		sessions, err := o.engine.ListSessions()
		if err != nil {
			return "", fmt.Errorf("failed to list sessions: %w", err)
		}
		if len(sessions) == 0 {
			return "No sessions found.", nil
		}
		activeID := o.GetChannelSession(provider, channelID)
		result := "**Sessions:**\n"
		for _, s := range sessions {
			marker := ""
			if s.ID == activeID {
				marker = " **(active in this channel)**"
			}
			title := s.Title
			if title == "" {
				title = "(untitled)"
			}
			result += fmt.Sprintf("- `%s` — %s%s\n", s.ID, title, marker)
		}
		return result, nil

	case "switch":
		if args == "" {
			return "Usage: /switch <session_id>", nil
		}
		session, err := o.engine.GetSession(args)
		if err != nil {
			return fmt.Sprintf("Session not found: %s", args), nil
		}
		o.SetChannelSession(provider, channelID, session.ID)
		title := session.Title
		if title == "" {
			title = "(untitled)"
		}
		return fmt.Sprintf("Switched to session: `%s` - %s", session.ID, title), nil

	case "context":
		sessionID := o.GetChannelSession(provider, channelID)
		if sessionID == "" {
			return "No active session in this channel. Send a message or use /new first.", nil
		}
		usage, err := o.engine.GetContextUsage(sessionID)
		if err != nil {
			return "", fmt.Errorf("failed to get context usage: %w", err)
		}
		return formatContextUsage(sessionID, usage), nil

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

// Engine returns the engine instance (for admin API wiring).
func (o *Orchestrator) Engine() engine.Engine {
	return o.engine
}

// CreateSession delegates to the engine.
func (o *Orchestrator) CreateSession() (*engine.Session, error) {
	return o.engine.CreateSession()
}

// ListSessions delegates to the engine.
func (o *Orchestrator) ListSessions() ([]engine.Session, error) {
	return o.engine.ListSessions()
}

// GetSession delegates to the engine.
func (o *Orchestrator) GetSession(id string) (*engine.Session, error) {
	return o.engine.GetSession(id)
}

// DeleteSession delegates to the engine.
func (o *Orchestrator) DeleteSession(id string) error {
	return o.engine.DeleteSession(id)
}

// GetMessages delegates to the engine.
func (o *Orchestrator) GetMessages(sessionID string, limit int) ([]engine.MessageInfo, error) {
	return o.engine.GetMessages(sessionID, limit)
}

// Send delegates to the engine.
func (o *Orchestrator) Send(ctx context.Context, sessionID string, messages []engine.Message) (<-chan engine.Response, error) {
	return o.engine.Send(ctx, sessionID, messages)
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

// GetContextUsage delegates to the engine.
func (o *Orchestrator) GetContextUsage(sessionID string) (*engine.ContextUsage, error) {
	return o.engine.GetContextUsage(sessionID)
}

// formatTokens formats a token count for display (e.g. 128500 -> "128.5k").
func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// formatContextUsage builds a human-readable context usage summary.
func formatContextUsage(sessionID string, usage *engine.ContextUsage) string {
	var b strings.Builder

	// Truncate session ID for display
	displayID := sessionID
	if len(displayID) > 8 {
		displayID = displayID[:8]
	}

	b.WriteString(fmt.Sprintf("**Context Usage** (session `%s`)\n", displayID))

	if usage.Model != "" {
		b.WriteString(fmt.Sprintf("Model: `%s`\n", usage.Model))
	}

	b.WriteString(fmt.Sprintf("Messages: %d assistant responses\n", usage.MessageCount))

	if usage.MessageCount == 0 {
		b.WriteString("No assistant messages yet — context usage unavailable.")
		return b.String()
	}

	// Current context with optional percentage
	if usage.ContextLimit > 0 {
		pct := float64(usage.CurrentContext) / float64(usage.ContextLimit) * 100
		b.WriteString(fmt.Sprintf("Current context: %s tokens (%.1f%% of %s)\n",
			formatTokens(usage.CurrentContext), pct, formatTokens(usage.ContextLimit)))
	} else {
		b.WriteString(fmt.Sprintf("Current context: %s tokens\n", formatTokens(usage.CurrentContext)))
	}

	// Output tokens
	if usage.TotalReasoning > 0 {
		b.WriteString(fmt.Sprintf("Total output: %s tokens (%s reasoning)\n",
			formatTokens(usage.TotalOutput), formatTokens(usage.TotalReasoning)))
	} else {
		b.WriteString(fmt.Sprintf("Total output: %s tokens\n", formatTokens(usage.TotalOutput)))
	}

	// Cache stats (only if non-zero)
	if usage.CacheRead > 0 || usage.CacheWrite > 0 {
		b.WriteString(fmt.Sprintf("Cache: %s read / %s write\n",
			formatTokens(usage.CacheRead), formatTokens(usage.CacheWrite)))
	}

	// Cost
	if usage.TotalCost > 0 {
		b.WriteString(fmt.Sprintf("Cost: $%.4f\n", usage.TotalCost))
	}

	return b.String()
}

// ReloadContext reloads context files (SOUL, USER, MEMORY)
func (o *Orchestrator) ReloadContext() error {
	systemPrompt, err := o.contextLoader.Load()
	if err != nil {
		return fmt.Errorf("failed to reload context: %w", err)
	}

	o.engine.SetSystemPrompt(systemPrompt)
	log.Println("Context reloaded successfully")
	return nil
}
