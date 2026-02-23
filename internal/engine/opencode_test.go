package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestBuildOpenCodeConfig_DisablesBuiltinTools(t *testing.T) {
	cfg := Config{}
	config := BuildOpenCodeConfig(cfg)

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
	config := BuildOpenCodeConfig(cfg)

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

func TestBuildOpenCodeConfig_ConfiguresMCPWhenBinaryExists(t *testing.T) {
	// Create a fake mcp-server on PATH so auto-discovery works
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/mcp-server", []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	cfg := Config{
		WorkDir: "/workspace",
	}
	config := BuildOpenCodeConfig(cfg)

	mcpSection, ok := config["mcp"].(map[string]interface{})
	if !ok {
		t.Fatal("expected mcp section in config")
	}

	openpact, ok := mcpSection["openpact"].(map[string]interface{})
	if !ok {
		t.Fatal("expected openpact MCP server config")
	}

	if openpact["type"] != "local" {
		t.Error("expected mcp type to be 'local'")
	}

	command, ok := openpact["command"].([]string)
	if !ok || len(command) != 1 {
		t.Errorf("expected command to be a single-element slice, got %v", command)
	}

	mcpEnv, ok := openpact["environment"].(map[string]string)
	if !ok {
		t.Fatal("expected environment map in MCP config")
	}

	if mcpEnv["OPENPACT_WORKSPACE_PATH"] != "/workspace" {
		t.Errorf("expected OPENPACT_WORKSPACE_PATH=/workspace, got %s", mcpEnv["OPENPACT_WORKSPACE_PATH"])
	}
}

func TestBuildOpenCodeConfig_NoMCPWhenBinaryMissing(t *testing.T) {
	// Empty PATH so mcp-server can't be found
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	cfg := Config{}
	config := BuildOpenCodeConfig(cfg)

	if _, ok := config["mcp"]; ok {
		t.Error("expected no mcp section when mcp-server binary not found")
	}
}

func TestBuildOpenCodeConfig_ValidJSON(t *testing.T) {
	// Create a fake mcp-server on PATH
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/mcp-server", []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	cfg := Config{
		WorkDir: "/workspace",
	}
	config := BuildOpenCodeConfig(cfg)

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
