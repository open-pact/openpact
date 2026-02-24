package admin

import (
	"os"
	"testing"
)

func TestModelPreferenceStore_GetReturnsNilWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	store := NewModelPreferenceStore(dir)

	pref, err := store.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pref != nil {
		t.Errorf("expected nil preference, got %+v", pref)
	}
}

func TestModelPreferenceStore_SetAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewModelPreferenceStore(dir)

	if err := store.Set("anthropic", "claude-opus-4-20250514"); err != nil {
		t.Fatalf("unexpected error on Set: %v", err)
	}

	pref, err := store.Get()
	if err != nil {
		t.Fatalf("unexpected error on Get: %v", err)
	}
	if pref == nil {
		t.Fatal("expected non-nil preference")
	}
	if pref.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", pref.Provider)
	}
	if pref.Model != "claude-opus-4-20250514" {
		t.Errorf("expected model 'claude-opus-4-20250514', got '%s'", pref.Model)
	}
	if pref.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestModelPreferenceStore_SetOverwrites(t *testing.T) {
	dir := t.TempDir()
	store := NewModelPreferenceStore(dir)

	if err := store.Set("anthropic", "claude-sonnet-4-20250514"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := store.Set("openai", "gpt-4o"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pref, err := store.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pref.Provider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", pref.Provider)
	}
	if pref.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got '%s'", pref.Model)
	}
}

func TestModelPreferenceStore_GetWithCorruptFile(t *testing.T) {
	dir := t.TempDir()
	store := NewModelPreferenceStore(dir)

	if err := os.WriteFile(store.filePath(), []byte("not json"), 0600); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	_, err := store.Get()
	if err == nil {
		t.Error("expected error for corrupt file")
	}
}

func TestModelPreferenceStore_SetCreatesDir(t *testing.T) {
	dir := t.TempDir() + "/nested/dir"
	store := NewModelPreferenceStore(dir)

	if err := store.Set("anthropic", "claude-opus-4-20250514"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pref, err := store.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pref == nil {
		t.Fatal("expected non-nil preference")
	}
}
