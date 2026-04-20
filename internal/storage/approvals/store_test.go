package approvals

import (
	"context"
	"testing"
	"time"

	"github.com/open-pact/openpact/internal/storage"
)

func TestStore_UpsertAndGet(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	a := Approval{
		ScriptName: "foo.star",
		Hash:       "sha256:abc",
		Status:     StatusPending,
		CreatedAt:  now,
		ModifiedAt: now,
	}
	if err := s.Upsert(ctx, a); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, "foo.star")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Hash != "sha256:abc" || got.Status != StatusPending {
		t.Errorf("Get = %+v", got)
	}
}

func TestStore_Upsert_Overwrite(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	s.Upsert(ctx, Approval{ScriptName: "x", Hash: "h1", Status: StatusPending, CreatedAt: now, ModifiedAt: now})

	approvedAt := now.Add(time.Hour)
	s.Upsert(ctx, Approval{
		ScriptName: "x", Hash: "h1", Status: StatusApproved,
		ApprovedAt: &approvedAt, ApprovedBy: "matt",
		CreatedAt: now, ModifiedAt: approvedAt,
	})

	got, _ := s.Get(ctx, "x")
	if got.Status != StatusApproved || got.ApprovedBy != "matt" {
		t.Errorf("after overwrite Get = %+v", got)
	}
	if got.ApprovedAt == nil || !got.ApprovedAt.Equal(approvedAt) {
		t.Errorf("ApprovedAt = %v, want %v", got.ApprovedAt, approvedAt)
	}
}

func TestStore_Get_Missing(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	got, err := s.Get(context.Background(), "missing.star")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("Get missing should be nil, got %+v", got)
	}
}

func TestStore_Delete(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	s.Upsert(ctx, Approval{ScriptName: "x", Hash: "h", Status: StatusPending, CreatedAt: now, ModifiedAt: now})
	if err := s.Delete(ctx, "x"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ := s.Get(ctx, "x")
	if got != nil {
		t.Errorf("after Delete Get should be nil, got %+v", got)
	}

	// Absent-row delete is not an error.
	if err := s.Delete(ctx, "never_existed"); err != nil {
		t.Errorf("Delete missing returned err: %v", err)
	}
}

func TestStore_All(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	for _, name := range []string{"a.star", "b.star", "c.star"} {
		s.Upsert(ctx, Approval{ScriptName: name, Hash: "h", Status: StatusPending, CreatedAt: now, ModifiedAt: now})
	}

	all, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}
	if all["a.star"] == nil || all["b.star"] == nil || all["c.star"] == nil {
		t.Errorf("missing entries: %+v", all)
	}
}
