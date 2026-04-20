package kv

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

// --- Advanced settings ---------------------------------------------------

// AdvancedSettings bundles the knobs that sit under the "Advanced
// settings" admin page. JSON tags match the payload the frontend has
// been consuming since this page shipped — keep them stable on the
// wire even when schema details change under the hood.
type AdvancedSettings struct {
	Logging   LoggingSettings   `json:"logging"`
	RateLimit RateLimitSettings `json:"rate_limit"`
	Server    ServerSettings    `json:"server"`
	Starlark  StarlarkSettings  `json:"starlark"`
}

type LoggingSettings struct {
	Level string `json:"level"` // debug / info / warn / error
	JSON  bool   `json:"json"`
}

type RateLimitSettings struct {
	Rate  float64 `json:"rate"`  // requests per second
	Burst int     `json:"burst"` // max burst
}

type ServerSettings struct {
	HealthAddr string `json:"health_addr"` // e.g. ":8081"
}

type StarlarkSettings struct {
	Enabled        bool  `json:"enabled"`
	MaxExecutionMs int64 `json:"max_execution_ms"`
	MaxMemoryMB    int   `json:"max_memory_mb"`
}

// DefaultAdvancedSettings returns the values used when the DB has no
// rows in scope=advanced_settings. Also used as the fallback for
// individual missing keys during Load — if only logging.level is
// persisted, the other logging fields come from here.
func DefaultAdvancedSettings() AdvancedSettings {
	return AdvancedSettings{
		Logging:   LoggingSettings{Level: "info", JSON: false},
		RateLimit: RateLimitSettings{Rate: 10, Burst: 20},
		Server:    ServerSettings{HealthAddr: ":8081"},
		Starlark:  StarlarkSettings{Enabled: true, MaxExecutionMs: 30000, MaxMemoryMB: 128},
	}
}

// LoadAdvancedSettings reads every scope=advanced_settings row and
// assembles an AdvancedSettings, filling missing keys from defaults.
func LoadAdvancedSettings(ctx context.Context, db *sql.DB) (AdvancedSettings, error) {
	store := NewStore(db)
	rows, err := store.GetAll(ctx, ScopeAdvancedSettings)
	if err != nil {
		return AdvancedSettings{}, err
	}

	out := DefaultAdvancedSettings()
	getStr := func(k, fallback string) string {
		if v, ok := rows[k]; ok {
			return v
		}
		return fallback
	}
	getBool := func(k string, fallback bool) bool {
		if v, ok := rows[k]; ok {
			b, _ := strconv.ParseBool(v)
			return b
		}
		return fallback
	}
	getInt := func(k string, fallback int64) int64 {
		if v, ok := rows[k]; ok {
			i, _ := strconv.ParseInt(v, 10, 64)
			return i
		}
		return fallback
	}
	getFloat := func(k string, fallback float64) float64 {
		if v, ok := rows[k]; ok {
			f, _ := strconv.ParseFloat(v, 64)
			return f
		}
		return fallback
	}

	out.Logging.Level = getStr("logging.level", out.Logging.Level)
	out.Logging.JSON = getBool("logging.json", out.Logging.JSON)
	out.RateLimit.Rate = getFloat("rate_limit.rate", out.RateLimit.Rate)
	out.RateLimit.Burst = int(getInt("rate_limit.burst", int64(out.RateLimit.Burst)))
	out.Server.HealthAddr = getStr("server.health_addr", out.Server.HealthAddr)
	out.Starlark.Enabled = getBool("starlark.enabled", out.Starlark.Enabled)
	out.Starlark.MaxExecutionMs = getInt("starlark.max_execution_ms", out.Starlark.MaxExecutionMs)
	out.Starlark.MaxMemoryMB = int(getInt("starlark.max_memory_mb", int64(out.Starlark.MaxMemoryMB)))
	return out, nil
}

// SaveAdvancedSettings replaces every row in scope=advanced_settings
// with the flattened pairs derived from s. Transactional — a partial
// failure leaves the DB unchanged.
func SaveAdvancedSettings(ctx context.Context, db *sql.DB, s AdvancedSettings) error {
	pairs := map[string]string{
		"logging.level":            s.Logging.Level,
		"logging.json":             strconv.FormatBool(s.Logging.JSON),
		"rate_limit.rate":          strconv.FormatFloat(s.RateLimit.Rate, 'g', -1, 64),
		"rate_limit.burst":         strconv.Itoa(s.RateLimit.Burst),
		"server.health_addr":       s.Server.HealthAddr,
		"starlark.enabled":         strconv.FormatBool(s.Starlark.Enabled),
		"starlark.max_execution_ms": strconv.FormatInt(s.Starlark.MaxExecutionMs, 10),
		"starlark.max_memory_mb":   strconv.Itoa(s.Starlark.MaxMemoryMB),
	}
	return NewStore(db).ReplaceScope(ctx, ScopeAdvancedSettings, pairs)
}

