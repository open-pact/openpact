package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestBuildOpenCodeConfig_DisablesBuiltinTools(t *testing.T) {
	cfg := Config{}
	config := BuildOpenCodeConfig(cfg, "test-token")

	tools, ok := config["tools"].(map[string]bool)
	if !ok {
		t.Fatal("expected tools to be map[string]bool")
	}

	disabledTools := []string{"bash", "write", "edit", "read", "grep", "glob", "list", "patch", "webfetch", "websearch"}
	for _, tool := range disabledTools {
		if val, exists := tools[tool]; !exists || val != false {
			t.Errorf("expected tool %q to be disabled (false), got %v", tool, val)
		}
	}
}

func TestBuildOpenCodeConfig_SetsPermissions(t *testing.T) {
	cfg := Config{}
	config := BuildOpenCodeConfig(cfg, "test-token")

	perms, ok := config["permission"].(map[string]string)
	if !ok {
		t.Fatal("expected permission to be map[string]string")
	}

	if v, ok := perms["openpact_*"]; !ok || v != "allow" {
		t.Errorf("expected openpact_* permission to be 'allow', got %q", v)
	}
}

func TestFindMCPBinary_NextToExecutable(t *testing.T) {
	// Create a temp dir with a fake mcp-server binary
	tmpDir := t.TempDir()
	fakeBinary := tmpDir + "/mcp-server"
	if err := os.WriteFile(fakeBinary, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// FindMCPBinary looks next to os.Executable(), which we can't override,
	// but we can verify it falls back to PATH lookup
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	path, err := FindMCPBinary()
	if err != nil {
		t.Fatalf("expected to find mcp-server in PATH, got error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestFindMCPBinary_NotFound(t *testing.T) {
	// Empty PATH so it can't find anything
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	_, err := FindMCPBinary()
	if err == nil {
		t.Error("expected error when mcp-server not found")
	}
}

func TestBuildOpenCodeConfig_ConfiguresRemoteMCP(t *testing.T) {
	cfg := Config{}
	token := "test-token-abc123"
	config := BuildOpenCodeConfig(cfg, token)

	mcpSection, ok := config["mcp"].(map[string]interface{})
	if !ok {
		t.Fatal("expected mcp section in config")
	}

	openpact, ok := mcpSection["openpact"].(map[string]interface{})
	if !ok {
		t.Fatal("expected openpact MCP server config")
	}

	if openpact["type"] != "remote" {
		t.Errorf("expected mcp type to be 'remote', got %v", openpact["type"])
	}

	expectedURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", MCPPort)
	if openpact["url"] != expectedURL {
		t.Errorf("expected url %q, got %v", expectedURL, openpact["url"])
	}

	headers, ok := openpact["headers"].(map[string]string)
	if !ok {
		t.Fatal("expected headers map in MCP config")
	}

	expectedAuth := "Bearer " + token
	if headers["Authorization"] != expectedAuth {
		t.Errorf("expected Authorization header %q, got %q", expectedAuth, headers["Authorization"])
	}
}

func TestBuildOpenCodeConfig_NoMCPWhenNoToken(t *testing.T) {
	cfg := Config{}
	config := BuildOpenCodeConfig(cfg, "")

	if _, ok := config["mcp"]; ok {
		t.Error("expected no mcp section when no token provided")
	}
}

func TestBuildOpenCodeConfig_ValidJSON(t *testing.T) {
	cfg := Config{
		WorkDir: "/workspace",
	}
	config := BuildOpenCodeConfig(cfg, "test-token")

	// Must be valid JSON (this is what gets passed as OPENCODE_CONFIG_CONTENT)
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("config must be JSON-serializable: %v", err)
	}

	// Verify it round-trips
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("config JSON must be parseable: %v", err)
	}
}
