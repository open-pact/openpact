package admin

import (
	"os"
	"testing"
)

func TestScheduleStore_CreateAndGet(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	sched, err := store.Create(&Schedule{
		Name:       "test-job",
		CronExpr:   "*/5 * * * *",
		Type:       "script",
		Enabled:    true,
		ScriptName: "hello.star",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if sched.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if sched.Name != "test-job" {
		t.Errorf("expected name 'test-job', got %q", sched.Name)
	}

	got, err := store.Get(sched.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "test-job" {
		t.Errorf("expected name 'test-job', got %q", got.Name)
	}
	if got.ScriptName != "hello.star" {
		t.Errorf("expected script_name 'hello.star', got %q", got.ScriptName)
	}
}

func TestScheduleStore_List(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	// Empty list
	list, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(list))
	}

	// Create two schedules
	store.Create(&Schedule{Name: "beta", CronExpr: "0 * * * *", Type: "agent", Prompt: "hello"})
	store.Create(&Schedule{Name: "alpha", CronExpr: "0 * * * *", Type: "script", ScriptName: "test.star"})

	list, err = store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(list))
	}
	// Should be sorted by name
	if list[0].Name != "alpha" {
		t.Errorf("expected first schedule 'alpha', got %q", list[0].Name)
	}
	if list[1].Name != "beta" {
		t.Errorf("expected second schedule 'beta', got %q", list[1].Name)
	}
}

func TestScheduleStore_Update(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	sched, _ := store.Create(&Schedule{
		Name:       "test-job",
		CronExpr:   "*/5 * * * *",
		Type:       "script",
		ScriptName: "hello.star",
	})

	updated, err := store.Update(sched.ID, &Schedule{
		Name:     "renamed-job",
		CronExpr: "*/10 * * * *",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "renamed-job" {
		t.Errorf("expected name 'renamed-job', got %q", updated.Name)
	}
	if updated.CronExpr != "*/10 * * * *" {
		t.Errorf("expected cron '*/10 * * * *', got %q", updated.CronExpr)
	}
	// Script name should be preserved
	if updated.ScriptName != "hello.star" {
		t.Errorf("expected script_name preserved as 'hello.star', got %q", updated.ScriptName)
	}
}

func TestScheduleStore_Delete(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	sched, _ := store.Create(&Schedule{
		Name:       "test-job",
		CronExpr:   "*/5 * * * *",
		Type:       "script",
		ScriptName: "hello.star",
	})

	err = store.Delete(sched.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(sched.ID)
	if err != ErrScheduleNotFound {
		t.Errorf("expected ErrScheduleNotFound, got %v", err)
	}
}

func TestScheduleStore_DeleteNotFound(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	err = store.Delete("nonexistent")
	if err != ErrScheduleNotFound {
		t.Errorf("expected ErrScheduleNotFound, got %v", err)
	}
}

func TestScheduleStore_SetEnabled(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	sched, _ := store.Create(&Schedule{
		Name:       "test-job",
		CronExpr:   "*/5 * * * *",
		Type:       "script",
		Enabled:    true,
		ScriptName: "hello.star",
	})

	err = store.SetEnabled(sched.ID, false)
	if err != nil {
		t.Fatalf("SetEnabled failed: %v", err)
	}

	got, _ := store.Get(sched.ID)
	if got.Enabled {
		t.Error("expected schedule to be disabled")
	}

	err = store.SetEnabled(sched.ID, true)
	if err != nil {
		t.Fatalf("SetEnabled failed: %v", err)
	}

	got, _ = store.Get(sched.ID)
	if !got.Enabled {
		t.Error("expected schedule to be enabled")
	}
}

func TestScheduleStore_UpdateLastRun(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	sched, _ := store.Create(&Schedule{
		Name:       "test-job",
		CronExpr:   "*/5 * * * *",
		Type:       "script",
		ScriptName: "hello.star",
	})

	err = store.UpdateLastRun(sched.ID, "success", "", "output text")
	if err != nil {
		t.Fatalf("UpdateLastRun failed: %v", err)
	}

	got, _ := store.Get(sched.ID)
	if got.LastRunStatus != "success" {
		t.Errorf("expected status 'success', got %q", got.LastRunStatus)
	}
	if got.LastRunOutput != "output text" {
		t.Errorf("expected output 'output text', got %q", got.LastRunOutput)
	}
	if got.LastRunAt == nil {
		t.Error("expected LastRunAt to be set")
	}
}

func TestScheduleStore_RunOnce(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	sched, err := store.Create(&Schedule{
		Name:       "one-off-job",
		CronExpr:   "0 3 * * *",
		Type:       "script",
		Enabled:    true,
		RunOnce:    true,
		ScriptName: "migrate.star",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !sched.RunOnce {
		t.Error("expected RunOnce to be true after create")
	}

	// Verify round-trip through Get
	got, err := store.Get(sched.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !got.RunOnce {
		t.Error("expected RunOnce to be true after Get")
	}

	// Verify it appears in List
	list, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(list))
	}
	if !list[0].RunOnce {
		t.Error("expected RunOnce to be true in List")
	}

	// Verify Update can set RunOnce to false
	updated, err := store.Update(sched.ID, &Schedule{RunOnce: false})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.RunOnce {
		t.Error("expected RunOnce to be false after update")
	}
}

func TestScheduleStore_ValidationErrors(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	tests := []struct {
		name string
		sched *Schedule
	}{
		{"empty name", &Schedule{CronExpr: "* * * * *", Type: "script", ScriptName: "test.star"}},
		{"empty cron", &Schedule{Name: "test", Type: "script", ScriptName: "test.star"}},
		{"invalid type", &Schedule{Name: "test", CronExpr: "* * * * *", Type: "invalid"}},
		{"script without script_name", &Schedule{Name: "test", CronExpr: "* * * * *", Type: "script"}},
		{"agent without prompt", &Schedule{Name: "test", CronExpr: "* * * * *", Type: "agent"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Create(tt.sched)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestScheduleStore_OutputTruncation(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	sched, _ := store.Create(&Schedule{
		Name:       "test-job",
		CronExpr:   "*/5 * * * *",
		Type:       "script",
		ScriptName: "hello.star",
	})

	// Create a string longer than maxOutputLen
	longOutput := ""
	for i := 0; i < maxOutputLen+500; i++ {
		longOutput += "x"
	}

	store.UpdateLastRun(sched.ID, "success", "", longOutput)

	got, _ := store.Get(sched.ID)
	if len(got.LastRunOutput) != maxOutputLen {
		t.Errorf("expected output truncated to %d, got %d", maxOutputLen, len(got.LastRunOutput))
	}
}

func TestScheduleStore_WithOutputTarget(t *testing.T) {
	dir, err := os.MkdirTemp("", "schedule-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewScheduleStore(dir)

	sched, err := store.Create(&Schedule{
		Name:       "test-job",
		CronExpr:   "*/5 * * * *",
		Type:       "script",
		ScriptName: "hello.star",
		OutputTarget: &OutputTarget{
			Provider:  "discord",
			ChannelID: "channel:123456",
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, _ := store.Get(sched.ID)
	if got.OutputTarget == nil {
		t.Fatal("expected output_target to be set")
	}
	if got.OutputTarget.Provider != "discord" {
		t.Errorf("expected provider 'discord', got %q", got.OutputTarget.Provider)
	}
	if got.OutputTarget.ChannelID != "channel:123456" {
		t.Errorf("expected channel_id 'channel:123456', got %q", got.OutputTarget.ChannelID)
	}
}
