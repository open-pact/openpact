package mcp

import (
	"log"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/starlark"
)

// RegistrationConfig holds the configuration needed to register all MCP tools.
type RegistrationConfig struct {
	WorkspacePath string
	AIDataDir     string
	ReloadContext ContextReloader   // nil for standalone mode (no context reload)
	Calendars     []CalendarConfig
	Vault         *VaultConfig      // nil if not configured
	GitHub        *GitHubConfig     // nil if not configured
	Script        *ScriptRegistrationConfig // nil if not configured
	Chat          ChatProviderLookup        // nil for standalone mode
	Models        ModelLookup               // nil for standalone mode
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

	// Derive system data dir from workspace path for secrets/approvals
	dataDir := cfg.WorkspacePath + "/secure/data"

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

		// Load secrets from store if secrets not provided
		if len(scriptCfg.Secrets) == 0 {
			secretStore := admin.NewSecretStore(dataDir)
			secrets, err := secretStore.All()
			if err != nil {
				log.Printf("Warning: failed to load secrets: %v", err)
				secrets = map[string]string{}
			}
			scriptCfg.Secrets = secrets
		}

		// Initialize script store for approval checking
		if cfg.Script.ScriptStore == nil {
			scriptStore, err := admin.NewScriptStore(cfg.Script.ScriptsDir, dataDir, cfg.Allowlist)
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
}

// RegisterAllToolsFromEnv creates a RegistrationConfig from the standalone MCP server's
// environment variables and registers all tools. Used by cmd/mcp-server.
func RegisterAllToolsFromEnv(srv *Server, workspacePath, features string) {
	aiDataDir := workspacePath + "/ai-data"

	cfg := RegistrationConfig{
		WorkspacePath: workspacePath,
		AIDataDir:     aiDataDir,
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
