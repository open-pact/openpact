package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-pact/openpact/internal/chat"
	"github.com/open-pact/openpact/internal/config"
	"github.com/open-pact/openpact/internal/engine"
)

func TestNewOrchestrator(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: tmpDir,
		},
		Discord: config.DiscordConfig{
			Enabled: false, // Don't try to connect
		},
	}
	cfg.Workspace.EnsureDirs()

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

	if o.cfg != cfg {
		t.Error("config not set correctly")
	}

	if o.contextLoader == nil {
		t.Error("context loader not initialized")
	}

	if o.mcpServer == nil {
		t.Error("MCP server not initialized")
	}

	if o.engine == nil {
		t.Error("engine not initialized")
	}
}

func TestNewOrchestratorWithDiscordNoToken(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: tmpDir,
		},
		Discord: config.DiscordConfig{
			Enabled:      true,
			AllowedUsers: []string{"123"},
			AllowedChans: []string{"456"},
		},
	}
	cfg.Workspace.EnsureDirs()

	// With no DISCORD_TOKEN env var, discord creation should be skipped
	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

	// No providers should be running since tokens were not set in env
	if len(o.providers) != 0 {
		t.Errorf("expected 0 running providers without tokens, got %d", len(o.providers))
	}
}

func TestOrchestratorDoubleStart(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: tmpDir,
		},
		Discord: config.DiscordConfig{
			Enabled: false,
		},
	}
	cfg.Workspace.EnsureDirs()

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

	// Manually set running state
	o.mu.Lock()
	o.running = true
	o.mu.Unlock()

	// Try to start - should fail
	err = o.Start(nil)
	if err == nil {
		t.Error("expected error when starting already-running orchestrator")
	}
}

func TestOrchestratorStop(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: tmpDir,
		},
		Discord: config.DiscordConfig{
			Enabled: false,
		},
	}
	cfg.Workspace.EnsureDirs()

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

	// Stop should be safe to call even before start
	o.Stop()
}

func TestOrchestratorReloadContext(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: tmpDir,
		},
		Discord: config.DiscordConfig{
			Enabled: false,
		},
	}
	cfg.Workspace.EnsureDirs()

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

	// Reload should work even with empty workspace
	err = o.ReloadContext()
	if err != nil {
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
	usage := &engine.ContextUsage{
		Model:          "claude-sonnet-4-20250514",
		MessageCount:   12,
		CurrentContext: 38100,
		TotalOutput:    7100,
		TotalReasoning: 2300,
		CacheRead:      25000,
		CacheWrite:     8500,
		TotalCost:      0.0832,
		ContextLimit:   200000,
	}

	result := formatContextUsage("abc12345xyz", usage)

	// Check key parts are present
	checks := []string{
		"**Context Usage**",
		"abc12345",
		"claude-sonnet-4-20250514",
		"12 assistant responses",
		"38.1k tokens",
		"19.1%",
		"200.0k",
		"7.1k tokens",
		"2.3k reasoning",
		"25.0k read",
		"8.5k write",
		"$0.0832",
	}
	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("formatContextUsage missing %q in output:\n%s", check, result)
		}
	}
}

func TestFormatContextUsageNoMessages(t *testing.T) {
	usage := &engine.ContextUsage{}
	result := formatContextUsage("session123", usage)

	if !strings.Contains(result, "0 assistant responses") {
		t.Errorf("expected '0 assistant responses' in output: %s", result)
	}
	if !strings.Contains(result, "unavailable") {
		t.Errorf("expected 'unavailable' in output: %s", result)
	}
}

func TestFormatContextUsageNoLimit(t *testing.T) {
	usage := &engine.ContextUsage{
		Model:          "gpt-4",
		MessageCount:   3,
		CurrentContext: 5000,
		TotalOutput:    1000,
		ContextLimit:   0, // unknown
	}

	result := formatContextUsage("sess1", usage)

	// Should NOT contain percentage when limit is 0
	if strings.Contains(result, "%") {
		t.Errorf("should not contain percentage when limit is 0: %s", result)
	}
	if !strings.Contains(result, "5.0k tokens") {
		t.Errorf("expected '5.0k tokens' in output: %s", result)
	}
}

func TestChannelModeGetSetDefault(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Engine:    config.EngineConfig{Type: "opencode"},
		Workspace: config.WorkspaceConfig{Path: tmpDir},
	}
	cfg.Workspace.EnsureDirs()

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

	// Default should be "simple"
	mode := o.GetChannelMode("discord", "chan123")
	if mode != chat.ModeSimple {
		t.Errorf("expected default mode %q, got %q", chat.ModeSimple, mode)
	}

	// Set mode
	o.SetChannelMode("discord", "chan123", chat.ModeFull)
	mode = o.GetChannelMode("discord", "chan123")
	if mode != chat.ModeFull {
		t.Errorf("expected mode %q, got %q", chat.ModeFull, mode)
	}

	// Different channel should still be default
	mode = o.GetChannelMode("discord", "chan456")
	if mode != chat.ModeSimple {
		t.Errorf("expected default mode for different channel, got %q", mode)
	}
}

func TestChannelModePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Engine:    config.EngineConfig{Type: "opencode"},
		Workspace: config.WorkspaceConfig{Path: tmpDir},
	}
	cfg.Workspace.EnsureDirs()

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

	// Set modes
	o.SetChannelMode("discord", "chan1", chat.ModeThinking)
	o.SetChannelMode("telegram", "chan2", chat.ModeTools)

	// Verify file was written
	path := filepath.Join(cfg.Workspace.DataDir(), "channel_modes.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read channel modes file: %v", err)
	}

	var f channelModesFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("failed to parse channel modes file: %v", err)
	}

	if f.Modes["discord:chan1"] != chat.ModeThinking {
		t.Errorf("expected mode %q in file, got %q", chat.ModeThinking, f.Modes["discord:chan1"])
	}
	if f.Modes["telegram:chan2"] != chat.ModeTools {
		t.Errorf("expected mode %q in file, got %q", chat.ModeTools, f.Modes["telegram:chan2"])
	}

	// Close first orchestrator's MCP server before creating second
	o.closeMCPHTTPServer()

	// Create new orchestrator and load
	o2, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create second orchestrator: %v", err)
	}
	defer o2.closeMCPHTTPServer()

	o2.loadChannelModes()

	if o2.GetChannelMode("discord", "chan1") != chat.ModeThinking {
		t.Errorf("mode not restored after reload, got %q", o2.GetChannelMode("discord", "chan1"))
	}
	if o2.GetChannelMode("telegram", "chan2") != chat.ModeTools {
		t.Errorf("mode not restored after reload, got %q", o2.GetChannelMode("telegram", "chan2"))
	}
}

func TestHandleModeCommands(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Engine:    config.EngineConfig{Type: "opencode"},
		Workspace: config.WorkspaceConfig{Path: tmpDir},
	}
	cfg.Workspace.EnsureDirs()

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

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
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Engine:    config.EngineConfig{Type: "opencode"},
		Workspace: config.WorkspaceConfig{Path: tmpDir},
	}
	cfg.Workspace.EnsureDirs()

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	defer o.closeMCPHTTPServer()

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

func TestExtractToolCall(t *testing.T) {
	// Real OpenCode structure: tool is a string, input/output are in state
	raw := json.RawMessage(`{
		"type": "tool",
		"id": "prt_abc123",
		"callID": "call_xyz",
		"tool": "openpact_workspace_list",
		"state": {
			"status": "completed",
			"input": {"path": "memory"},
			"output": "2026-02-22.md\n2026-02-23.md"
		}
	}`)

	tc, ok := extractToolCall(raw)
	if !ok {
		t.Fatal("expected tool call to be extracted")
	}
	if tc.Name != "openpact_workspace_list" {
		t.Errorf("name = %q, want %q", tc.Name, "openpact_workspace_list")
	}
	if !strings.Contains(tc.Input, `"path"`) || !strings.Contains(tc.Input, `"memory"`) {
		t.Errorf("input = %q, expected to contain path and memory", tc.Input)
	}
	if tc.Output != "2026-02-22.md\n2026-02-23.md" {
		t.Errorf("output = %q, want %q", tc.Output, "2026-02-22.md\n2026-02-23.md")
	}
}

func TestExtractToolCallObjectTool(t *testing.T) {
	// Fallback: tool as an object with name field (in case some providers use this)
	raw := json.RawMessage(`{
		"type": "tool",
		"tool": {"name": "workspace_read"},
		"state": {"status": "completed", "input": {"path": "/foo"}, "output": "file contents"}
	}`)

	tc, ok := extractToolCall(raw)
	if !ok {
		t.Fatal("expected object-format tool call to be extracted")
	}
	if tc.Name != "workspace_read" {
		t.Errorf("name = %q, want %q", tc.Name, "workspace_read")
	}
	if tc.Output != "file contents" {
		t.Errorf("output = %q, want %q", tc.Output, "file contents")
	}
}

func TestExtractToolCallNonTool(t *testing.T) {
	raw := json.RawMessage(`{"type": "text", "content": "hello"}`)

	_, ok := extractToolCall(raw)
	if ok {
		t.Error("expected non-tool part to not be extracted")
	}
}

func TestExtractToolCallNoName(t *testing.T) {
	// Part with type "tool" but no tool field at all — should be skipped
	raw := json.RawMessage(`{"type": "tool", "id": "t1", "text": "some_tool"}`)

	_, ok := extractToolCall(raw)
	if ok {
		t.Error("expected tool part without tool name to not be extracted")
	}
}

func TestExtractToolCallRunningState(t *testing.T) {
	// Running tool calls (no output yet) should still be extracted
	raw := json.RawMessage(`{
		"type": "tool",
		"tool": "openpact_workspace_read",
		"state": {
			"status": "running",
			"input": {"path": "SOUL.md"}
		}
	}`)

	tc, ok := extractToolCall(raw)
	if !ok {
		t.Fatal("expected running tool call to be extracted")
	}
	if tc.Name != "openpact_workspace_read" {
		t.Errorf("name = %q, want %q", tc.Name, "openpact_workspace_read")
	}
	if tc.Output != "" {
		t.Errorf("output = %q, want empty for running tool", tc.Output)
	}
}
