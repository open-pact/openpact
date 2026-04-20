package orchestrator

import (
	"context"
	"crypto/rand"
	"database/sql"
	"strings"
	"sync"
	"testing"

	"github.com/open-pact/openpact/internal/chat"
	"github.com/open-pact/openpact/internal/config"
	"github.com/open-pact/openpact/internal/storage"
	"github.com/open-pact/openpact/internal/storage/secrets"
	"github.com/stack-bound/stackllm/conversation"
	"github.com/stack-bound/stackllm/session"
)

// testSecretStore builds an encrypted secret store over the given DB
// with a throwaway per-test key — since in-memory test DBs are reset
// between tests, the key is never reused.
func testSecretStore(t *testing.T, db *sql.DB) *secrets.Store {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	s, err := secrets.NewStore(db, key)
	if err != nil {
		t.Fatalf("secrets.NewStore: %v", err)
	}
	return s
}

func newTestOrchestrator(t *testing.T) (*Orchestrator, *config.Config) {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Workspace: config.WorkspaceConfig{Path: tmpDir},
	}
	if err := cfg.Workspace.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	db := storage.NewTestDB(t)
	o, err := New(cfg, db, testSecretStore(t, db), nil)
	if err != nil {
		t.Fatalf("New orchestrator: %v", err)
	}
	t.Cleanup(func() {
		if o.stack != nil {
			o.stack.Close()
		}
	})
	return o, cfg
}

func TestNewOrchestrator(t *testing.T) {
	o, cfg := newTestOrchestrator(t)

	if o.cfg != cfg {
		t.Error("config not set correctly")
	}
	if o.contextLoader == nil {
		t.Error("context loader not initialized")
	}
	if o.mcpServer == nil {
		t.Error("MCP server not initialized")
	}
	if o.stack == nil {
		t.Fatal("stack not initialized")
	}
	if o.stack.Handler == nil {
		t.Error("stack handler not initialized")
	}
	if o.stack.Sessions == nil {
		t.Error("session store not initialized")
	}
}

func TestNewOrchestratorStartsWithNoProviders(t *testing.T) {
	// After the YAML → DB migration, provider config comes from the
	// DB only. A fresh workspace has no chat providers enabled, so the
	// orchestrator's provider map stays empty until the admin UI
	// enables one.
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Workspace: config.WorkspaceConfig{Path: tmpDir},
	}
	cfg.Workspace.EnsureDirs()

	db2 := storage.NewTestDB(t)
	o, err := New(cfg, db2, testSecretStore(t, db2), nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	t.Cleanup(func() { o.stack.Close() })

	if len(o.providers) != 0 {
		t.Errorf("expected 0 running providers on fresh DB, got %d", len(o.providers))
	}
}

func TestOrchestratorDoubleStart(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	o.mu.Lock()
	o.running = true
	o.mu.Unlock()

	// Start should fail when already running; pass a cancelled context so
	// it doesn't block.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := o.Start(ctx); err == nil {
		t.Error("expected error when starting already-running orchestrator")
	}
}

func TestOrchestratorStop(t *testing.T) {
	o, _ := newTestOrchestrator(t)
	// Safe to Stop even before Start.
	o.Stop()
}

func TestOrchestratorReloadContext(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	if err := o.ReloadContext(); err != nil {
		t.Errorf("ReloadContext failed: %v", err)
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{38100, "38.1k"},
		{128500, "128.5k"},
		{200000, "200.0k"},
	}

	for _, tt := range tests {
		got := formatTokens(tt.input)
		if got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatContextUsage(t *testing.T) {
	usage := &ContextUsage{
		Model:        "openai/gpt-4o",
		MessageCount: 12,
		PromptTokens: 38100,
		OutputTokens: 7100,
		ContextLimit: 200000,
	}

	result := formatContextUsage("abc12345xyz", usage)

	checks := []string{
		"**Context Usage**",
		"abc12345",
		"openai/gpt-4o",
		"12 assistant responses",
		"38.1k tokens",
		"19.1%",
		"200.0k",
		"7.1k",
	}
	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("formatContextUsage missing %q in output:\n%s", check, result)
		}
	}
}

func TestFormatContextUsageNoMessages(t *testing.T) {
	usage := &ContextUsage{}
	result := formatContextUsage("session123", usage)

	if !strings.Contains(result, "0 assistant responses") {
		t.Errorf("expected '0 assistant responses' in output: %s", result)
	}
}

