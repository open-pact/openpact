package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all OpenPact configuration
type Config struct {
	Engine    EngineConfig     `yaml:"engine"`
	Workspace WorkspaceConfig  `yaml:"workspace"`
	Discord   DiscordConfig    `yaml:"discord"`
	Telegram  TelegramConfig   `yaml:"telegram"`
	Slack     SlackConfig      `yaml:"slack"`
	GitHub    GitHubConfig     `yaml:"github"`
	Calendars []CalendarConfig `yaml:"calendars"`
	Vault     VaultConfig      `yaml:"vault"`
	Starlark  StarlarkConfig   `yaml:"starlark"`
	Logging   LoggingConfig    `yaml:"logging"`
	Server    ServerConfig     `yaml:"server"`
	Admin     AdminConfig      `yaml:"admin"`
}

// AdminConfig configures the admin web UI
type AdminConfig struct {
	Enabled   bool     `yaml:"enabled"`   // Enable admin UI
	Bind      string   `yaml:"bind"`      // Address to bind (e.g., "localhost:8080")
	Allowlist []string `yaml:"allowlist"` // Always-approved scripts
	DevMode   bool     `yaml:"dev_mode"`  // Disable secure cookies for localhost
}

// LoggingConfig configures structured logging
type LoggingConfig struct {
	Level string `yaml:"level"` // debug, info, warn, error
	JSON  bool   `yaml:"json"`  // Output JSON format
}

// ServerConfig configures the HTTP server (health/metrics)
type ServerConfig struct {
	HealthAddr string          `yaml:"health_addr"` // Address for health endpoint
	RateLimit  RateLimitConfig `yaml:"rate_limit"`
}

// RateLimitConfig configures rate limiting
type RateLimitConfig struct {
	Rate  float64 `yaml:"rate"`  // Requests per second
	Burst int     `yaml:"burst"` // Max burst size
}

// GitHubConfig configures GitHub API integration
type GitHubConfig struct {
	Enabled bool `yaml:"enabled"` // Enable GitHub tools
}

// VaultConfig configures Obsidian vault integration
type VaultConfig struct {
	Path     string `yaml:"path"`      // Local path to vault
	GitRepo  string `yaml:"git_repo"`  // Git repository URL (optional)
	AutoSync bool   `yaml:"auto_sync"` // Auto pull/push on read/write
}

// CalendarConfig configures a calendar feed
type CalendarConfig struct {
	Name string `yaml:"name"` // Display name
	URL  string `yaml:"url"`  // iCal feed URL
}

// EngineConfig configures the AI engine
type EngineConfig struct {
	Type     string `yaml:"type"`     // "opencode"
	Provider string `yaml:"provider"` // For OpenCode: "anthropic", "openai", "ollama", etc.
	Model    string `yaml:"model"`    // Model name
	Port     int    `yaml:"port"`     // Port for opencode serve (default: 4098)
	Password string `yaml:"password"` // Optional OPENCODE_SERVER_PASSWORD
}

// WorkspaceConfig configures workspace paths
type WorkspaceConfig struct {
	Path string `yaml:"path"` // Base workspace path
}

// SecureDir returns the path to the secure directory (system-only, AI has zero access).
func (w WorkspaceConfig) SecureDir() string {
	return filepath.Join(w.Path, "secure")
}

// AIDataDir returns the path to the AI-accessible data directory.
func (w WorkspaceConfig) AIDataDir() string {
	return filepath.Join(w.Path, "ai-data")
}

// DataDir returns the path to the system data directory within the secure area.
func (w WorkspaceConfig) DataDir() string {
	return filepath.Join(w.Path, "secure", "data")
}

// ScriptsDir returns the path to the scripts directory within the AI-accessible area.
func (w WorkspaceConfig) ScriptsDir() string {
	return filepath.Join(w.Path, "ai-data", "scripts")
}

// EnsureDirs creates all required workspace directories if they don't exist.
func (w WorkspaceConfig) EnsureDirs() error {
	dirs := []string{
		w.Path,
		w.SecureDir(),
		w.DataDir(),
		w.AIDataDir(),
		w.ScriptsDir(),
		filepath.Join(w.AIDataDir(), "memory"),
		filepath.Join(w.AIDataDir(), "skills"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// DiscordConfig configures Discord integration
type DiscordConfig struct {
	Enabled      bool     `yaml:"enabled"`
	AllowedUsers []string `yaml:"allowed_users"` // User IDs allowed to DM
	AllowedChans []string `yaml:"allowed_chans"` // Channel IDs allowed
}

// TelegramConfig configures Telegram bot integration
type TelegramConfig struct {
	Enabled      bool     `yaml:"enabled"`
	AllowedUsers []string `yaml:"allowed_users"` // User IDs or usernames allowed
}

// SlackConfig configures Slack bot integration (Socket Mode)
type SlackConfig struct {
	Enabled      bool     `yaml:"enabled"`
	AllowedUsers []string `yaml:"allowed_users"` // Slack user IDs allowed
	AllowedChans []string `yaml:"allowed_chans"` // Slack channel IDs allowed
}

// StarlarkConfig configures Starlark script limits
type StarlarkConfig struct {
	Enabled        bool  `yaml:"enabled"`          // Enable Starlark scripts
	MaxExecutionMs int64 `yaml:"max_execution_ms"` // Max script runtime
	MaxMemoryMB    int   `yaml:"max_memory_mb"`    // Max memory usage
}

// Default returns a config with sensible defaults
func Default() *Config {
	return &Config{
		Engine: EngineConfig{
			Type:     "opencode",
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
			Port:     4098,
		},
		Workspace: WorkspaceConfig{
			Path: "/workspace",
		},
		Discord: DiscordConfig{
			Enabled: true,
		},
		Telegram: TelegramConfig{Enabled: false},
		Slack:    SlackConfig{Enabled: false},
		Starlark: StarlarkConfig{
			Enabled:        true,
			MaxExecutionMs: 30000, // 30 seconds
			MaxMemoryMB:    128,
		},
		Logging: LoggingConfig{
			Level: "info",
			JSON:  false,
		},
		Server: ServerConfig{
			HealthAddr: ":8081",
			RateLimit: RateLimitConfig{
				Rate:  10,
				Burst: 20,
			},
		},
		Admin: AdminConfig{
			Enabled: true,
			Bind:    "localhost:8080",
			DevMode: true,
		},
	}
}

// Load reads config from file and environment variables.
// It first loads any .env file in the current directory, then reads
// the YAML config file, then applies environment variable overrides.
func Load() (*Config, error) {
	// Load .env file (real env vars take precedence)
	if err := LoadDotEnv(); err != nil {
		return nil, err
	}

	cfg := Default()

	// Apply workspace path override early so config file lookup uses it
	if v := os.Getenv("WORKSPACE_PATH"); v != "" {
		cfg.Workspace.Path = v
	}

	// Try to load from file
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = filepath.Join(cfg.Workspace.Path, "secure", "config.yaml")
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Override with environment variables (re-apply after config file may have changed them)
	if v := os.Getenv("ENGINE_TYPE"); v != "" {
		cfg.Engine.Type = v
	}
	if v := os.Getenv("WORKSPACE_PATH"); v != "" {
		cfg.Workspace.Path = v
	}
	if v := os.Getenv("ADMIN_BIND"); v != "" {
		cfg.Admin.Bind = v
	}

	return cfg, nil
}
