// Package chatproviders persists Discord/Slack/Telegram provider
// configuration in the op_chat_providers table. Tokens, allowed-user
// lists, and allowed-channel lists are stored as JSON columns —
// small per-provider records that aren't queried by individual
// allowed-user, so per-row normalisation would be churn.
//
// Tokens live inline (not in op_secrets) and are protected by the
// DB file's 0600 permission. A future encryption pass can move them
// into op_secrets using the same HKDF-from-data-encryption-key
// pattern the Starlark secrets use.
package chatproviders

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Errors. Unknown-provider errors come from the handler's validator,
// not here — the store accepts any name.
var (
	ErrProviderNotFound = errors.New("provider not found")
)

// Config is the persisted row shape. JSON tags match the wire format
// the admin UI already consumes so the handler just marshals this
// struct directly.
type Config struct {
	Name         string            `json:"name"`
	Enabled      bool              `json:"enabled"`
	Tokens       map[string]string `json:"tokens"`
	AllowedUsers []string          `json:"allowed_users"`
	AllowedChans []string          `json:"allowed_chans"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// TokenInfo describes token availability without exposing values.
// Kept on this package so callers don't have to import admin.
type TokenInfo struct {
	HasToken    bool   `json:"has_token"`
	HasEnvToken bool   `json:"has_env_token"`
	TokenSource string `json:"token_source"`
	TokenHint   string `json:"token_hint"`
}

// providerEnvKeys maps provider → token-key → env-var name. Same
// mapping as the old JSON-backed store; kept here so the chat
// provider logic (ResolveToken / HasEnvTokens) still knows where to
// look for env overrides.
var providerEnvKeys = map[string]map[string]string{
	"discord":  {"token": "DISCORD_TOKEN"},
	"telegram": {"token": "TELEGRAM_BOT_TOKEN"},
	"slack":    {"bot_token": "SLACK_BOT_TOKEN", "app_token": "SLACK_APP_TOKEN"},
}

// validProviderNames enumerates the providers the UI and orchestrator
// understand. An unknown provider in the DB is a bug somewhere else;
// the store itself doesn't enforce this.
var validProviderNames = map[string]bool{"discord": true, "telegram": true, "slack": true}

// Store is a thin handle around the shared *sql.DB for
// op_chat_providers.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over the given shared DB.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// List returns every provider row in insertion order. Empty slice
// (not nil) when no rows exist.
func (s *Store) List(ctx context.Context) ([]Config, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, enabled, tokens_json, allowed_users_json, allowed_chans_json,
		        created_at, updated_at
		   FROM op_chat_providers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("chatproviders: list: %w", err)
	}
	defer rows.Close()

	out := []Config{}
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get returns the config for a provider or ErrProviderNotFound.
func (s *Store) Get(ctx context.Context, name string) (Config, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT name, enabled, tokens_json, allowed_users_json, allowed_chans_json,
		        created_at, updated_at
		   FROM op_chat_providers WHERE name = ?`, name)
	c, err := scanConfig(row)
	if err == sql.ErrNoRows {
		return Config{}, ErrProviderNotFound
	}
	if err != nil {
		return Config{}, err
	}
	return c, nil
}

// Set upserts a provider's full config. If the provider exists, the
// CreatedAt is preserved; if it doesn't, it's set to now.
// Mimics the old SQL-less store's semantics.
func (s *Store) Set(ctx context.Context, name string, cfg Config) error {
	if !validProviderNames[name] {
		return fmt.Errorf("invalid provider name: %s", name)
	}

	now := time.Now().UTC()
	cfg.Name = name
	cfg.UpdatedAt = now
	if existing, err := s.Get(ctx, name); err == nil {
		cfg.CreatedAt = existing.CreatedAt
		if cfg.Tokens == nil {
			cfg.Tokens = existing.Tokens
		}
	} else {
		cfg.CreatedAt = now
	}
	if cfg.Tokens == nil {
		cfg.Tokens = map[string]string{}
	}
	if cfg.AllowedUsers == nil {
		cfg.AllowedUsers = []string{}
	}
	if cfg.AllowedChans == nil {
		cfg.AllowedChans = []string{}
	}

	return s.upsertRaw(ctx, cfg)
}

// SetTokens merges the given tokens into the provider's token map,
// auto-creating the provider row if needed (mirrors the old store's
// behaviour — the admin UI POSTs tokens before the full config).
func (s *Store) SetTokens(ctx context.Context, name string, tokens map[string]string) error {
	if !validProviderNames[name] {
		return fmt.Errorf("invalid provider name: %s", name)
	}

	cfg, err := s.Get(ctx, name)
	if err != nil && !errors.Is(err, ErrProviderNotFound) {
		return err
	}
	if errors.Is(err, ErrProviderNotFound) {
		now := time.Now().UTC()
		cfg = Config{
			Name:         name,
			Tokens:       map[string]string{},
			AllowedUsers: []string{},
			AllowedChans: []string{},
			CreatedAt:    now,
			UpdatedAt:    now,
		}
	}
	if cfg.Tokens == nil {
		cfg.Tokens = map[string]string{}
	}
	for k, v := range tokens {
		if v != "" {
			cfg.Tokens[k] = v
		}
	}
	cfg.UpdatedAt = time.Now().UTC()
	return s.upsertRaw(ctx, cfg)
}

