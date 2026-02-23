package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterScriptTools(t *testing.T) {
	dir := t.TempDir()
	srv := NewServer(nil, nil)

	RegisterScriptTools(srv, ScriptConfig{
		ScriptsDir:     dir,
		MaxExecutionMs: 5000,
	})

	// Check all tools are registered
	tools := []string{"script_list", "script_run", "script_exec", "script_reload"}
	for _, name := range tools {
		if _, ok := srv.tools[name]; !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestScriptList(t *testing.T) {
	dir := t.TempDir()

	// Create test scripts
	script1 := `# @description: First script
result = 1`
	script2 := `# @description: Second script
result = 2`

	os.WriteFile(filepath.Join(dir, "one.star"), []byte(script1), 0644)
	os.WriteFile(filepath.Join(dir, "two.star"), []byte(script2), 0644)

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	result, err := srv.tools["script_list"].Handler(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["count"] != 2 {
		t.Errorf("count = %v, want 2", m["count"])
	}
}

func TestScriptRun(t *testing.T) {
	dir := t.TempDir()

	script := `# @description: Greeting script
def greet(name):
    return "Hello, " + name

result = greet("World")
`
	os.WriteFile(filepath.Join(dir, "greet.star"), []byte(script), 0644)

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	result, err := srv.tools["script_run"].Handler(ctx, map[string]interface{}{
		"name": "greet",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["value"] != "Hello, World" {
		t.Errorf("value = %v, want 'Hello, World'", m["value"])
	}
	if m["error"] != "" {
		t.Errorf("unexpected error: %v", m["error"])
	}
}

func TestScriptRunFunction(t *testing.T) {
	dir := t.TempDir()

	script := `
def add(a, b):
    return a + b

def multiply(a, b):
    return a * b
`
	os.WriteFile(filepath.Join(dir, "math.star"), []byte(script), 0644)

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	result, err := srv.tools["script_run"].Handler(ctx, map[string]interface{}{
		"name":     "math",
		"function": "add",
		"args":     []interface{}{10, 20},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["value"] != int64(30) {
		t.Errorf("value = %v, want 30", m["value"])
	}
}

func TestScriptRunNotFound(t *testing.T) {
	dir := t.TempDir()

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	_, err := srv.tools["script_run"].Handler(ctx, map[string]interface{}{
		"name": "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent script")
	}
}

func TestScriptExec(t *testing.T) {
	dir := t.TempDir()

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	result, err := srv.tools["script_exec"].Handler(ctx, map[string]interface{}{
		"code": "result = [x * 2 for x in range(5)]",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	list := m["value"].([]interface{})
	if len(list) != 5 {
		t.Errorf("len = %d, want 5", len(list))
	}
	if list[4] != int64(8) {
		t.Errorf("list[4] = %v, want 8", list[4])
	}
}

func TestScriptExecWithMain(t *testing.T) {
	dir := t.TempDir()

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	result, err := srv.tools["script_exec"].Handler(ctx, map[string]interface{}{
		"code": "def main():\n    return {'status': 'ok', 'count': 42}",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	value := m["value"].(map[string]interface{})
	if value["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", value["status"])
	}
}

func TestScriptExecError(t *testing.T) {
	dir := t.TempDir()

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	result, err := srv.tools["script_exec"].Handler(ctx, map[string]interface{}{
		"code": "result = 1 / 0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["error"] == "" {
		t.Error("expected error for division by zero")
	}
}

func TestScriptExecJSON(t *testing.T) {
	dir := t.TempDir()

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	result, err := srv.tools["script_exec"].Handler(ctx, map[string]interface{}{
		"code": "data = json.decode('{\"name\": \"test\"}')\nresult = data['name']",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["value"] != "test" {
		t.Errorf("value = %v, want 'test'", m["value"])
	}
}

func TestScriptReload(t *testing.T) {
	dir := t.TempDir()

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()

	// First reload with empty dir
	result, err := srv.tools["script_reload"].Handler(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["count"] != 0 {
		t.Errorf("count = %v, want 0", m["count"])
	}

	// Add a script
	os.WriteFile(filepath.Join(dir, "new.star"), []byte("result = 1"), 0644)

	// Reload
	result, err = srv.tools["script_reload"].Handler(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m = result.(map[string]interface{})
	if m["count"] != 1 {
		t.Errorf("after add count = %v, want 1", m["count"])
	}
}

func TestScriptDuration(t *testing.T) {
	dir := t.TempDir()

	srv := NewServer(nil, nil)
	RegisterScriptTools(srv, ScriptConfig{ScriptsDir: dir})

	ctx := context.Background()
	result, err := srv.tools["script_exec"].Handler(ctx, map[string]interface{}{
		"code": "time.sleep(0.05)\nresult = 'done'",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	durationMs := m["duration_ms"].(int64)
	if durationMs < 40 {
		t.Errorf("duration_ms = %d, expected >= 40", durationMs)
	}
}
