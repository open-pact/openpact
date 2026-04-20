package mcp

import (
	"context"
	"database/sql"
	"log"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/starlark"
	"github.com/open-pact/openpact/internal/storage/secrets"
)

// RegistrationConfig holds the configuration needed to register all MCP tools.
type RegistrationConfig struct {
	WorkspacePath string
	AIDataDir     string
	DB            *sql.DB // shared database handle (approvals + secrets live here now)
	Secrets       *secrets.Store // pre-constructed encrypted secrets store
	ReloadContext ContextReloader   // nil for standalone mode (no context reload)
	Calendars     []CalendarConfig
	Vault         *VaultConfig      // nil if not configured
	GitHub        *GitHubConfig     // nil if not configured
	Script        *ScriptRegistrationConfig // nil if not configured
	Chat          ChatProviderLookup        // nil for standalone mode
	Models        ModelLookup               // nil for standalone mode
	Scheduler     SchedulerLookup           // nil for standalone mode
	Allowlist     []string          // Script allowlist for admin
}

// ScriptRegistrationConfig holds config for registering script tools.
type ScriptRegistrationConfig struct {
	ScriptsDir     string
	MaxExecutionMs int64
	Secrets        map[string]string
	ScriptStore    *admin.ScriptStore // nil if approvals not enabled
}

// RegisterAllTools registers all MCP tools on the given server using the provided config.
// This is used by both the orchestrator (in-process) and the standalone MCP server binary.
func RegisterAllTools(srv *Server, cfg RegistrationConfig) {
	// Workspace + memory tools (always registered, scoped to AI data dir)
	RegisterDefaultTools(srv, cfg.AIDataDir, cfg.ReloadContext)

	// Calendar tools
	if len(cfg.Calendars) > 0 {
		RegisterCalendarTools(srv, cfg.Calendars)
	}

	// Vault tools
	if cfg.Vault != nil && cfg.Vault.Path != "" {
		RegisterVaultTools(srv, *cfg.Vault)
	}

	// Web tools (always available)
	RegisterWebTools(srv)

	// GitHub tools
	if cfg.GitHub != nil && cfg.GitHub.Token != "" {
		RegisterGitHubTools(srv, *cfg.GitHub)
	}

	// Starlark script tools
	if cfg.Script != nil {
		scriptCfg := ScriptConfig{
			ScriptsDir:     cfg.Script.ScriptsDir,
			MaxExecutionMs: cfg.Script.MaxExecutionMs,
			Secrets:        cfg.Script.Secrets,
			ScriptStore:    cfg.Script.ScriptStore,
		}

		// Load secrets from store if secrets not provided. The caller
		// is responsible for constructing the cipher-initialised
		// secrets store and passing it via cfg.Secrets (or pre-loading
		// the plaintext map into scriptCfg.Secrets for legacy paths).
		if len(scriptCfg.Secrets) == 0 && cfg.Secrets != nil {
			loaded, err := cfg.Secrets.All(context.Background())
			if err != nil {
				log.Printf("Warning: failed to load secrets: %v", err)
				loaded = map[string]string{}
			}
			scriptCfg.Secrets = loaded
		}

		// Initialize script store for approval checking. Requires a DB
		// — callers running in environments without SQLite access
		// (e.g. a future mcp-server-only binary that doesn't own the
		// workspace) should construct the ScriptStore themselves and
		// pass it in.
		if cfg.Script.ScriptStore == nil && cfg.DB != nil {
			scriptStore, err := admin.NewScriptStore(cfg.DB, cfg.Script.ScriptsDir, cfg.Allowlist)
			if err != nil {
				log.Printf("Warning: failed to create script store: %v", err)
			} else {
				scriptCfg.ScriptStore = scriptStore
				log.Println("Script approval checking enabled")
			}
		}

		RegisterScriptTools(srv, scriptCfg)
	}

	// Chat tools
	if cfg.Chat != nil {
		RegisterChatTools(srv, cfg.Chat)
	}

	// Model tools
	if cfg.Models != nil {
		RegisterModelTools(srv, cfg.Models)
	}

	// Schedule tools
	if cfg.Scheduler != nil {
		RegisterScheduleTools(srv, cfg.Scheduler)
	}
}

// RegisterAllToolsFromEnv creates a RegistrationConfig from the standalone MCP server's
// environment variables and registers all tools. Used by cmd/mcp-server.
// db is the shared SQLite handle — required for script approval
// checking and (after the secrets migration) secret decryption.
func RegisterAllToolsFromEnv(srv *Server, workspacePath, features string, db *sql.DB, secretStore *secrets.Store) {
	aiDataDir := workspacePath + "/ai-data"

	cfg := RegistrationConfig{
		WorkspacePath: workspacePath,
		AIDataDir:     aiDataDir,
		DB:            db,
		Secrets:       secretStore,
	}

	// In standalone mode, context reload is not available (the orchestrator handles it)
	cfg.ReloadContext = nil

	// Parse features to enable optional tools
	featureSet := parseFeatures(features)

	// Scripts are enabled by default if scripts dir exists
	scriptsDir := aiDataDir + "/scripts"
	if _, ok := featureSet["scripts"]; ok || features == "" {
		cfg.Script = &ScriptRegistrationConfig{
			ScriptsDir:     scriptsDir,
			MaxExecutionMs: 30000,
		}
	}

	RegisterAllTools(srv, cfg)
}

// parseFeatures parses a comma-separated feature string into a set.
func parseFeatures(features string) map[string]bool {
	result := make(map[string]bool)
	if features == "" {
		return result
	}
	for _, f := range splitAndTrim(features) {
		result[f] = true
	}
	return result
}

// splitAndTrim splits a string by comma and trims whitespace.
func splitAndTrim(s string) []string {
	var result []string
	for _, part := range splitString(s, ',') {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// splitString splits a string by a separator rune.
func splitString(s string, sep rune) []string {
	var result []string
	current := ""
	for _, r := range s {
		if r == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	result = append(result, current)
	return result
}

// trimSpace trims leading/trailing whitespace from a string.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// NewSecretProviderFromMap creates a starlark.SecretProvider from a map.
// Exported for use by the standalone MCP server.
func NewSecretProviderFromMap(secrets map[string]string) *starlark.SecretProvider {
	sp := starlark.NewSecretProvider()
	for name, value := range secrets {
		sp.Set(name, value)
	}
	return sp
}
