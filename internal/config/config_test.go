package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Engine.Type != "opencode" {
		t.Errorf("expected default engine type 'opencode', got '%s'", cfg.Engine.Type)
	}

	if cfg.Engine.Provider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got '%s'", cfg.Engine.Provider)
	}

	if cfg.Workspace.Path != "/workspace" {
		t.Errorf("expected default workspace '/workspace', got '%s'", cfg.Workspace.Path)
	}

	if !cfg.Discord.Enabled {
		t.Error("expected Discord to be enabled by default")
	}

	if cfg.Starlark.MaxExecutionMs != 30000 {
		t.Errorf("expected Starlark max execution 30000ms, got %d", cfg.Starlark.MaxExecutionMs)
	}

	if cfg.Starlark.MaxMemoryMB != 128 {
		t.Errorf("expected Starlark max memory 128MB, got %d", cfg.Starlark.MaxMemoryMB)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write test config
	configContent := `
engine:
  type: opencode
  provider: anthropic
  model: claude-opus-4-20250514
workspace:
  path: /custom/workspace
discord:
  enabled: false
  allowed_users:
    - "123456"
starlark:
  max_execution_ms: 60000
  max_memory_mb: 256
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set env var to point to our config
	os.Setenv("CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFIG_PATH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Engine.Type != "opencode" {
		t.Errorf("expected engine type 'opencode', got '%s'", cfg.Engine.Type)
	}

	if cfg.Engine.Model != "claude-opus-4-20250514" {
		t.Errorf("expected model 'claude-opus-4-20250514', got '%s'", cfg.Engine.Model)
	}

	if cfg.Workspace.Path != "/custom/workspace" {
		t.Errorf("expected workspace '/custom/workspace', got '%s'", cfg.Workspace.Path)
	}

	if cfg.Discord.Enabled {
		t.Error("expected Discord to be disabled")
	}

	if len(cfg.Discord.AllowedUsers) != 1 || cfg.Discord.AllowedUsers[0] != "123456" {
		t.Errorf("expected allowed_users ['123456'], got %v", cfg.Discord.AllowedUsers)
	}

	if cfg.Starlark.MaxExecutionMs != 60000 {
		t.Errorf("expected max_execution_ms 60000, got %d", cfg.Starlark.MaxExecutionMs)
	}

	if cfg.Starlark.MaxMemoryMB != 256 {
		t.Errorf("expected max_memory_mb 256, got %d", cfg.Starlark.MaxMemoryMB)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	// Create minimal config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("engine:\n  type: opencode"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("CONFIG_PATH", configPath)
	os.Setenv("WORKSPACE_PATH", "/env/workspace")
	defer func() {
		os.Unsetenv("CONFIG_PATH")
		os.Unsetenv("WORKSPACE_PATH")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.Path != "/env/workspace" {
		t.Errorf("expected WORKSPACE_PATH env override '/env/workspace', got '%s'", cfg.Workspace.Path)
	}
}

func TestEnsureDirs(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "workspace")

	w := WorkspaceConfig{Path: workspace}

	// Directories should not exist yet
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatal("workspace should not exist yet")
	}

	// EnsureDirs should create all directories
	if err := w.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs failed: %v", err)
	}

	// Verify all directories were created
	for _, dir := range []string{workspace, w.DataDir(), w.ScriptsDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %s was not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}

	// Calling EnsureDirs again should be a no-op (idempotent)
	if err := w.EnsureDirs(); err != nil {
		t.Fatalf("second EnsureDirs call failed: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	os.Setenv("CONFIG_PATH", "/nonexistent/config.yaml")
	defer os.Unsetenv("CONFIG_PATH")

	// Should not error, just use defaults
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for missing config file, got: %v", err)
	}

	// Should have default values
	if cfg.Engine.Type != "opencode" {
		t.Errorf("expected default engine type, got '%s'", cfg.Engine.Type)
	}
}
