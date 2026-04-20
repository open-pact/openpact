package secrets

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-pact/openpact/internal/storage"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	key := make([]byte, keySize)
	_, _ = rand.Read(key)
	s, err := NewStore(storage.NewTestDB(t), key)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestNewStore_RejectsWrongKeyLength(t *testing.T) {
	if _, err := NewStore(storage.NewTestDB(t), []byte("too short")); err == nil {
		t.Error("expected error for short key")
	}
}

func TestStore_SetGetRoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if err := s.Set(ctx, "API_KEY", "s3cret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := s.Get(ctx, "API_KEY")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "s3cret" {
		t.Errorf("Get = %q, want s3cret", got)
	}
}

func TestStore_Get_MissingReturnsSentinel(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), "NOPE")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("err = %v, want ErrSecretNotFound", err)
	}
}

func TestStore_Create_Duplicate(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.Create(ctx, "X", "v1"); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := s.Create(ctx, "X", "v2"); !errors.Is(err, ErrSecretExists) {
		t.Errorf("second Create = %v, want ErrSecretExists", err)
	}
	// Original value preserved.
	got, _ := s.Get(ctx, "X")
	if got != "v1" {
		t.Errorf("after duplicate Create = %q, want v1", got)
	}
}

func TestStore_Update(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Create(ctx, "X", "v1")
	if err := s.Update(ctx, "X", "v2"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := s.Get(ctx, "X")
	if got != "v2" {
		t.Errorf("after Update Get = %q, want v2", got)
	}

	if err := s.Update(ctx, "MISSING", "v"); !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("Update missing = %v, want ErrSecretNotFound", err)
	}
}

func TestStore_Delete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Create(ctx, "X", "v")
	if err := s.Delete(ctx, "X"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, "X"); !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("second Delete = %v, want ErrSecretNotFound", err)
	}
}

func TestStore_List_Sorted(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Set(ctx, "B", "b")
	_ = s.Set(ctx, "A", "a")
	_ = s.Set(ctx, "C", "c")

	entries, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	if entries[0].Name != "A" || entries[2].Name != "C" {
		t.Errorf("List order = %+v", entries)
	}
}

func TestStore_All(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Set(ctx, "A", "va")
	_ = s.Set(ctx, "B", "vb")

	all, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if all["A"] != "va" || all["B"] != "vb" {
		t.Errorf("All = %+v", all)
	}
}

func TestStore_ValidateName(t *testing.T) {
	s := newStore(t)
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"API_KEY", false},
		{"MY_SECRET_123", false},
		{"lowercase", true},
		{"123_STARTS_WITH_NUMBER", true},
		{"HAS SPACES", true},
		{"", true},
		{strings.Repeat("A", 65), true},
	}
	for _, c := range cases {
		err := s.Set(context.Background(), c.name, "value")
		if c.wantErr && err == nil {
			t.Errorf("Set(%q) expected error", c.name)
		}
		if !c.wantErr && err != nil {
			t.Errorf("Set(%q) unexpected error: %v", c.name, err)
		}
	}
}

func TestStore_ValueNotPlaintextOnDisk(t *testing.T) {
	// Sanity check: the stored value should not contain the plaintext.
	db := storage.NewTestDB(t)
	key := make([]byte, keySize)
	_, _ = rand.Read(key)
	s, _ := NewStore(db, key)

	if err := s.Set(context.Background(), "API_KEY", "the-real-plaintext-v1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var raw string
	if err := db.QueryRow(`SELECT value FROM op_secrets WHERE name = ?`, "API_KEY").Scan(&raw); err != nil {
		t.Fatalf("query raw: %v", err)
	}
	if strings.Contains(raw, "the-real-plaintext") {
		t.Errorf("stored value contains plaintext: %q", raw)
	}
}

func TestLoadOrCreateKey_PersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	k1, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("first LoadOrCreateKey: %v", err)
	}
	if len(k1) != keySize {
		t.Fatalf("key len = %d, want %d", len(k1), keySize)
	}

	k2, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("second LoadOrCreateKey: %v", err)
	}
	if string(k1) != string(k2) {
		t.Error("key changed across calls; expected stable")
	}

	// File should exist and be 0o600.
	info, err := os.Stat(filepath.Join(dir, keyFileName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o, want 0o600", info.Mode().Perm())
	}
}
