package channels

import (
	"context"
	"testing"

	"github.com/open-pact/openpact/internal/storage"
)

func TestSessionStore_RoundTrip(t *testing.T) {
	s := NewSessionStore(storage.NewTestDB(t))
	ctx := context.Background()

	if _, ok, err := s.Get(ctx, "discord", "c1"); err != nil || ok {
		t.Fatalf("missing Get: err=%v ok=%v", err, ok)
	}

	if err := s.Set(ctx, "discord", "c1", "sess-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	id, ok, err := s.Get(ctx, "discord", "c1")
	if err != nil || !ok || id != "sess-1" {
		t.Errorf("Get = %q/%v, want sess-1/true", id, ok)
	}

	// Overwrite
	if err := s.Set(ctx, "discord", "c1", "sess-2"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	id, _, _ = s.Get(ctx, "discord", "c1")
	if id != "sess-2" {
		t.Errorf("after overwrite Get = %q, want sess-2", id)
	}
}

func TestSessionStore_All(t *testing.T) {
	s := NewSessionStore(storage.NewTestDB(t))
	ctx := context.Background()

	_ = s.Set(ctx, "discord", "c1", "a")
	_ = s.Set(ctx, "slack", "c2", "b")

	all, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 2 || all["discord:c1"] != "a" || all["slack:c2"] != "b" {
		t.Errorf("All = %+v", all)
	}
}

func TestModeStore_RoundTrip(t *testing.T) {
	s := NewModeStore(storage.NewTestDB(t))
	ctx := context.Background()

	if err := s.Set(ctx, "slack", "c1", "full"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	m, ok, err := s.Get(ctx, "slack", "c1")
	if err != nil || !ok || m != "full" {
		t.Errorf("Get = %q/%v", m, ok)
	}

	_ = s.Set(ctx, "slack", "c1", "thinking")
	m, _, _ = s.Get(ctx, "slack", "c1")
	if m != "thinking" {
		t.Errorf("after overwrite = %q, want thinking", m)
	}
}

func TestModeStore_All(t *testing.T) {
	s := NewModeStore(storage.NewTestDB(t))
	ctx := context.Background()

	_ = s.Set(ctx, "discord", "c1", "simple")
	_ = s.Set(ctx, "slack", "c2", "full")

	all, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if all["discord:c1"] != "simple" || all["slack:c2"] != "full" {
		t.Errorf("All = %+v", all)
	}
}