// Delete removes a provider's row.
func (s *Store) Delete(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM op_chat_providers WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("chatproviders: delete %s: %w", name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrProviderNotFound
	}
	return nil
}

// ResolveToken returns the effective token for (provider, key).
// Stored tokens win over env vars.
func (s *Store) ResolveToken(ctx context.Context, name, key string) string {
	cfg, err := s.Get(ctx, name)
	if err == nil && cfg.Tokens != nil {
		if v, ok := cfg.Tokens[key]; ok && v != "" {
			return v
		}
	}
	return envToken(name, key)
}

// HasStoredTokens returns true if the provider has any non-empty
// tokens persisted.
func (s *Store) HasStoredTokens(ctx context.Context, name string) bool {
	cfg, err := s.Get(ctx, name)
	if err != nil {
		return false
	}
	for _, v := range cfg.Tokens {
		if v != "" {
			return true
		}
	}
	return false
}

// HasEnvTokens returns true if any of the provider's expected env vars
// is set and non-empty.
func (s *Store) HasEnvTokens(name string) bool {
	keys, ok := providerEnvKeys[name]
	if !ok {
		return false
	}
	for _, envVar := range keys {
		if os.Getenv(envVar) != "" {
			return true
		}
	}
	return false
}

// TokenHint returns the last 4 characters of the effective token,
// prefixed with "..." for display in the admin UI. Empty if no
// token is resolvable.
func (s *Store) TokenHint(ctx context.Context, name, key string) string {
	token := s.ResolveToken(ctx, name, key)
	if token == "" {
		return ""
	}
	if len(token) < 8 {
		return "...****"
	}
	return "..." + token[len(token)-4:]
}

// TokenInfoFor returns the aggregate availability shape the admin UI
// renders per-provider.
func (s *Store) TokenInfoFor(ctx context.Context, name, key string) TokenInfo {
	hasStored := false
	cfg, err := s.Get(ctx, name)
	if err == nil && cfg.Tokens != nil {
		if v, ok := cfg.Tokens[key]; ok && v != "" {
			hasStored = true
		}
	}
	hasEnv := false
	if keys, ok := providerEnvKeys[name]; ok {
		if envVar, ok := keys[key]; ok {
			hasEnv = os.Getenv(envVar) != ""
		}
	}
	source := "none"
	if hasStored {
		source = "store"
	} else if hasEnv {
		source = "env"
	}
	return TokenInfo{
		HasToken:    hasStored,
		HasEnvToken: hasEnv,
		TokenSource: source,
		TokenHint:   s.TokenHint(ctx, name, key),
	}
}

// RequiredTokenKeys returns the token keys a provider expects. Same
// list the old store exposed for setup-wizard validation.
func RequiredTokenKeys(name string) []string {
	switch name {
	case "discord":
		return []string{"token"}
	case "telegram":
		return []string{"token"}
	case "slack":
		return []string{"bot_token", "app_token"}
	default:
		return nil
	}
}

// --- internals ----------------------------------------------------------

func (s *Store) upsertRaw(ctx context.Context, c Config) error {
	tokens, _ := json.Marshal(c.Tokens)
	users, _ := json.Marshal(c.AllowedUsers)
	chans, _ := json.Marshal(c.AllowedChans)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO op_chat_providers
		    (name, enabled, tokens_json, allowed_users_json, allowed_chans_json, created_at, updated_at)
		    VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
		    enabled = excluded.enabled,
		    tokens_json = excluded.tokens_json,
		    allowed_users_json = excluded.allowed_users_json,
		    allowed_chans_json = excluded.allowed_chans_json,
		    updated_at = excluded.updated_at
	`, c.Name, boolToInt(c.Enabled), string(tokens), string(users), string(chans),
		c.CreatedAt.UTC().Format(time.RFC3339Nano),
		c.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("chatproviders: upsert %s: %w", c.Name, err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanConfig(r rowScanner) (Config, error) {
	var (
		c                                    Config
		enabled                              int
		tokensJSON, usersJSON, chansJSON     string
		createdAt, updatedAt                 string
	)
	err := r.Scan(&c.Name, &enabled, &tokensJSON, &usersJSON, &chansJSON, &createdAt, &updatedAt)
	if err != nil {
		return Config{}, err
	}
	c.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(tokensJSON), &c.Tokens)
	_ = json.Unmarshal([]byte(usersJSON), &c.AllowedUsers)
	_ = json.Unmarshal([]byte(chansJSON), &c.AllowedChans)
	if c.Tokens == nil {
		c.Tokens = map[string]string{}
	}
	if c.AllowedUsers == nil {
		c.AllowedUsers = []string{}
	}
	if c.AllowedChans == nil {
		c.AllowedChans = []string{}
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return c, nil
}

func envToken(name, key string) string {
	if keys, ok := providerEnvKeys[name]; ok {
		if envVar, ok := keys[key]; ok {
			return os.Getenv(envVar)
		}
	}
	return ""
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
