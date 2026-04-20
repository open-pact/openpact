// Package config holds OpenPact's bootstrap configuration — the
// minimal set of values that must be known BEFORE the shared SQLite
// database is opened. Everything else (logging level, rate limit,
// health-server address, Starlark limits, calendars, vault, GitHub,
// chat-provider tokens, allowed users/channels, admin users,
// schedules, secrets, setup state, channel sessions, channel modes)
// lives in op_* tables and is managed through the admin UI.
//
// Runtime-mutable fields deliberately do NOT appear here. Adding
// them back creates the class of bug the config-migration PR fixed:
// "I edited the UI but nothing happened because the process still
// reads the YAML."
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds OpenPact's bootstrap configuration.
type Config struct {
	Engine    EngineConfig    `yaml:"engine"`
	Workspace WorkspaceConfig `yaml:"workspace"`
	Admin     AdminConfig     `yaml:"admin"`

	// Runtime config read from the database on boot. These fields are
	// NOT serialized to/from YAML — they're populated by cmd/openpact
	// after opening the DB and loading advanced_settings. The struct
	// still carries them because the scheduler + MCP script tool
	// consume the values through `*Config` before the orchestrator
	// exists to pass them around directly.
	Starlark StarlarkRuntime `yaml:"-"`
}

// AdminConfig configures the admin web UI.
type AdminConfig struct {
	Enabled   bool     `yaml:"enabled"`   // Enable admin UI
	Bind      string   `yaml:"bind"`      // Address to bind (e.g., "localhost:8080")
	Allowlist []string `yaml:"allowlist"` // Always-approved scripts
}

// EngineConfig configures the AI engine. Provider credentials, the
// default model, and every other runtime setting live in stackllm's
// own stores under <workspace>/secure/data/ or OpenPact's op_* tables;
// only the SQLite database path is surfaced here, and only as an
// optional override — the default is <workspace>/secure/data/stackllm.db.
type EngineConfig struct {
	DBPath string `yaml:"db_path,omitempty"` // Optional override for the stackllm SQLite path
}

// WorkspaceConfig configures workspace paths.
type WorkspaceConfig struct {
	Path string `yaml:"path"` // Base workspace path
}

// StarlarkRuntime mirrors advanced_settings.starlark for the packages
// that still read it off *Config (scheduler, MCP script tool). Values
// are populated in main.go from the DB — not from YAML.
type StarlarkRuntime struct {
	Enabled        bool
	MaxExecutionMs int64
	MaxMemoryMB    int
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

// EnsureDirs creates all required workspace directories if they
// don't exist. secure/ and secure/data/ are 0700 because they hold
// the DB, encryption keys, JWT secret, stackllm auth file. ai-data/
// and children are 0755 so the orchestrator can read SOUL/USER/MEMORY
// and write memory rolls.
func (w WorkspaceConfig) EnsureDirs() error {
	tight := []string{w.SecureDir(), w.DataDir()}
	loose := []string{
		w.Path,
		w.AIDataDir(),
		w.ScriptsDir(),
		filepath.Join(w.AIDataDir(), "memory"),
		filepath.Join(w.AIDataDir(), "skills"),
	}
	for _, dir := range tight {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	for _, dir := range loose {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// Default returns a config with sensible bootstrap defaults. Runtime
// settings (logging level, starlark limits, etc.) are not covered
// here — the DB-backed stores own their own defaults.
func Default() *Config {
	return &Config{
		Engine:    EngineConfig{},
		Workspace: WorkspaceConfig{Path: "/workspace"},
		Admin:     AdminConfig{Enabled: true, Bind: "localhost:8080"},
	}
}

// Load reads config from file and environment variables. Loads any
// .env in the current directory, reads the YAML config file, then
// applies bootstrap env-var overrides (WORKSPACE_PATH, ADMIN_BIND).
func Load() (*Config, error) {
	if err := LoadDotEnv(); err != nil {
		return nil, err
	}

	cfg := Default()

	// Apply workspace path override early so config file lookup uses it.
	if v := os.Getenv("WORKSPACE_PATH"); v != "" {
		cfg.Workspace.Path = v
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = filepath.Join(cfg.Workspace.Path, "secure", "config.yaml")
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	if v := os.Getenv("WORKSPACE_PATH"); v != "" {
		cfg.Workspace.Path = v
	}
	if v := os.Getenv("ADMIN_BIND"); v != "" {
		cfg.Admin.Bind = v
	}

	return cfg, nil
}