// --- Integrations (scalar parts) -----------------------------------------

// VaultSettings is the Obsidian-vault config surfaced by the
// Integrations admin page.
type VaultSettings struct {
	Path     string `json:"path"`
	GitRepo  string `json:"git_repo"`
	AutoSync bool   `json:"auto_sync"`
}

// GitHubSettings is the GitHub integration flag. The token itself
// lives in op_secrets under a fixed name; nothing sensitive goes in
// op_kv.
type GitHubSettings struct {
	Enabled bool `json:"enabled"`
}

// IntegrationScalars holds every settings.integrations.* scalar —
// calendars (a list) come from a separate store.
type IntegrationScalars struct {
	Vault  VaultSettings  `json:"vault"`
	GitHub GitHubSettings `json:"github"`
}

// DefaultIntegrationScalars returns the zero-state values. Both
// sub-blocks are effectively "off" by default.
func DefaultIntegrationScalars() IntegrationScalars {
	return IntegrationScalars{
		Vault:  VaultSettings{},
		GitHub: GitHubSettings{},
	}
}

// LoadIntegrationScalars reads every scope=integrations row (except
// the calendar rows — those live in op_calendars, not here).
func LoadIntegrationScalars(ctx context.Context, db *sql.DB) (IntegrationScalars, error) {
	store := NewStore(db)
	rows, err := store.GetAll(ctx, ScopeIntegrations)
	if err != nil {
		return IntegrationScalars{}, err
	}

	out := DefaultIntegrationScalars()
	out.Vault.Path = rows["vault.path"]
	out.Vault.GitRepo = rows["vault.git_repo"]
	if v, ok := rows["vault.auto_sync"]; ok {
		b, _ := strconv.ParseBool(v)
		out.Vault.AutoSync = b
	}
	if v, ok := rows["github.enabled"]; ok {
		b, _ := strconv.ParseBool(v)
		out.GitHub.Enabled = b
	}
	return out, nil
}

// SaveIntegrationScalars replaces the scalar portion of the
// integrations scope. Calendar rows are untouched — they live in
// op_calendars and the calendars store is the source of truth for
// them. To keep this simple we delete-then-insert only the keys we
// own; a full ReplaceScope would drop calendar rows if any ever
// ended up in op_kv by mistake.
func SaveIntegrationScalars(ctx context.Context, db *sql.DB, s IntegrationScalars) error {
	store := NewStore(db)
	pairs := map[string]string{
		"vault.path":       s.Vault.Path,
		"vault.git_repo":   s.Vault.GitRepo,
		"vault.auto_sync":  strconv.FormatBool(s.Vault.AutoSync),
		"github.enabled":   strconv.FormatBool(s.GitHub.Enabled),
	}
	for k, v := range pairs {
		if err := store.Set(ctx, ScopeIntegrations, k, v); err != nil {
			return fmt.Errorf("kv: save integrations %s: %w", k, err)
		}
	}
	return nil
}

// --- Setup state ---------------------------------------------------------

// SetupState is the multi-step setup wizard's progress flags.
type SetupState struct {
	ProfileComplete  bool `json:"profile_complete"`
	ProviderComplete bool `json:"provider_complete"`
}

// LoadSetupState reads both flags from scope=setup_state. Missing
// rows are treated as false — the wizard hasn't been through that
// step yet.
func LoadSetupState(ctx context.Context, db *sql.DB) (SetupState, error) {
	store := NewStore(db)
	rows, err := store.GetAll(ctx, ScopeSetupState)
	if err != nil {
		return SetupState{}, err
	}
	var s SetupState
	if v, ok := rows["profile_complete"]; ok {
		b, _ := strconv.ParseBool(v)
		s.ProfileComplete = b
	}
	if v, ok := rows["provider_complete"]; ok {
		b, _ := strconv.ParseBool(v)
		s.ProviderComplete = b
	}
	return s, nil
}

// SaveSetupState upserts both flags. Not a ReplaceScope — so adding a
// third setup step in the future doesn't silently wipe old rows.
func SaveSetupState(ctx context.Context, db *sql.DB, s SetupState) error {
	store := NewStore(db)
	if err := store.Set(ctx, ScopeSetupState, "profile_complete", strconv.FormatBool(s.ProfileComplete)); err != nil {
		return err
	}
	if err := store.Set(ctx, ScopeSetupState, "provider_complete", strconv.FormatBool(s.ProviderComplete)); err != nil {
		return err
	}
	return nil
}
