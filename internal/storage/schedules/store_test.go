package schedules

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/open-pact/openpact/internal/storage"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(storage.NewTestDB(t))
}

func TestStore_CreateAndGet(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	in := &Schedule{
		Name:       "daily-report",
		CronExpr:   "0 9 * * *",
		Type:       "script",
		ScriptName: "report.star",
		Enabled:    true,
	}
	out, err := s.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if out.ID == "" {
		t.Error("expected generated ID")
	}
	if out.Name != "daily-report" {
		t.Errorf("Name = %q, want daily-report", out.Name)
	}

	got, err := s.Get(ctx, out.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != out.ID || got.ScriptName != "report.star" {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), "missing")
	if !errors.Is(err, ErrScheduleNotFound) {
		t.Errorf("err = %v, want ErrScheduleNotFound", err)
	}
}

func TestStore_Create_AgentWithOutputTarget(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	in := &Schedule{
		Name:     "digest",
		CronExpr: "@daily",
		Type:     "agent",
		Prompt:   "Summarize today",
		OutputTarget: &OutputTarget{
			Provider:  "slack",
			ChannelID: "C123",
		},
	}
	out, err := s.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, _ := s.Get(ctx, out.ID)
	if got.OutputTarget == nil || got.OutputTarget.ChannelID != "C123" {
		t.Errorf("OutputTarget round-trip lost: %+v", got.OutputTarget)
	}
}

func TestStore_Create_Validation(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	cases := []*Schedule{
		{Name: "", CronExpr: "*", Type: "script", ScriptName: "x"},
		{Name: "ok", CronExpr: "", Type: "script", ScriptName: "x"},
		{Name: "ok", CronExpr: "*", Type: "bogus"},
		{Name: "ok", CronExpr: "*", Type: "script"},             // missing script_name
		{Name: "ok", CronExpr: "*", Type: "agent"},              // missing prompt
	}
	for i, c := range cases {
		if _, err := s.Create(ctx, c); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestStore_List(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	_, _ = s.Create(ctx, &Schedule{Name: "b", CronExpr: "*", Type: "script", ScriptName: "b.star"})
	_, _ = s.Create(ctx, &Schedule{Name: "a", CronExpr: "*", Type: "script", ScriptName: "a.star"})
	_, _ = s.Create(ctx, &Schedule{Name: "c", CronExpr: "*", Type: "script", ScriptName: "c.star"})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len = %d, want 3", len(list))
	}
	if list[0].Name != "a" || list[2].Name != "c" {
		t.Errorf("order wrong: %+v", list)
	}
}

func TestStore_Update(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	sc, _ := s.Create(ctx, &Schedule{Name: "x", CronExpr: "*", Type: "script", ScriptName: "s.star"})

	_, err := s.Update(ctx, sc.ID, &Schedule{Name: "renamed", Enabled: true})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get(ctx, sc.ID)
	if got.Name != "renamed" {
		t.Errorf("Name not updated: %+v", got)
	}
}

func TestStore_Delete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	sc, _ := s.Create(ctx, &Schedule{Name: "x", CronExpr: "*", Type: "script", ScriptName: "s.star"})
	if err := s.Delete(ctx, sc.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, sc.ID); !errors.Is(err, ErrScheduleNotFound) {
		t.Errorf("second Delete = %v, want ErrScheduleNotFound", err)
	}
}

func TestStore_SetEnabled(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	sc, _ := s.Create(ctx, &Schedule{Name: "x", CronExpr: "*", Type: "script", ScriptName: "s.star"})

	if err := s.SetEnabled(ctx, sc.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	got, _ := s.Get(ctx, sc.ID)
	if !got.Enabled {
		t.Error("Enabled should be true")
	}

	if err := s.SetEnabled(ctx, "missing", true); !errors.Is(err, ErrScheduleNotFound) {
		t.Errorf("SetEnabled missing = %v, want ErrScheduleNotFound", err)
	}
}

func TestStore_UpdateLastRun(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	sc, _ := s.Create(ctx, &Schedule{Name: "x", CronExpr: "*", Type: "script", ScriptName: "s.star"})

	if err := s.UpdateLastRun(ctx, sc.ID, "ok", "", "hello"); err != nil {
		t.Fatalf("UpdateLastRun: %v", err)
	}
	got, _ := s.Get(ctx, sc.ID)
	if got.LastRunStatus != "ok" || got.LastRunOutput != "hello" {
		t.Errorf("LastRun not persisted: %+v", got)
	}
	if got.LastRunAt == nil {
		t.Error("LastRunAt should be set")
	}

	// Output truncation
	huge := make([]byte, maxOutputLen+100)
	for i := range huge {
		huge[i] = 'x'
	}
	_ = s.UpdateLastRun(ctx, sc.ID, "ok", "", string(huge))
	got, _ = s.Get(ctx, sc.ID)
	if len(got.LastRunOutput) != maxOutputLen {
		t.Errorf("output len = %d, want %d", len(got.LastRunOutput), maxOutputLen)
	}

	_ = time.Now // keep time import used on older Go versions — the file relies on time.RFC3339Nano indirectly
}
