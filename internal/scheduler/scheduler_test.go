package scheduler

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/engine"
)

func setupTestScheduler(t *testing.T) (*Scheduler, string) {
	t.Helper()

	dir, err := os.MkdirTemp("", "scheduler-test")
	if err != nil {
		t.Fatal(err)
	}

	// Create scripts dir
	scriptsDir := dir + "/scripts"
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	store := admin.NewScheduleStore(dir)

	cfg := Config{
		ScriptsDir:     scriptsDir,
		MaxExecutionMs: 5000,
	}

	s := New(store, cfg)
	return s, dir
}

func TestScheduler_StartStop(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	s.Stop()
}

func TestScheduler_Reload(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	// Create a schedule
	s.store.Create(&admin.Schedule{
		Name:       "test-job",
		CronExpr:   "0 0 * * *",
		Type:       "script",
		Enabled:    true,
		ScriptName: "test.star",
	})

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()

	if len(s.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(s.entries))
	}

	// Add another schedule and reload
	s.store.Create(&admin.Schedule{
		Name:       "test-job-2",
		CronExpr:   "0 12 * * *",
		Type:       "script",
		Enabled:    true,
		ScriptName: "test2.star",
	})

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if len(s.entries) != 2 {
		t.Errorf("expected 2 entries after reload, got %d", len(s.entries))
	}
}

func TestScheduler_DisabledNotRegistered(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	s.store.Create(&admin.Schedule{
		Name:       "disabled-job",
		CronExpr:   "0 0 * * *",
		Type:       "script",
		Enabled:    false,
		ScriptName: "test.star",
	})

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()

	if len(s.entries) != 0 {
		t.Errorf("expected 0 entries (disabled), got %d", len(s.entries))
	}
}

