package starlark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader(t *testing.T) {
	sandbox := New(Config{})
	loader := NewLoader("/tmp/scripts", sandbox)
	if loader == nil {
		t.Fatal("NewLoader returned nil")
	}
}

func TestLoadFile(t *testing.T) {
	// Create temp directory
	dir := t.TempDir()

	// Create a test script
	scriptPath := filepath.Join(dir, "test.star")
	content := `# @description: A test script
# @author: test

def greet(name):
    return "Hello, " + name
`
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	script, err := loader.LoadFile(scriptPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if script.Name != "test" {
		t.Errorf("Name = %q, want %q", script.Name, "test")
	}
	if script.Description != "A test script" {
		t.Errorf("Description = %q, want %q", script.Description, "A test script")
	}
	if script.Metadata["author"] != "test" {
		t.Errorf("author = %q, want %q", script.Metadata["author"], "test")
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()

	// Create a test script
	scriptPath := filepath.Join(dir, "hello.star")
	content := `result = "hello"`
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	script, err := loader.Load("hello")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if script.Name != "hello" {
		t.Errorf("Name = %q, want %q", script.Name, "hello")
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	_, err := loader.Load("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent script")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()

	// Create some test scripts
	scripts := []string{"one.star", "two.star", "three.star"}
	for _, name := range scripts {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("result = 1"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Also create a non-script file
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a script"), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	list, err := loader.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("len = %d, want 3", len(list))
	}
}

func TestListEmptyDir(t *testing.T) {
	dir := t.TempDir()

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	list, err := loader.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}

func TestListNonexistentDir(t *testing.T) {
	sandbox := New(Config{})
	loader := NewLoader("/nonexistent/path", sandbox)

	list, err := loader.List()
	if err != nil {
		t.Fatalf("List should not fail for nonexistent dir: %v", err)
	}

	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}

func TestCaching(t *testing.T) {
	dir := t.TempDir()

	scriptPath := filepath.Join(dir, "cached.star")
	if err := os.WriteFile(scriptPath, []byte("result = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	// Load first time
	script1, err := loader.Load("cached")
	if err != nil {
		t.Fatal(err)
	}

	// Load second time (should be cached)
	script2, err := loader.Load("cached")
	if err != nil {
		t.Fatal(err)
	}

	// Should be the same pointer
	if script1 != script2 {
		t.Error("expected cached script to be the same instance")
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()

	scriptPath := filepath.Join(dir, "getme.star")
	if err := os.WriteFile(scriptPath, []byte("result = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	// Before loading
	_, ok := loader.Get("getme")
	if ok {
		t.Error("expected Get to return false before Load")
	}

	// Load it
	loader.Load("getme")

	// After loading
	script, ok := loader.Get("getme")
	if !ok {
		t.Error("expected Get to return true after Load")
	}
	if script.Name != "getme" {
		t.Errorf("Name = %q, want %q", script.Name, "getme")
	}
}

func TestCount(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, string(rune('a'+i))+".star")
		os.WriteFile(name, []byte("result = 1"), 0644)
	}

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	if loader.Count() != 0 {
		t.Errorf("initial count = %d, want 0", loader.Count())
	}

	loader.List() // This loads all scripts

	if loader.Count() != 5 {
		t.Errorf("after List count = %d, want 5", loader.Count())
	}
}

func TestReload(t *testing.T) {
	dir := t.TempDir()

	scriptPath := filepath.Join(dir, "reload.star")
	if err := os.WriteFile(scriptPath, []byte("result = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox := New(Config{})
	loader := NewLoader(dir, sandbox)

	loader.Load("reload")
	if loader.Count() != 1 {
		t.Fatalf("count = %d, want 1", loader.Count())
	}

	// Clear and reload
	if err := loader.Reload(); err != nil {
		t.Fatal(err)
	}

	// Count should still be 1 after reload (List was called)
	if loader.Count() != 1 {
		t.Errorf("after reload count = %d, want 1", loader.Count())
	}
}

func TestParseMetadata(t *testing.T) {
	script := &Script{
		Source: `# @description: Does something cool
# @author: Alice
# @version: 1.0.0
# @tags: utility, helper

def main():
    pass
`,
		Metadata: make(map[string]string),
	}

	script.parseMetadata()

	if script.Description != "Does something cool" {
		t.Errorf("Description = %q", script.Description)
	}
	if script.Metadata["author"] != "Alice" {
		t.Errorf("author = %q", script.Metadata["author"])
	}
	if script.Metadata["version"] != "1.0.0" {
		t.Errorf("version = %q", script.Metadata["version"])
	}
	if script.Metadata["tags"] != "utility, helper" {
		t.Errorf("tags = %q", script.Metadata["tags"])
	}
}
