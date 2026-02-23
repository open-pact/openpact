package engine

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestBuildFilteredEnv_ExcludesSecrets(t *testing.T) {
	// Set up test environment
	os.Setenv("DISCORD_TOKEN", "secret-discord")
	os.Setenv("GITHUB_TOKEN", "secret-github")
	os.Setenv("SLACK_BOT_TOKEN", "secret-slack")
	os.Setenv("TELEGRAM_BOT_TOKEN", "secret-telegram")
	os.Setenv("ADMIN_JWT_SECRET", "secret-jwt")
	defer func() {
		os.Unsetenv("DISCORD_TOKEN")
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("SLACK_BOT_TOKEN")
		os.Unsetenv("TELEGRAM_BOT_TOKEN")
		os.Unsetenv("ADMIN_JWT_SECRET")
	}()

	env := buildFilteredEnv(Config{})
	envMap := envToMap(env)

	excluded := []string{"DISCORD_TOKEN", "GITHUB_TOKEN", "SLACK_BOT_TOKEN", "TELEGRAM_BOT_TOKEN", "ADMIN_JWT_SECRET"}
	for _, key := range excluded {
		if _, ok := envMap[key]; ok {
			t.Errorf("expected %s to be excluded from filtered env", key)
		}
	}
}

func TestBuildFilteredEnv_IncludesAllowlisted(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	os.Setenv("OPENAI_API_KEY", "test-openai")
	defer func() {
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("OPENAI_API_KEY")
	}()

	env := buildFilteredEnv(Config{})
	envMap := envToMap(env)

	// PATH and HOME should always be present (they're set in any environment)
	if _, ok := envMap["PATH"]; !ok {
		t.Error("expected PATH to be included")
	}

	// LLM provider keys should be included
	if v, ok := envMap["ANTHROPIC_API_KEY"]; !ok || v != "test-key" {
		t.Error("expected ANTHROPIC_API_KEY to be included with correct value")
	}
	if v, ok := envMap["OPENAI_API_KEY"]; !ok || v != "test-openai" {
		t.Error("expected OPENAI_API_KEY to be included with correct value")
	}
}

func TestBuildFilteredEnv_IncludesXDG(t *testing.T) {
	os.Setenv("XDG_CONFIG_HOME", "/test/config")
	os.Setenv("XDG_DATA_HOME", "/test/data")
	defer func() {
		os.Unsetenv("XDG_CONFIG_HOME")
		os.Unsetenv("XDG_DATA_HOME")
	}()

	env := buildFilteredEnv(Config{})
	envMap := envToMap(env)

	if v, ok := envMap["XDG_CONFIG_HOME"]; !ok || v != "/test/config" {
		t.Error("expected XDG_CONFIG_HOME to be included")
	}
	if v, ok := envMap["XDG_DATA_HOME"]; !ok || v != "/test/data" {
		t.Error("expected XDG_DATA_HOME to be included")
	}
}

func TestBuildFilteredEnv_ExcludesUnknownVars(t *testing.T) {
	os.Setenv("RANDOM_SECRET_VAR", "should-not-pass")
	os.Setenv("DATABASE_URL", "postgres://secret")
	defer func() {
		os.Unsetenv("RANDOM_SECRET_VAR")
		os.Unsetenv("DATABASE_URL")
	}()

	env := buildFilteredEnv(Config{})
	envMap := envToMap(env)

	if _, ok := envMap["RANDOM_SECRET_VAR"]; ok {
		t.Error("expected RANDOM_SECRET_VAR to be excluded")
	}
	if _, ok := envMap["DATABASE_URL"]; ok {
		t.Error("expected DATABASE_URL to be excluded")
	}
}

func TestBuildFilteredEnv_OverridesHOMEForRunAsUser(t *testing.T) {
	// This test only works if there's a user to look up.
	// We test with the current user as a known-good value.
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		t.Skip("USER env var not set")
	}

	env := buildFilteredEnv(Config{RunAsUser: currentUser})
	envMap := envToMap(env)

	if v, ok := envMap["USER"]; !ok || v != currentUser {
		t.Errorf("expected USER=%s, got %s", currentUser, v)
	}
	if _, ok := envMap["HOME"]; !ok {
		t.Error("expected HOME to be set when RunAsUser is configured")
	}
}

func TestBuildFilteredEnv_GracefulWithInvalidUser(t *testing.T) {
	// Invalid user shouldn't crash, just skip the HOME override
	env := buildFilteredEnv(Config{RunAsUser: "nonexistent-user-xyzzy"})
	if len(env) == 0 {
		t.Error("expected non-empty env even with invalid RunAsUser")
	}
}

func TestBuildOpenCodeConfig_DisablesBuiltinTools(t *testing.T) {
	cfg := Config{}
	config := buildOpenCodeConfig(cfg)

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
	config := buildOpenCodeConfig(cfg)

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

	// findMCPBinary looks next to os.Executable(), which we can't override,
	// but we can verify it falls back to PATH lookup
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	path, err := findMCPBinary()
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

	_, err := findMCPBinary()
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
		DataDir: "/workspace/data",
		MCPEnv: map[string]string{
			"OPENPACT_FEATURES": "scripts,github",
		},
	}
	config := buildOpenCodeConfig(cfg)

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
	if mcpEnv["OPENPACT_DATA_DIR"] != "/workspace/data" {
		t.Errorf("expected OPENPACT_DATA_DIR=/workspace/data, got %s", mcpEnv["OPENPACT_DATA_DIR"])
	}
	if mcpEnv["OPENPACT_FEATURES"] != "scripts,github" {
		t.Errorf("expected OPENPACT_FEATURES=scripts,github, got %s", mcpEnv["OPENPACT_FEATURES"])
	}
}

func TestBuildOpenCodeConfig_NoMCPWhenBinaryMissing(t *testing.T) {
	// Empty PATH so mcp-server can't be found
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	cfg := Config{}
	config := buildOpenCodeConfig(cfg)

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
		DataDir: "/workspace/data",
	}
	config := buildOpenCodeConfig(cfg)

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

func TestSetSysProcCredential_InvalidUser(t *testing.T) {
	cmd := exec.Command("echo")
	err := setSysProcCredential(cmd, "nonexistent-user-xyzzy-12345")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestSetSysProcCredential_ValidUser(t *testing.T) {
	// Use current user as a known-valid user
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		t.Skip("USER env var not set")
	}

	cmd := exec.Command("echo")
	err := setSysProcCredential(cmd, currentUser)
	if err != nil {
		t.Fatalf("unexpected error for current user: %v", err)
	}

	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set")
	}
	if cmd.SysProcAttr.Credential == nil {
		t.Fatal("expected Credential to be set")
	}
}

func TestFilterEnvKey(t *testing.T) {
	env := []string{"HOME=/home/test", "PATH=/usr/bin", "HOME=/other"}
	result := filterEnvKey(env, "HOME")

	for _, e := range result {
		if strings.HasPrefix(e, "HOME=") {
			t.Errorf("expected HOME to be filtered out, found: %s", e)
		}
	}

	// PATH should remain
	found := false
	for _, e := range result {
		if strings.HasPrefix(e, "PATH=") {
			found = true
		}
	}
	if !found {
		t.Error("expected PATH to remain after filtering HOME")
	}
}

// envToMap converts a []string env to a map for easier testing.
func envToMap(env []string) map[string]string {
	m := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
