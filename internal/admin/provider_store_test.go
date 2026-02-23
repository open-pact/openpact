package admin

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewProviderStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewProviderStore(tmpDir)

	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.dataDir != tmpDir {
		t.Errorf("Expected dataDir %s, got %s", tmpDir, store.dataDir)
	}
}

func TestProviderStore_SetAndGet(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	cfg := ProviderConfig{
		Enabled:      true,
		AllowedUsers: []string{"user1", "user2"},
		AllowedChans: []string{"chan1"},
	}

	if err := store.Set("discord", cfg); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := store.Get("discord")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Name != "discord" {
		t.Errorf("Expected name 'discord', got %s", got.Name)
	}
	if !got.Enabled {
		t.Error("Expected enabled=true")
	}
	if len(got.AllowedUsers) != 2 {
		t.Errorf("Expected 2 allowed users, got %d", len(got.AllowedUsers))
	}
	if got.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestProviderStore_GetNotFound(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	_, err := store.Get("discord")
	if err != ErrProviderNotFound {
		t.Errorf("Expected ErrProviderNotFound, got %v", err)
	}
}

func TestProviderStore_InvalidName(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	err := store.Set("invalid_provider", ProviderConfig{})
	if err == nil {
		t.Error("Expected error for invalid provider name")
	}
}

func TestProviderStore_List(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	store.Set("discord", ProviderConfig{Enabled: true})
	store.Set("telegram", ProviderConfig{Enabled: false})

	configs, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("Expected 2 configs, got %d", len(configs))
	}
}

func TestProviderStore_ListEmpty(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	configs, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(configs) != 0 {
		t.Errorf("Expected 0 configs, got %d", len(configs))
	}
}

func TestProviderStore_Delete(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	store.Set("discord", ProviderConfig{Enabled: true})

	err := store.Delete("discord")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get("discord")
	if err != ErrProviderNotFound {
		t.Error("Expected provider to be deleted")
	}
}

func TestProviderStore_DeleteNotFound(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	err := store.Delete("discord")
	if err != ErrProviderNotFound {
		t.Errorf("Expected ErrProviderNotFound, got %v", err)
	}
}

func TestProviderStore_SetTokens(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	// Set tokens creates provider entry if needed
	err := store.SetTokens("discord", map[string]string{"token": "test-token-12345678"})
	if err != nil {
		t.Fatalf("SetTokens failed: %v", err)
	}

	cfg, err := store.Get("discord")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if cfg.Tokens["token"] != "test-token-12345678" {
		t.Errorf("Expected token 'test-token-12345678', got %s", cfg.Tokens["token"])
	}
}

func TestProviderStore_SetTokensMerge(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	store.SetTokens("slack", map[string]string{"bot_token": "xoxb-123"})
	store.SetTokens("slack", map[string]string{"app_token": "xapp-456"})

	cfg, _ := store.Get("slack")
	if cfg.Tokens["bot_token"] != "xoxb-123" {
		t.Errorf("Expected bot_token 'xoxb-123', got %s", cfg.Tokens["bot_token"])
	}
	if cfg.Tokens["app_token"] != "xapp-456" {
		t.Errorf("Expected app_token 'xapp-456', got %s", cfg.Tokens["app_token"])
	}
}

func TestProviderStore_SetPreservesTokens(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	store.SetTokens("discord", map[string]string{"token": "my-secret-token"})

	// Update config without tokens field
	store.Set("discord", ProviderConfig{
		Enabled:      true,
		AllowedUsers: []string{"user1"},
	})

	cfg, _ := store.Get("discord")
	if cfg.Tokens["token"] != "my-secret-token" {
		t.Errorf("Expected token to be preserved, got %s", cfg.Tokens["token"])
	}
	if !cfg.Enabled {
		t.Error("Expected enabled=true")
	}
}

func TestProviderStore_ResolveToken(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	// Stored token takes priority
	store.SetTokens("discord", map[string]string{"token": "stored-token"})
	os.Setenv("DISCORD_TOKEN", "env-token")
	defer os.Unsetenv("DISCORD_TOKEN")

	token := store.ResolveToken("discord", "token")
	if token != "stored-token" {
		t.Errorf("Expected 'stored-token', got '%s'", token)
	}
}

func TestProviderStore_ResolveTokenFallback(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	os.Setenv("DISCORD_TOKEN", "env-token-fallback")
	defer os.Unsetenv("DISCORD_TOKEN")

	token := store.ResolveToken("discord", "token")
	if token != "env-token-fallback" {
		t.Errorf("Expected 'env-token-fallback', got '%s'", token)
	}
}

func TestProviderStore_ResolveTokenNone(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	// Ensure env var is not set
	os.Unsetenv("DISCORD_TOKEN")

	token := store.ResolveToken("discord", "token")
	if token != "" {
		t.Errorf("Expected empty token, got '%s'", token)
	}
}

func TestProviderStore_HasStoredTokens(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	if store.HasStoredTokens("discord") {
		t.Error("Expected no stored tokens")
	}

	store.SetTokens("discord", map[string]string{"token": "abc"})

	if !store.HasStoredTokens("discord") {
		t.Error("Expected stored tokens")
	}
}

