package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultReadTool(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\nHello world"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := VaultConfig{Path: tmpDir}
	tool := vaultReadTool(cfg)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "test.md",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := result.(string)
	if !strings.Contains(content, "Hello world") {
		t.Errorf("expected content to contain 'Hello world', got: %s", content)
	}
}

func TestVaultReadToolNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := VaultConfig{Path: tmpDir}
	tool := vaultReadTool(cfg)

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "nonexistent.md",
	})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestVaultReadToolPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := VaultConfig{Path: tmpDir}
	tool := vaultReadTool(cfg)

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "../../../etc/passwd",
	})
	if err == nil {
		t.Error("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Errorf("expected 'escapes' error, got: %v", err)
	}
}

func TestVaultWriteTool(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := VaultConfig{Path: tmpDir, AutoSync: false}
	tool := vaultWriteTool(cfg)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":    "Projects/test.md",
		"content": "# Test\nNew content",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.(string), "Wrote") {
		t.Errorf("expected success message, got: %v", result)
	}

	// Verify file exists
	content, err := os.ReadFile(filepath.Join(tmpDir, "Projects", "test.md"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(content), "New content") {
		t.Error("file content mismatch")
	}
}

func TestVaultWriteToolPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := VaultConfig{Path: tmpDir}
	tool := vaultWriteTool(cfg)

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":    "../../../tmp/evil.md",
		"content": "bad stuff",
	})
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestVaultListTool(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test files
	os.MkdirAll(filepath.Join(tmpDir, "Projects"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("readme"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "Projects", "test.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)

	cfg := VaultConfig{Path: tmpDir}
	tool := vaultListTool(cfg)

	result, err := tool.Handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := result.(string)
	if !strings.Contains(files, "README.md") {
		t.Error("expected README.md in list")
	}
	if !strings.Contains(files, "Projects/") {
		t.Error("expected Projects/ in list")
	}
	if strings.Contains(files, ".hidden") {
		t.Error("hidden files should be excluded")
	}
}

func TestVaultListToolRecursive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure
	os.MkdirAll(filepath.Join(tmpDir, "A", "B"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.md"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "A", "a.md"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "A", "B", "b.md"), []byte("b"), 0644)

	cfg := VaultConfig{Path: tmpDir}
	tool := vaultListTool(cfg)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"recursive": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := result.(string)
	if !strings.Contains(files, "root.md") {
		t.Error("expected root.md")
	}
	if !strings.Contains(files, "A/a.md") {
		t.Error("expected A/a.md")
	}
	if !strings.Contains(files, "A/B/b.md") {
		t.Error("expected A/B/b.md")
	}
}

func TestVaultSearchTool(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "apple.md"), []byte("I like apples"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "banana.md"), []byte("I like bananas"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "orange.md"), []byte("I like oranges"), 0644)

	cfg := VaultConfig{Path: tmpDir}
	tool := vaultSearchTool(cfg)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "apple",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := result.(string)
	if !strings.Contains(files, "apple.md") {
		t.Error("expected to find apple.md")
	}
	if strings.Contains(files, "banana.md") {
		t.Error("should not find banana.md")
	}
}

func TestVaultSearchToolCaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("UPPERCASE content"), 0644)

	cfg := VaultConfig{Path: tmpDir}
	tool := vaultSearchTool(cfg)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "uppercase",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.(string), "test.md") {
		t.Error("search should be case-insensitive")
	}
}

func TestVaultSearchToolNoResults(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("some content"), 0644)

	cfg := VaultConfig{Path: tmpDir}
	tool := vaultSearchTool(cfg)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "nonexistent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.(string), "No files") {
		t.Errorf("expected 'No files' message, got: %v", result)
	}
}

func TestRegisterVaultTools(t *testing.T) {
	s := NewServer(nil, nil)
	cfg := VaultConfig{Path: "/vault"}

	RegisterVaultTools(s, cfg)

	tools := s.ListTools()
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	expected := []string{"vault_read", "vault_write", "vault_list", "vault_search"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}
}
