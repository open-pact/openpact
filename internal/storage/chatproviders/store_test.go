package chatproviders

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

func TestStore_SetAndGet(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	err := s.Set(ctx, "discord", Config{
		Enabled:      true,
		Tokens:       map[string]string{"token": "t1"},
		AllowedUsers: []string{"u1", "u2"},
		AllowedChans: []string{"c1"},
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := s.Get(ctx, "discord")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Enabled || got.Tokens["token"] != "t1" {
		t.Errorf("Get = %+v", got)
	}
	if len(got.AllowedUsers) != 2 || got.AllowedUsers[0] != "u1" {
		t.Errorf("AllowedUsers: %v", got.AllowedUsers)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), "discord")
	if !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("err = %v, want ErrProviderNotFound", err)
	}
}

func TestStore_List(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for _, name := range []string{"slack", "discord", "telegram"} {
		if err := s.Set(ctx, name, Config{Enabled: false}); err != nil {
			t.Fatalf("Set %s: %v", name, err)
		}
	}

	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
}

func TestStore_SetTokens_Merges(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	_ = s.Set(ctx, "slack", Config{
		Enabled: true,
		Tokens:  map[string]string{"bot_token": "b1", "app_token": "a1"},
	})

	if err := s.SetTokens(ctx, "slack", map[string]string{"bot_token": "b2"}); err != nil {
		t.Fatalf("SetTokens: %v", err)
	}

	got, _ := s.Get(ctx, "slack")
	if got.Tokens["bot_token"] != "b2" || got.Tokens["app_token"] != "a1" {
		t.Errorf("merge failed: %+v", got.Tokens)
	}
}

func TestStore_SetTokens_AutoCreates(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if err := s.SetTokens(ctx, "discord", map[string]string{"token": "x"}); err != nil {
		t.Fatalf("SetTokens: %v", err)
	}

	got, err := s.Get(ctx, "discord")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Tokens["token"] != "x" {
		t.Errorf("token not persisted: %+v", got)
	}
}

func TestStore_Delete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Set(ctx, "discord", Config{Enabled: true})

	if err := s.Delete(ctx, "discord"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "discord"); !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("after Delete Get = %v, want ErrProviderNotFound", err)
	}

	if err := s.Delete(ctx, "discord"); !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("second Delete = %v, want ErrProviderNotFound", err)
	}
}

func TestStore_ResolveToken_StoreWinsOverEnv(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	t.Setenv("DISCORD_TOKEN", "envtoken")

	_ = s.Set(ctx, "discord", Config{Tokens: map[string]string{"token": "stored"}})
	if got := s.ResolveToken(ctx, "discord", "token"); got != "stored" {
		t.Errorf("got %q, want stored", got)
	}
}

func TestStore_ResolveToken_FallsBackToEnv(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	t.Setenv("DISCORD_TOKEN", "envtoken")

	if got := s.ResolveToken(ctx, "discord", "token"); got != "envtoken" {
		t.Errorf("got %q, want envtoken", got)
	}
}

func TestStore_TokenHint(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.Set(ctx, "discord", Config{Tokens: map[string]string{"token": "abcdefgh1234"}})

	hint := s.TokenHint(ctx, "discord", "token")
	if hint != "...1234" {
		t.Errorf("hint = %q, want ...1234", hint)
	}
}

func TestRequiredTokenKeys(t *testing.T) {
	if got := RequiredTokenKeys("slack"); len(got) != 2 {
		t.Errorf("slack keys = %v, want 2", got)
	}
	if got := RequiredTokenKeys("other"); got != nil {
		t.Errorf("unknown provider keys = %v, want nil", got)
	}
}