func TestFormatContextUsageNoLimit(t *testing.T) {
	usage := &ContextUsage{
		Model:        "ollama/llama3",
		MessageCount: 3,
		PromptTokens: 5000,
		OutputTokens: 1000,
		ContextLimit: 0,
	}

	result := formatContextUsage("sess1", usage)

	if strings.Contains(result, "%") {
		t.Errorf("should not contain percentage when limit is 0: %s", result)
	}
	if !strings.Contains(result, "5.0k tokens") {
		t.Errorf("expected '5.0k tokens' in output: %s", result)
	}
}

func TestChannelModeGetSetDefault(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	mode := o.GetChannelMode("discord", "chan123")
	if mode != chat.ModeSimple {
		t.Errorf("expected default mode %q, got %q", chat.ModeSimple, mode)
	}

	o.SetChannelMode("discord", "chan123", chat.ModeFull)
	mode = o.GetChannelMode("discord", "chan123")
	if mode != chat.ModeFull {
		t.Errorf("expected mode %q, got %q", chat.ModeFull, mode)
	}

	mode = o.GetChannelMode("discord", "chan456")
	if mode != chat.ModeSimple {
		t.Errorf("expected default mode for different channel, got %q", mode)
	}
}

func TestChannelModePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Workspace: config.WorkspaceConfig{Path: tmpDir},
	}
	cfg.Workspace.EnsureDirs()

	// Share a DB between the two orchestrators so the second one sees
	// the rows the first one wrote. (Previously this test reopened the
	// same data dir and read the JSON file back; the DB-backed
	// equivalent is sharing the *sql.DB handle.)
	db := storage.NewTestDB(t)
	o, err := New(cfg, db, testSecretStore(t, db), nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	o.SetChannelMode("discord", "chan1", chat.ModeThinking)
	o.SetChannelMode("telegram", "chan2", chat.ModeTools)

	// Close the first orchestrator's stack before reopening the workspace.
	o.stack.Close()

	o2, err := New(cfg, db, testSecretStore(t, db), nil)
	if err != nil {
		t.Fatalf("failed to create second orchestrator: %v", err)
	}
	t.Cleanup(func() { o2.stack.Close() })

	o2.loadChannelModes()

	if o2.GetChannelMode("discord", "chan1") != chat.ModeThinking {
		t.Errorf("mode not restored after reload, got %q", o2.GetChannelMode("discord", "chan1"))
	}
	if o2.GetChannelMode("telegram", "chan2") != chat.ModeTools {
		t.Errorf("mode not restored after reload, got %q", o2.GetChannelMode("telegram", "chan2"))
	}
}

func TestHandleModeCommands(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	tests := []struct {
		command  string
		wantMode string
	}{
		{"mode-simple", chat.ModeSimple},
		{"mode-thinking", chat.ModeThinking},
		{"mode-tools", chat.ModeTools},
		{"mode-full", chat.ModeFull},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			resp, err := o.handleChatCommand("discord", "testchan", "user1", tt.command, "")
			if err != nil {
				t.Fatalf("handleChatCommand returned error: %v", err)
			}
			if resp == "" {
				t.Error("expected non-empty response")
			}

			mode := o.GetChannelMode("discord", "testchan")
			if mode != tt.wantMode {
				t.Errorf("after /%s, mode = %q, want %q", tt.command, mode, tt.wantMode)
			}
		})
	}
}

func TestListChannelModes(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	o.SetChannelMode("discord", "c1", chat.ModeFull)
	o.SetChannelMode("slack", "c2", chat.ModeThinking)

	modes := o.ListChannelModes()
	if len(modes) != 2 {
		t.Errorf("expected 2 modes, got %d", len(modes))
	}
	if modes["discord:c1"] != chat.ModeFull {
		t.Errorf("expected %q, got %q", chat.ModeFull, modes["discord:c1"])
	}
	if modes["slack:c2"] != chat.ModeThinking {
		t.Errorf("expected %q, got %q", chat.ModeThinking, modes["slack:c2"])
	}
}

// TestSessionLockSerializesPerSession confirms the per-session mutex is
// the same instance for the same session ID (so concurrent messages to a
// channel queue up behind one agent run).
func TestSessionLockSerializesPerSession(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	m1 := o.sessionLock("sess-a")
	m2 := o.sessionLock("sess-a")
	m3 := o.sessionLock("sess-b")

	if m1 != m2 {
		t.Error("sessionLock returned different mutexes for the same session ID")
	}
	if m1 == m3 {
		t.Error("sessionLock returned the same mutex for different session IDs")
	}
}