func TestProviderStore_HasEnvTokens(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	os.Unsetenv("DISCORD_TOKEN")
	if store.HasEnvTokens("discord") {
		t.Error("Expected no env tokens")
	}

	os.Setenv("DISCORD_TOKEN", "some-token")
	defer os.Unsetenv("DISCORD_TOKEN")

	if !store.HasEnvTokens("discord") {
		t.Error("Expected env tokens")
	}
}

func TestProviderStore_TokenHint(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	// No token
	hint := store.TokenHint("discord", "token")
	if hint != "" {
		t.Errorf("Expected empty hint, got '%s'", hint)
	}

	// Long token
	store.SetTokens("discord", map[string]string{"token": "abcdefghijklmnop"})
	hint = store.TokenHint("discord", "token")
	if hint != "...mnop" {
		t.Errorf("Expected '...mnop', got '%s'", hint)
	}

	// Short token
	store.SetTokens("telegram", map[string]string{"token": "short"})
	hint = store.TokenHint("telegram", "token")
	if hint != "...****" {
		t.Errorf("Expected '...****', got '%s'", hint)
	}
}

func TestProviderStore_TokenInfo(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	os.Unsetenv("DISCORD_TOKEN")

	info := store.TokenInfo("discord", "token")
	if info.HasToken || info.HasEnvToken {
		t.Error("Expected no tokens")
	}
	if info.TokenSource != "none" {
		t.Errorf("Expected source 'none', got '%s'", info.TokenSource)
	}

	store.SetTokens("discord", map[string]string{"token": "stored-token-12345678"})
	info = store.TokenInfo("discord", "token")
	if !info.HasToken {
		t.Error("Expected has_token=true")
	}
	if info.TokenSource != "store" {
		t.Errorf("Expected source 'store', got '%s'", info.TokenSource)
	}
	if info.TokenHint != "...5678" {
		t.Errorf("Expected hint '...5678', got '%s'", info.TokenHint)
	}
}

func TestProviderStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	store1 := NewProviderStore(tmpDir)
	store1.Set("discord", ProviderConfig{Enabled: true, AllowedUsers: []string{"u1"}})
	store1.SetTokens("discord", map[string]string{"token": "persist-me"})

	store2 := NewProviderStore(tmpDir)
	cfg, err := store2.Get("discord")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !cfg.Enabled {
		t.Error("Expected enabled=true after reload")
	}
	if cfg.Tokens["token"] != "persist-me" {
		t.Errorf("Expected token 'persist-me', got '%s'", cfg.Tokens["token"])
	}
}

func TestProviderStore_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewProviderStore(tmpDir)

	store.Set("discord", ProviderConfig{Enabled: true})

	info, err := os.Stat(filepath.Join(tmpDir, "chat_providers.json"))
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Expected permissions 0600, got %o", perm)
	}
}

func TestProviderStore_SeedFromConfig(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	providers := map[string]ProviderConfig{
		"discord": {
			Enabled:      true,
			AllowedUsers: []string{"user1"},
			AllowedChans: []string{"chan1"},
		},
		"telegram": {
			Enabled:      true,
			AllowedUsers: []string{"tguser1"},
		},
	}

	err := store.SeedFromConfig(providers)
	if err != nil {
		t.Fatalf("SeedFromConfig failed: %v", err)
	}

	cfg, _ := store.Get("discord")
	if !cfg.Enabled {
		t.Error("Expected discord enabled")
	}
	if len(cfg.AllowedUsers) != 1 || cfg.AllowedUsers[0] != "user1" {
		t.Errorf("Expected allowed users [user1], got %v", cfg.AllowedUsers)
	}

	// Seeding again should be a no-op
	providers["discord"] = ProviderConfig{Enabled: false}
	err = store.SeedFromConfig(providers)
	if err != nil {
		t.Fatalf("Second SeedFromConfig failed: %v", err)
	}

	cfg, _ = store.Get("discord")
	if !cfg.Enabled {
		t.Error("Expected discord still enabled (seed should be no-op)")
	}
}

func TestProviderStore_ConcurrentAccess(t *testing.T) {
	store := NewProviderStore(t.TempDir())

	var wg sync.WaitGroup
	iterations := 50

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			names := []string{"discord", "telegram", "slack"}
			name := names[n%3]
			store.Set(name, ProviderConfig{Enabled: n%2 == 0})
		}(i)
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Get("discord")
		}()
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.List()
		}()
	}

	wg.Wait()

	// Verify store is still functional
	err := store.Set("discord", ProviderConfig{Enabled: true})
	if err != nil {
		t.Fatalf("Store broken after concurrent access: %v", err)
	}
}

func TestRequiredTokenKeys(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		{"discord", 1},
		{"telegram", 1},
		{"slack", 2},
		{"unknown", 0},
	}

	for _, tt := range tests {
		keys := RequiredTokenKeys(tt.name)
		if len(keys) != tt.expected {
			t.Errorf("RequiredTokenKeys(%s): expected %d keys, got %d", tt.name, tt.expected, len(keys))
		}
	}
}
