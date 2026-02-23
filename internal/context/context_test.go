package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLoader(t *testing.T) {
	l := NewLoader("/workspace")
	if l.workspacePath != "/workspace" {
		t.Errorf("expected workspace '/workspace', got '%s'", l.workspacePath)
	}
}

func TestLoadEmptyWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLoader(tmpDir)

	prompt, err := l.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Even with no workspace files, the hardcoded openpact section is always present
	if !strings.Contains(prompt, "<openpact>") {
		t.Error("prompt should always contain <openpact> section, even with empty workspace")
	}

	if !strings.Contains(prompt, "How You Work") {
		t.Error("prompt should contain OpenPact framework docs")
	}
}

func TestLoadSoul(t *testing.T) {
	tmpDir := t.TempDir()

	// Write SOUL.md
	soulContent := "I am a helpful assistant."
	if err := os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte(soulContent), 0644); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(tmpDir)
	prompt, err := l.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "I am a helpful assistant.") {
		t.Error("prompt should contain SOUL.md content")
	}

	if !strings.Contains(prompt, "<identity>") {
		t.Error("prompt should have identity tag")
	}
}

func TestLoadAllContextFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create all context files
	files := map[string]string{
		"SOUL.md":   "I am Remy, a helpful fox.",
		"USER.md":   "Matt is a software engineer.",
		"MEMORY.md": "- User prefers concise answers",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create daily memory
	today := time.Now().Format("2006-01-02")
	memoryDir := filepath.Join(tmpDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatal(err)
	}
	dailyContent := "Today we worked on OpenPact."
	if err := os.WriteFile(filepath.Join(memoryDir, today+".md"), []byte(dailyContent), 0644); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(tmpDir)
	prompt, err := l.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check all sections are present
	expectedContents := []string{
		"<openpact>",
		"How You Work",
		"I am Remy, a helpful fox.",
		"Matt is a software engineer.",
		"User prefers concise answers",
		"Today we worked on OpenPact.",
		"<identity>",
		"<user-profile>",
		"<long-term-memory>",
		"<todays-memory-",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt should contain '%s'", expected)
		}
	}
}

func TestOpenPactContentAlwaysFirst(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a SOUL.md so we can verify ordering
	if err := os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte("My identity"), 0644); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(tmpDir)
	prompt, err := l.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openpactIdx := strings.Index(prompt, "<openpact>")
	identityIdx := strings.Index(prompt, "<identity>")

	if openpactIdx == -1 {
		t.Fatal("prompt missing <openpact> section")
	}
	if identityIdx == -1 {
		t.Fatal("prompt missing <identity> section")
	}
	if openpactIdx >= identityIdx {
		t.Error("openpact section should come before identity section")
	}
}

func TestOpenPactContentIncludesMCPTools(t *testing.T) {
	if !strings.Contains(OpenPactContent, "MCP tools") {
		t.Error("OpenPactContent should reference MCP tools")
	}
	if !strings.Contains(OpenPactContent, "workspace_read") {
		t.Error("OpenPactContent should document workspace_read tool")
	}
	if !strings.Contains(OpenPactContent, "workspace_write") {
		t.Error("OpenPactContent should document workspace_write tool")
	}
	if !strings.Contains(OpenPactContent, "MEMORY.md") {
		t.Error("OpenPactContent should document memory file paths")
	}
}

func TestLoadFile(t *testing.T) {
	tmpDir := t.TempDir()

	content := "Test content"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(tmpDir)
	result, err := l.LoadFile("test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != content {
		t.Errorf("expected '%s', got '%s'", content, result)
	}
}

func TestLoadFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLoader(tmpDir)

	result, err := l.LoadFile("nonexistent.md")
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty string for missing file, got: %s", result)
	}
}

func TestListDailyMemories(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")

	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create some memory files
	dates := []string{"2026-02-01", "2026-02-02", "2026-02-03"}
	for _, date := range dates {
		path := filepath.Join(memoryDir, date+".md")
		if err := os.WriteFile(path, []byte("Memory for "+date), 0644); err != nil {
			t.Fatal(err)
		}
	}

	l := NewLoader(tmpDir)
	result, err := l.ListDailyMemories()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 memories, got %d", len(result))
	}
}

func TestListDailyMemoriesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLoader(tmpDir)

	result, err := l.ListDailyMemories()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 memories for missing dir, got %d", len(result))
	}
}

func TestToTagName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Identity", "identity"},
		{"User Profile", "user-profile"},
		{"Long-Term Memory", "long-term-memory"},
		{"Today's Memory (2026-02-03)", "todays-memory-2026-02-03"},
	}

	for _, tt := range tests {
		result := toTagName(tt.input)
		if result != tt.expected {
			t.Errorf("toTagName(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetPaths(t *testing.T) {
	l := NewLoader("/workspace")

	dailyPath := l.GetDailyMemoryPath("2026-02-03")
	if dailyPath != "/workspace/memory/2026-02-03.md" {
		t.Errorf("unexpected daily path: %s", dailyPath)
	}

	longTermPath := l.GetLongTermMemoryPath()
	if longTermPath != "/workspace/MEMORY.md" {
		t.Errorf("unexpected long-term path: %s", longTermPath)
	}
}
