package users

import (
	"context"
	"errors"
	"testing"

	"github.com/open-pact/openpact/internal/storage"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(storage.NewTestDB(t))
}

func TestStore_CreateAndGet(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	u, err := s.Create(ctx, "alice", "password12345678")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("Username = %q, want alice", u.Username)
	}
	if u.PasswordHash == "" {
		t.Error("PasswordHash should not be empty")
	}
	if u.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	got, err := s.Get(ctx, "alice")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Username != u.Username || got.PasswordHash != u.PasswordHash {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, u)
	}
}

func TestStore_CreateDuplicate(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	_, err := s.Create(ctx, "alice", "password12345678")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err = s.Create(ctx, "alice", "password12345678")
	if !errors.Is(err, ErrUserExists) {
		t.Errorf("second Create err = %v, want ErrUserExists", err)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), "nobody")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestStore_ValidateSuccess(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	_, err := s.Create(ctx, "alice", "password12345678")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	u, err := s.Validate(ctx, "alice", "password12345678")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("Validate returned wrong user: %+v", u)
	}
	if u.LastLoginAt.IsZero() {
		t.Error("LastLoginAt should be set after successful Validate")
	}

	// Confirm the LastLoginAt was persisted.
	got, _ := s.Get(ctx, "alice")
	if got.LastLoginAt.IsZero() {
		t.Error("LastLoginAt not persisted to DB")
	}
}

func TestStore_ValidateWrongPassword(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	_, _ = s.Create(ctx, "alice", "password12345678")
	_, err := s.Validate(ctx, "alice", "wrong password 12345")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("err = %v, want ErrInvalidPassword", err)
	}
}

func TestStore_ValidateMissingUser(t *testing.T) {
	s := newStore(t)
	_, err := s.Validate(context.Background(), "nobody", "anything123456789")
	// Missing user collapses to ErrInvalidPassword to defeat timing.
	if !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("err = %v, want ErrInvalidPassword", err)
	}
}

func TestStore_CountAndHasUsers(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if s.HasUsers(ctx) {
		t.Error("empty store should have no users")
	}
	if n, _ := s.Count(ctx); n != 0 {
		t.Errorf("empty Count = %d, want 0", n)
	}

	_, _ = s.Create(ctx, "a", "password12345678")
	_, _ = s.Create(ctx, "b", "password12345678")

	if !s.HasUsers(ctx) {
		t.Error("HasUsers should be true after Create")
	}
	if n, _ := s.Count(ctx); n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		pw      string
		wantErr bool
	}{
		{"short", true},
		{"tooshort1234", true},                // 12 chars, only 2 categories
		{"Ok1short12345", false},              // 13 chars, 3 categories (upper+lower+number) - 13>=12
		{"averylongpassphrase", false},        // 20 chars
		{"", true},
	}
	for _, c := range cases {
		err := ValidatePassword(c.pw)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidatePassword(%q) err=%v, wantErr=%v", c.pw, err, c.wantErr)
		}
	}
}

func TestValidatePasswords_Match(t *testing.T) {
	if err := ValidatePasswords("passphrase_goodlen", "passphrase_goodlen"); err != nil {
		t.Errorf("match path: %v", err)
	}
	if err := ValidatePasswords("passphrase_goodlen", "different_17_chars"); !errors.Is(err, ErrPasswordMismatch) {
		t.Errorf("mismatch path: %v", err)
	}
}
