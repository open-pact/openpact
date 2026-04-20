package kv

import (
	"context"
	"testing"

	"github.com/open-pact/openpact/internal/storage"
)

func TestStore_SetGet(t *testing.T) {
	db := storage.NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	// Missing key reports ok=false, not an error.
	if _, ok, err := s.Get(ctx, "advanced_settings", "logging.level"); err != nil || ok {
		t.Fatalf("Get missing: err=%v ok=%v", err, ok)
	}

	if err := s.Set(ctx, "advanced_settings", "logging.level", "debug"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok, err := s.Get(ctx, "advanced_settings", "logging.level")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok || v != "debug" {
		t.Errorf("Get = %q/%v, want debug/true", v, ok)
	}

	// Overwrite
	if err := s.Set(ctx, "advanced_settings", "logging.level", "warn"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	v, _, _ = s.Get(ctx, "advanced_settings", "logging.level")
	if v != "warn" {
		t.Errorf("after overwrite Get = %q, want warn", v)
	}
}

func TestStore_GetAll(t *testing.T) {
	db := storage.NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_ = s.Set(ctx, "advanced_settings", "a", "1")
	_ = s.Set(ctx, "advanced_settings", "b", "2")
	_ = s.Set(ctx, "other_scope", "c", "3")

	rows, err := s.GetAll(ctx, "advanced_settings")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(rows) != 2 || rows["a"] != "1" || rows["b"] != "2" {
		t.Errorf("GetAll = %v, want {a:1 b:2}", rows)
	}
}

func TestStore_ReplaceScope_DeletesStaleKeys(t *testing.T) {
	db := storage.NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_ = s.Set(ctx, "scope1", "old", "gone")
	_ = s.Set(ctx, "scope1", "keep", "old_value")
	_ = s.Set(ctx, "scope2", "untouched", "ok")

	err := s.ReplaceScope(ctx, "scope1", map[string]string{
		"keep": "new_value",
		"fresh": "yes",
	})
	if err != nil {
		t.Fatalf("ReplaceScope: %v", err)
	}

	rows, _ := s.GetAll(ctx, "scope1")
	if _, ok := rows["old"]; ok {
		t.Error("old key should be deleted")
	}
	if rows["keep"] != "new_value" || rows["fresh"] != "yes" {
		t.Errorf("rows = %v", rows)
	}

	// Other scopes untouched.
	other, _ := s.GetAll(ctx, "scope2")
	if other["untouched"] != "ok" {
		t.Error("unrelated scope was modified")
	}
}

func TestLoadAdvancedSettings_DefaultsOnEmpty(t *testing.T) {
	db := storage.NewTestDB(t)
	s, err := LoadAdvancedSettings(context.Background(), db)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Logging.Level != "info" {
		t.Errorf("Level = %q, want info", s.Logging.Level)
	}
	if s.RateLimit.Rate != 10 {
		t.Errorf("Rate = %v, want 10", s.RateLimit.Rate)
	}
	if s.RateLimit.Burst != 20 {
		t.Errorf("Burst = %v, want 20", s.RateLimit.Burst)
	}
	if s.Server.HealthAddr != ":8081" {
		t.Errorf("HealthAddr = %q, want :8081", s.Server.HealthAddr)
	}
	if !s.Starlark.Enabled || s.Starlark.MaxExecutionMs != 30000 || s.Starlark.MaxMemoryMB != 128 {
		t.Errorf("Starlark = %+v, want defaults", s.Starlark)
	}
}

func TestAdvancedSettings_RoundTrip(t *testing.T) {
	db := storage.NewTestDB(t)
	ctx := context.Background()

	in := AdvancedSettings{
		Logging:   LoggingSettings{Level: "debug", JSON: true},
		RateLimit: RateLimitSettings{Rate: 5.5, Burst: 50},
		Server:    ServerSettings{HealthAddr: "127.0.0.1:9999"},
		Starlark:  StarlarkSettings{Enabled: false, MaxExecutionMs: 12345, MaxMemoryMB: 256},
	}
	if err := SaveAdvancedSettings(ctx, db, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := LoadAdvancedSettings(ctx, db)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != in {
		t.Errorf("round-trip mismatch\n got %+v\nwant %+v", got, in)
	}
}

func TestIntegrationScalars_RoundTrip(t *testing.T) {
	db := storage.NewTestDB(t)
	ctx := context.Background()

	in := IntegrationScalars{
		Vault:  VaultSettings{Path: "/home/user/vault", GitRepo: "git@github.com:u/v.git", AutoSync: true},
		GitHub: GitHubSettings{Enabled: true},
	}
	if err := SaveIntegrationScalars(ctx, db, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := LoadIntegrationScalars(ctx, db)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != in {
		t.Errorf("round-trip mismatch\n got %+v\nwant %+v", got, in)
	}
}

func TestSetupState_RoundTrip(t *testing.T) {
	db := storage.NewTestDB(t)
	ctx := context.Background()

	got, err := LoadSetupState(ctx, db)
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if got != (SetupState{}) {
		t.Errorf("empty Load = %+v, want zero", got)
	}

	if err := SaveSetupState(ctx, db, SetupState{ProfileComplete: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ = LoadSetupState(ctx, db)
	if !got.ProfileComplete || got.ProviderComplete {
		t.Errorf("Load after Save = %+v, want {profile:true provider:false}", got)
	}

	if err := SaveSetupState(ctx, db, SetupState{ProfileComplete: true, ProviderComplete: true}); err != nil {
		t.Fatalf("Save full: %v", err)
	}
	got, _ = LoadSetupState(ctx, db)
	if !got.ProfileComplete || !got.ProviderComplete {
		t.Errorf("Load after full Save = %+v, want both true", got)
	}
}