// TestEnsureChannelSessionPersists creates a new session via the public
// helper and confirms it lands in the SQLite store.
func TestEnsureChannelSessionPersists(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	sid, err := o.ensureChannelSession("discord", "c1")
	if err != nil {
		t.Fatalf("ensureChannelSession: %v", err)
	}
	if sid == "" {
		t.Fatal("session ID is empty")
	}

	// Reusing the same channel returns the same ID.
	sid2, err := o.ensureChannelSession("discord", "c1")
	if err != nil {
		t.Fatalf("ensureChannelSession second call: %v", err)
	}
	if sid != sid2 {
		t.Errorf("second call returned a different session: %s != %s", sid, sid2)
	}

	// And the session is loadable from SQLite.
	sess, err := o.stack.Sessions.Load(context.Background(), sid)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sess.ID != sid {
		t.Errorf("loaded session ID = %s, want %s", sess.ID, sid)
	}

	// New sessions must carry a human-readable name so the admin UI list
	// doesn't show a grid of indistinguishable UUIDs.
	if sess.Name != "Discord: c1" {
		t.Errorf("Session.Name = %q, want %q", sess.Name, "Discord: c1")
	}
}

func TestChannelSessionName(t *testing.T) {
	cases := []struct {
		provider  string
		channelID string
		want      string
	}{
		{"discord", "general", "Discord: general"},
		{"slack", "C123ABC", "Slack: C123ABC"},
		{"telegram", "42", "Telegram: 42"},
		{"discord", "", "Discord"},
	}
	for _, tc := range cases {
		got := channelSessionName(tc.provider, tc.channelID)
		if got != tc.want {
			t.Errorf("channelSessionName(%q, %q) = %q, want %q",
				tc.provider, tc.channelID, got, tc.want)
		}
	}
}

// TestSessionSaveLoadRoundTrip drives a message through the SQLite store
// directly to verify orchestrator's session plumbing is intact. This does
// not call the live agent — we can't from unit tests — but it does run
// through AppendMessage, Save, Load.
func TestSessionSaveLoadRoundTrip(t *testing.T) {
	o, _ := newTestOrchestrator(t)
	ctx := context.Background()

	sess := session.New()
	sess.AppendMessage(conversation.Message{
		Role:   conversation.RoleUser,
		Blocks: []conversation.Block{{Type: conversation.BlockText, Text: "hi"}},
	})
	if err := o.stack.Sessions.Save(ctx, sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := o.stack.Sessions.Load(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(loaded.Messages))
	}
	if loaded.Messages[0].TextContent() != "hi" {
		t.Errorf("text content = %q, want hi", loaded.Messages[0].TextContent())
	}
}

// TestConcurrentSameSessionSerializes confirms the per-session mutex
// prevents concurrent goroutines from interleaving into the critical
// section. We can't run the actual agent, but we can exercise the lock
// and make sure only one goroutine holds it at a time.
func TestConcurrentSameSessionSerializes(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	var active int32 = 0
	var maxActive int32 = 0
	var wg sync.WaitGroup
	var stateMu sync.Mutex

	lock := o.sessionLock("s1")

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock.Lock()
			defer lock.Unlock()

			stateMu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			stateMu.Unlock()

			// Yield so another goroutine would race if the lock were not held.
			for i := 0; i < 100; i++ {
			}

			stateMu.Lock()
			active--
			stateMu.Unlock()
		}()
	}
	wg.Wait()
	if maxActive > 1 {
		t.Errorf("expected at most 1 goroutine in critical section, saw %d", maxActive)
	}
}

func TestHandleCommandNew_CreatesSession(t *testing.T) {
	o, _ := newTestOrchestrator(t)

	resp, err := o.handleChatCommand("discord", "chan-new", "u1", "new", "")
	if err != nil {
		t.Fatalf("handleChatCommand: %v", err)
	}
	if !strings.Contains(resp, "New session started") {
		t.Errorf("response did not confirm creation: %s", resp)
	}

	sid := o.GetChannelSession("discord", "chan-new")
	if sid == "" {
		t.Fatal("new command did not map a session for the channel")
	}

	sess, err := o.stack.Sessions.Load(context.Background(), sid)
	if err != nil {
		t.Fatalf("load session created by /new: %v", err)
	}
	if sess.Name != "Discord: chan-new" {
		t.Errorf("Session.Name = %q, want %q", sess.Name, "Discord: chan-new")
	}
}

func TestHandleCommandUnknown(t *testing.T) {
	o, _ := newTestOrchestrator(t)
	resp, err := o.handleChatCommand("discord", "chan", "u", "does-not-exist", "")
	if err != nil {
		t.Fatalf("handleChatCommand: %v", err)
	}
	if !strings.Contains(resp, "Unknown command") {
		t.Errorf("expected unknown-command response, got %q", resp)
	}
}
