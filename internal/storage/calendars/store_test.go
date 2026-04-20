package calendars

import (
	"context"
	"testing"

	"github.com/open-pact/openpact/internal/storage"
)

func TestList_Empty(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got == nil {
		t.Error("List should return [] not nil so JSON encodes as []")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestReplace_RoundTrip(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	ctx := context.Background()

	want := []Feed{
		{Name: "Work", URL: "https://work.example/cal.ics"},
		{Name: "Home", URL: "https://home.example/cal.ics"},
	}
	if err := s.Replace(ctx, want); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestReplace_PreservesOrder(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	ctx := context.Background()

	// Insert in a specific order, then replace reversed — the result
	// must match the new order exactly.
	_ = s.Replace(ctx, []Feed{{Name: "A"}, {Name: "B"}, {Name: "C"}})
	_ = s.Replace(ctx, []Feed{{Name: "C"}, {Name: "B"}, {Name: "A"}})

	got, _ := s.List(ctx)
	if len(got) != 3 || got[0].Name != "C" || got[2].Name != "A" {
		t.Errorf("order not preserved: %+v", got)
	}
}

func TestReplace_Empty_Clears(t *testing.T) {
	s := NewStore(storage.NewTestDB(t))
	ctx := context.Background()
	_ = s.Replace(ctx, []Feed{{Name: "A"}, {Name: "B"}})
	if err := s.Replace(ctx, nil); err != nil {
		t.Fatalf("Replace nil: %v", err)
	}
	got, _ := s.List(ctx)
	if len(got) != 0 {
		t.Errorf("after Replace(nil) List = %+v, want empty", got)
	}
}
