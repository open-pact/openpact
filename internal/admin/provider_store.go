package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrProviderNotFound = errors.New("provider not found")
	validProviderNames  = map[string]bool{"discord": true, "telegram": true, "slack": true}
)

// ProviderConfig is the stored configuration for a chat provider.
type ProviderConfig struct {
	Name         string            `json:"name"`
	Enabled      bool              `json:"enabled"`
	Tokens       map[string]string `json:"tokens"`
	AllowedUsers []string          `json:"allowed_users"`
	AllowedChans []string          `json:"allowed_chans"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ProviderTokenInfo describes token availability without exposing values.
type ProviderTokenInfo struct {
	HasToken    bool   `json:"has_token"`
	HasEnvToken bool   `json:"has_env_token"`
	TokenSource string `json:"token_source"` // "store", "env", "none"
	TokenHint   string `json:"token_hint"`   // last 4 chars, e.g. "...x7kQ"
}

// providerEnvKeys maps provider name -> token key -> env var name.
var providerEnvKeys = map[string]map[string]string{
	"discord":  {"token": "DISCORD_TOKEN"},
	"telegram": {"token": "TELEGRAM_BOT_TOKEN"},
	"slack":    {"bot_token": "SLACK_BOT_TOKEN", "app_token": "SLACK_APP_TOKEN"},
}

// providerFile is the on-disk JSON format.
type providerFile struct {
	Providers map[string]ProviderConfig `json:"providers"`
}

// ProviderStore manages provider config persistence.
type ProviderStore struct {
	dataDir string
	mu      sync.RWMutex
}

// NewProviderStore creates a new provider store.
func NewProviderStore(dataDir string) *ProviderStore {
	return &ProviderStore{dataDir: dataDir}
}

func (s *ProviderStore) filePath() string {
	return filepath.Join(s.dataDir, "chat_providers.json")
}

func (s *ProviderStore) load() (*providerFile, error) {
	pf := &providerFile{Providers: make(map[string]ProviderConfig)}

	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return pf, nil
		}
		return nil, fmt.Errorf("failed to read providers: %w", err)
	}

	if err := json.Unmarshal(data, pf); err != nil {
		return nil, fmt.Errorf("failed to parse providers: %w", err)
	}

	if pf.Providers == nil {
		pf.Providers = make(map[string]ProviderConfig)
	}

	return pf, nil
}

func (s *ProviderStore) save(pf *providerFile) error {
	if err := os.MkdirAll(s.dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal providers: %w", err)
	}

	if err := os.WriteFile(s.filePath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write providers: %w", err)
	}

	return nil
}

// List returns all provider configs.
func (s *ProviderStore) List() ([]ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pf, err := s.load()
	if err != nil {
		return nil, err
	}

	configs := make([]ProviderConfig, 0, len(pf.Providers))
	for _, cfg := range pf.Providers {
		configs = append(configs, cfg)
	}
	return configs, nil
}

// Get returns config for a single provider.
func (s *ProviderStore) Get(name string) (ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pf, err := s.load()
	if err != nil {
		return ProviderConfig{}, err
	}

	cfg, ok := pf.Providers[name]
	if !ok {
		return ProviderConfig{}, ErrProviderNotFound
	}

	return cfg, nil
}

// Set creates or updates a provider config.
func (s *ProviderStore) Set(name string, cfg ProviderConfig) error {
	if !validProviderNames[name] {
		return fmt.Errorf("invalid provider name: %s", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pf, err := s.load()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	existing, exists := pf.Providers[name]
	cfg.Name = name
	cfg.UpdatedAt = now
	if exists {
		cfg.CreatedAt = existing.CreatedAt
		// Preserve tokens if not provided in the update
		if cfg.Tokens == nil {
			cfg.Tokens = existing.Tokens
		}
	} else {
		cfg.CreatedAt = now
	}
	if cfg.Tokens == nil {
		cfg.Tokens = make(map[string]string)
	}
	if cfg.AllowedUsers == nil {
		cfg.AllowedUsers = []string{}
	}
	if cfg.AllowedChans == nil {
		cfg.AllowedChans = []string{}
	}

	pf.Providers[name] = cfg
	return s.save(pf)
}

// SetTokens sets tokens for a provider (merge, not replace).
func (s *ProviderStore) SetTokens(name string, tokens map[string]string) error {
	if !validProviderNames[name] {
		return fmt.Errorf("invalid provider name: %s", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pf, err := s.load()
	if err != nil {
		return err
	}

	cfg, ok := pf.Providers[name]
	if !ok {
		// Auto-create provider entry when setting tokens
		now := time.Now().UTC()
		cfg = ProviderConfig{
			Name:         name,
			Tokens:       make(map[string]string),
			AllowedUsers: []string{},
			AllowedChans: []string{},
			CreatedAt:    now,
			UpdatedAt:    now,
		}
	}

	if cfg.Tokens == nil {
		cfg.Tokens = make(map[string]string)
	}
	for k, v := range tokens {
		if v != "" {
			cfg.Tokens[k] = v
		}
	}
	cfg.UpdatedAt = time.Now().UTC()

	pf.Providers[name] = cfg
	return s.save(pf)
}

// Delete removes a provider config.
func (s *ProviderStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pf, err := s.load()
	if err != nil {
		return err
	}

	if _, ok := pf.Providers[name]; !ok {
		return ErrProviderNotFound
	}

	delete(pf.Providers, name)
	return s.save(pf)
}

// ResolveToken returns the effective token value for a provider key.
// Stored tokens take priority over env vars.
func (s *ProviderStore) ResolveToken(name, key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pf, err := s.load()
	if err != nil {
		return s.envToken(name, key)
	}

	cfg, ok := pf.Providers[name]
	if ok && cfg.Tokens != nil {
		if v, exists := cfg.Tokens[key]; exists && v != "" {
			return v
		}
	}

	return s.envToken(name, key)
}

func (s *ProviderStore) envToken(name, key string) string {
	if keys, ok := providerEnvKeys[name]; ok {
		if envVar, ok := keys[key]; ok {
			return os.Getenv(envVar)
		}
	}
	return ""
}

// HasStoredTokens returns true if the provider has any stored tokens.
func (s *ProviderStore) HasStoredTokens(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pf, err := s.load()
	if err != nil {
		return false
	}

	cfg, ok := pf.Providers[name]
	if !ok {
		return false
	}

	for _, v := range cfg.Tokens {
		if v != "" {
			return true
		}
	}
	return false
}

// HasEnvTokens returns true if the provider has any env var tokens available.
func (s *ProviderStore) HasEnvTokens(name string) bool {
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

// TokenHint returns the last 4 characters of a token value for display.
func (s *ProviderStore) TokenHint(name, key string) string {
	token := s.ResolveToken(name, key)
	if token == "" {
		return ""
	}
	if len(token) < 8 {
		return "...****"
	}
	return "..." + token[len(token)-4:]
}

// TokenInfo returns token metadata for a specific provider and key.
func (s *ProviderStore) TokenInfo(name, key string) ProviderTokenInfo {
	hasStored := false
	s.mu.RLock()
	pf, err := s.load()
	if err == nil {
		if cfg, ok := pf.Providers[name]; ok && cfg.Tokens != nil {
			if v, exists := cfg.Tokens[key]; exists && v != "" {
				hasStored = true
			}
		}
	}
	s.mu.RUnlock()

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

	return ProviderTokenInfo{
		HasToken:    hasStored,
		HasEnvToken: hasEnv,
		TokenSource: source,
		TokenHint:   s.TokenHint(name, key),
	}
}

// SeedFromConfig initializes the store with config values if no stored config exists.
// This is a one-time migration from YAML config to the store.
func (s *ProviderStore) SeedFromConfig(providers map[string]ProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pf, err := s.load()
	if err != nil {
		return err
	}

	// Only seed if the store is empty
	if len(pf.Providers) > 0 {
		return nil
	}

	now := time.Now().UTC()
	for name, cfg := range providers {
		cfg.Name = name
		cfg.CreatedAt = now
		cfg.UpdatedAt = now
		if cfg.Tokens == nil {
			cfg.Tokens = make(map[string]string)
		}
		if cfg.AllowedUsers == nil {
			cfg.AllowedUsers = []string{}
		}
		if cfg.AllowedChans == nil {
			cfg.AllowedChans = []string{}
		}
		pf.Providers[name] = cfg
	}

	return s.save(pf)
}

// RequiredTokenKeys returns the token keys required for a provider.
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
