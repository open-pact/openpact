package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Engine.DBPath != "" {
		t.Errorf("expected default DBPath to be empty (so workspace default is used), got '%s'", cfg.Engine.DBPath)
	}

	if cfg.Workspace.Path != "/workspace" {
		t.Errorf("expected default workspace '/workspace', got '%s'", cfg.Workspace.Path)
	}

	if !cfg.Admin.Enabled {
		t.Error("expected Admin to be enabled by default")
	}
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// YAML now only carries bootstrap fields. Runtime settings
	// (logging/ratelimit/starlark/integrations) live in the DB and are
	// not accepted from YAML.
	configContent := `
engine:
  db_path: /custom/path/sessions.db
workspace:
  path: /custom/workspace
admin:
  bind: 127.0.0.1:9999
  allowlist:
    - trusted.star
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFIG_PATH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Engine.DBPath != "/custom/path/sessions.db" {
		t.Errorf("expected DBPath '/custom/path/sessions.db', got '%s'", cfg.Engine.DBPath)
	}

	if cfg.Workspace.Path != "/custom/workspace" {
		t.Errorf("expected workspace '/custom/workspace', got '%s'", cfg.Workspace.Path)
	}

	if cfg.Admin.Bind != "127.0.0.1:9999" {
		t.Errorf("expected admin bind '127.0.0.1:9999', got '%s'", cfg.Admin.Bind)
	}

	if len(cfg.Admin.Allowlist) != 1 || cfg.Admin.Allowlist[0] != "trusted.star" {
		t.Errorf("expected allowlist [trusted.star], got %v", cfg.Admin.Allowlist)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("admin:\n  enabled: true"), 0644); err != nil {
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

	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatal("workspace should not exist yet")
	}

	if err := w.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs failed: %v", err)
	}

	expectedDirs := []string{
		workspace,
		w.SecureDir(),
		w.DataDir(),
		w.AIDataDir(),
		w.ScriptsDir(),
		filepath.Join(w.AIDataDir(), "memory"),
		filepath.Join(w.AIDataDir(), "skills"),
	}
	for _, dir := range expectedDirs {
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

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for missing config file, got: %v", err)
	}

	if cfg.Engine.DBPath != "" {
		t.Errorf("expected empty DBPath default, got '%s'", cfg.Engine.DBPath)
	}
	if cfg.Workspace.Path == "" {
		t.Error("expected workspace path default")
	}
}
