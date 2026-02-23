package orchestrator

import (
	"strings"
	"testing"

	"github.com/open-pact/openpact/internal/config"
	"github.com/open-pact/openpact/internal/engine"
)

func TestNewOrchestrator(t *testing.T) {
	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: t.TempDir(),
		},
		Discord: config.DiscordConfig{
			Enabled: false, // Don't try to connect
		},
	}

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

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
	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: t.TempDir(),
		},
		Discord: config.DiscordConfig{
			Enabled:      true,
			AllowedUsers: []string{"123"},
			AllowedChans: []string{"456"},
		},
	}

	// With no DISCORD_TOKEN env var, discord creation should be skipped
	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// No providers should be running since tokens were not set in env
	if len(o.providers) != 0 {
		t.Errorf("expected 0 running providers without tokens, got %d", len(o.providers))
	}
}

func TestOrchestratorDoubleStart(t *testing.T) {
	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: t.TempDir(),
		},
		Discord: config.DiscordConfig{
			Enabled: false,
		},
	}

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

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
	cfg := &config.Config{
		Engine: config.EngineConfig{
			Type: "opencode",
		},
		Workspace: config.WorkspaceConfig{
			Path: t.TempDir(),
		},
		Discord: config.DiscordConfig{
			Enabled: false,
		},
	}

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

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

	o, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

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