func TestScheduler_ExecuteScript(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	// Write a test script
	scriptPath := dir + "/scripts/hello.star"
	if err := os.WriteFile(scriptPath, []byte(`result = "hello from script"`), 0644); err != nil {
		t.Fatal(err)
	}

	sched, err := s.store.Create(&admin.Schedule{
		Name:       "script-job",
		CronExpr:   "0 0 * * *",
		Type:       "script",
		Enabled:    true,
		ScriptName: "hello.star",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Execute directly
	s.executeJob(sched)

	// Check last run status
	got, _ := s.store.Get(sched.ID)
	if got.LastRunStatus != "success" {
		t.Errorf("expected status 'success', got %q (error: %s)", got.LastRunStatus, got.LastRunError)
	}
	if got.LastRunOutput != "hello from script" {
		t.Errorf("expected output 'hello from script', got %q", got.LastRunOutput)
	}
}

func TestScheduler_ExecuteScriptError(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	sched, _ := s.store.Create(&admin.Schedule{
		Name:       "missing-script-job",
		CronExpr:   "0 0 * * *",
		Type:       "script",
		Enabled:    true,
		ScriptName: "nonexistent.star",
	})

	s.executeJob(sched)

	got, _ := s.store.Get(sched.ID)
	if got.LastRunStatus != "error" {
		t.Errorf("expected status 'error', got %q", got.LastRunStatus)
	}
	if got.LastRunError == "" {
		t.Error("expected error message")
	}
}

// mockEngine implements EngineAPI for testing.
type mockEngine struct {
	createSessionFn func() (*engine.Session, error)
	sendFn          func(ctx context.Context, sessionID string, messages []engine.Message) (<-chan engine.Response, error)
}

func (m *mockEngine) CreateSession() (*engine.Session, error) {
	return m.createSessionFn()
}

func (m *mockEngine) Send(ctx context.Context, sessionID string, messages []engine.Message) (<-chan engine.Response, error) {
	return m.sendFn(ctx, sessionID, messages)
}

func TestScheduler_ExecuteAgent(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	mock := &mockEngine{
		createSessionFn: func() (*engine.Session, error) {
			return &engine.Session{ID: "test-session"}, nil
		},
		sendFn: func(ctx context.Context, sessionID string, messages []engine.Message) (<-chan engine.Response, error) {
			ch := make(chan engine.Response, 1)
			ch <- engine.Response{Content: "agent response"}
			close(ch)
			return ch, nil
		},
	}
	s.SetEngineAPI(mock)

	sched, _ := s.store.Create(&admin.Schedule{
		Name:     "agent-job",
		CronExpr: "0 0 * * *",
		Type:     "agent",
		Enabled:  true,
		Prompt:   "Hello agent",
	})

	s.executeJob(sched)

	got, _ := s.store.Get(sched.ID)
	if got.LastRunStatus != "success" {
		t.Errorf("expected status 'success', got %q (error: %s)", got.LastRunStatus, got.LastRunError)
	}
	if got.LastRunOutput != "agent response" {
		t.Errorf("expected output 'agent response', got %q", got.LastRunOutput)
	}
}

func TestScheduler_ExecuteAgentNoEngine(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	sched, _ := s.store.Create(&admin.Schedule{
		Name:     "agent-job",
		CronExpr: "0 0 * * *",
		Type:     "agent",
		Enabled:  true,
		Prompt:   "Hello agent",
	})

	s.executeJob(sched)

	got, _ := s.store.Get(sched.ID)
	if got.LastRunStatus != "error" {
		t.Errorf("expected status 'error', got %q", got.LastRunStatus)
	}
	if got.LastRunError == "" {
		t.Error("expected error about engine not available")
	}
}

func TestScheduler_RunNow(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	// Write a test script
	scriptPath := dir + "/scripts/quick.star"
	if err := os.WriteFile(scriptPath, []byte(`result = "quick run"`), 0644); err != nil {
		t.Fatal(err)
	}

	sched, _ := s.store.Create(&admin.Schedule{
		Name:       "quick-job",
		CronExpr:   "0 0 * * *",
		Type:       "script",
		Enabled:    true,
		ScriptName: "quick.star",
	})

	if err := s.RunNow(sched.ID); err != nil {
		t.Fatalf("RunNow failed: %v", err)
	}

	// Wait briefly for goroutine to complete
	time.Sleep(500 * time.Millisecond)

	got, _ := s.store.Get(sched.ID)
	if got.LastRunStatus != "success" {
		t.Errorf("expected status 'success', got %q (error: %s)", got.LastRunStatus, got.LastRunError)
	}
}

// mockChat implements ChatAPI for testing.
type mockChat struct {
	lastProvider string
	lastTarget   string
	lastContent  string
}

func (m *mockChat) SendViaProvider(provider, target, content string) error {
	m.lastProvider = provider
	m.lastTarget = target
	m.lastContent = content
	return nil
}

func TestScheduler_OutputDelivery(t *testing.T) {
	s, dir := setupTestScheduler(t)
	defer os.RemoveAll(dir)

	chat := &mockChat{}
	s.SetChatAPI(chat)

	// Write a test script
	scriptPath := dir + "/scripts/output.star"
	if err := os.WriteFile(scriptPath, []byte(`result = "hello world"`), 0644); err != nil {
		t.Fatal(err)
	}

	sched, _ := s.store.Create(&admin.Schedule{
		Name:       "output-job",
		CronExpr:   "0 0 * * *",
		Type:       "script",
		Enabled:    true,
		ScriptName: "output.star",
		OutputTarget: &admin.OutputTarget{
			Provider:  "discord",
			ChannelID: "channel:123",
		},
	})

	s.executeJob(sched)

	if chat.lastProvider != "discord" {
		t.Errorf("expected provider 'discord', got %q", chat.lastProvider)
	}
	if chat.lastTarget != "channel:123" {
		t.Errorf("expected target 'channel:123', got %q", chat.lastTarget)
	}
	if chat.lastContent == "" {
		t.Error("expected non-empty content")
	}
}
