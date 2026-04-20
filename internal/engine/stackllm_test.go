package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-pact/openpact/internal/mcp"
	"github.com/open-pact/openpact/internal/storage"
)

// TestNew_BuildsAllComponents verifies that New() wires all the expected
// fields and creates the data directory under the workspace.
func TestNew_BuildsAllComponents(t *testing.T) {
	workspace := t.TempDir()

	// Build an MCP server with one tool so we can verify the registry is
	// populated from it.
	mcpSrv := mcp.NewServer(nil, nil)
	mcpSrv.RegisterTool(&mcp.Tool{
		Name:        "echo",
		Description: "Echo back the input",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return args["text"], nil
		},
	})

	db := storage.NewTestDB(t)
	stack, err := New(Config{
		WorkspacePath: workspace,
		DB:            db,
		Tools:         mcpSrv,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()

	if stack.Manager == nil {
		t.Fatal("Stack.Manager is nil")
	}
	if stack.Sessions == nil {
		t.Fatal("Stack.Sessions is nil")
	}
	if stack.Tools == nil {
		t.Fatal("Stack.Tools is nil")
	}
	if stack.Handler == nil {
		t.Fatal("Stack.Handler is nil")
	}

	// Registry should have exactly one tool, matching the MCP tool name.
	defs := stack.Tools.Definitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 tool in registry, got %d", len(defs))
	}
	if defs[0].Name != "echo" {
		t.Errorf("expected tool name 'echo', got %q", defs[0].Name)
	}
	if defs[0].Description == "" {
		t.Error("expected non-empty description")
	}

	// Auth + config files should live under <workspace>/secure/data/.
	dataDir := filepath.Join(workspace, "secure", "data")
	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("data dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("data dir path is not a directory")
	}
}

func TestNew_RejectsEmptyWorkspace(t *testing.T) {
	db := storage.NewTestDB(t)
	_, err := New(Config{DB: db})
	if err == nil {
		t.Fatal("expected error for empty WorkspacePath")
	}
}

func TestNew_RejectsNilDB(t *testing.T) {
	_, err := New(Config{WorkspacePath: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for nil DB")
	}
}

// TestNew_NilToolsServer ensures that a Stack can be built without any MCP
// server — useful for tests that don't care about tools — and that the
// registry is simply empty.
func TestNew_NilToolsServer(t *testing.T) {
	workspace := t.TempDir()
	db := storage.NewTestDB(t)
	stack, err := New(Config{WorkspacePath: workspace, DB: db})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()

	if stack.Tools == nil {
		t.Fatal("registry should still be built even without MCP server")
	}
	if defs := stack.Tools.Definitions(); len(defs) != 0 {
		t.Fatalf("expected empty registry, got %d tools", len(defs))
	}
}

// TestMCPToolAdapter_CallDispatchesToHandler checks that the adapter
// correctly unmarshals JSON args and passes them to the underlying MCP
// handler, and stringifies the result.
func TestMCPToolAdapter_CallDispatchesToHandler(t *testing.T) {
	tool := &mcp.Tool{
		Name:        "greet",
		Description: "Greet a name",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			name, _ := args["name"].(string)
			return "Hello, " + name, nil
		},
	}

	adapter := newMCPToolAdapter(tool)

	def := adapter.Definition()
	if def.Name != "greet" {
		t.Errorf("Definition.Name = %q, want %q", def.Name, "greet")
	}
	if def.Parameters == nil {
		t.Error("Parameters should not be nil")
	}

	result, err := adapter.Call(context.Background(), `{"name":"Matt"}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result != "Hello, Matt" {
		t.Errorf("Call result = %q, want %q", result, "Hello, Matt")
	}
}

// TestMCPToolAdapter_EmptyArgs verifies that an empty arguments string is
// dispatched as an empty map (not nil), so handlers that assert non-nil
// args still work.
func TestMCPToolAdapter_EmptyArgs(t *testing.T) {
	var received map[string]interface{}
	tool := &mcp.Tool{
		Name: "noop",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			received = args
			return "ok", nil
		},
	}
	adapter := newMCPToolAdapter(tool)

	result, err := adapter.Call(context.Background(), "")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want ok", result)
	}
	if received == nil {
		t.Error("handler received nil args; should receive empty map")
	}
}

// TestMCPToolAdapter_BadJSONArgs verifies that malformed JSON returns an
// error without invoking the handler.
func TestMCPToolAdapter_BadJSONArgs(t *testing.T) {
	called := false
	tool := &mcp.Tool{
		Name: "never",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			called = true
			return nil, nil
		},
	}
	adapter := newMCPToolAdapter(tool)

	_, err := adapter.Call(context.Background(), "{not json")
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
	if called {
		t.Error("handler should not have been invoked on bad args")
	}
}

// TestMCPToolAdapter_HandlerError propagates the handler error verbatim.
func TestMCPToolAdapter_HandlerError(t *testing.T) {
	sentinel := errors.New("tool failed")
	tool := &mcp.Tool{
		Name: "fail",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return nil, sentinel
		},
	}
	adapter := newMCPToolAdapter(tool)

	_, err := adapter.Call(context.Background(), "{}")
	if !errors.Is(err, sentinel) {
		t.Errorf("got err=%v, want sentinel", err)
	}
}

// TestStringifyToolResult covers the different return-value types a tool
// handler might produce.
func TestStringifyToolResult(t *testing.T) {
	cases := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"bytes", []byte("world"), "world"},
		{"map", map[string]int{"a": 1}, `{"a":1}`},
		{"slice", []string{"a", "b"}, `["a","b"]`},
		{"int", 42, "42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stringifyToolResult(tc.input)
			if got != tc.want {
				t.Errorf("stringifyToolResult(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestRegisterMCPTools_AllRegistered confirms every MCP-registered tool
// ends up in the stackllm registry with the correct name.
func TestRegisterMCPTools_AllRegistered(t *testing.T) {
	srv := mcp.NewServer(nil, nil)
	names := []string{"tool_a", "tool_b", "tool_c"}
	for _, n := range names {
		n := n
		srv.RegisterTool(&mcp.Tool{
			Name: n,
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				return n, nil
			},
		})
	}

	stack, err := New(Config{WorkspacePath: t.TempDir(), DB: storage.NewTestDB(t), Tools: srv})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()

	defs := stack.Tools.Definitions()
	got := make(map[string]bool, len(defs))
	for _, d := range defs {
		got[d.Name] = true
	}
	for _, n := range names {
		if !got[n] {
			t.Errorf("registry missing tool %q", n)
		}
	}
}

// TestStack_HandlerMountsEndpoints smoke-tests that the web.ManagedHandler
// is mounted and responds to GET /providers with a 200.
func TestStack_HandlerMountsEndpoints(t *testing.T) {
	stack, err := New(Config{WorkspacePath: t.TempDir(), DB: storage.NewTestDB(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)
	rec := httptest.NewRecorder()
	stack.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /providers status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
}

// TestStack_SystemPromptRoundTrip verifies Set/Get semantics for the
// orchestrator's system prompt plumbing.
func TestStack_SystemPromptRoundTrip(t *testing.T) {
	stack, err := New(Config{WorkspacePath: t.TempDir(), DB: storage.NewTestDB(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()

	if got := stack.SystemPrompt(); got != "" {
		t.Errorf("initial SystemPrompt = %q, want empty", got)
	}

	stack.SetSystemPrompt("you are Nova")
	if got := stack.SystemPrompt(); got != "you are Nova" {
		t.Errorf("SystemPrompt = %q, want 'you are Nova'", got)
	}
}

// TestStack_DefaultModel_NoneSet returns ok=false before any default is
// persisted.
func TestStack_DefaultModel_NoneSet(t *testing.T) {
	stack, err := New(Config{WorkspacePath: t.TempDir(), DB: storage.NewTestDB(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()

	_, ok, err := stack.DefaultModel(context.Background())
	if err != nil {
		t.Fatalf("DefaultModel: %v", err)
	}
	if ok {
		t.Error("expected ok=false before default is set")
	}
}

// Guard against accidentally losing the data directory between runs by
// reusing the same workspace path — the stackllm config and auth files
// must survive a Stack rebuild.
func TestNew_ReopenPreservesConfig(t *testing.T) {
	workspace := t.TempDir()

	// Stackllm auth lives on disk (file path derived from WorkspacePath),
	// so reopening the stack with a fresh DB still finds the persisted
	// auth file. DB-backed state (session history) would vanish with a
	// fresh in-memory DB, but that's not what this test covers.
	db1 := storage.NewTestDB(t)
	stack, err := New(Config{WorkspacePath: workspace, DB: db1})
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	ctx := context.Background()
	if err := stack.Manager.SaveAPIKey(ctx, "openai", "sk-test-12345"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	stack.Close()

	db2 := storage.NewTestDB(t)
	stack2, err := New(Config{WorkspacePath: workspace, DB: db2})
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	defer stack2.Close()

	statuses, err := stack2.Manager.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	var openaiAuth bool
	for _, s := range statuses {
		if s.Name == "openai" {
			openaiAuth = s.Authenticated
		}
	}
	if !openaiAuth {
		t.Error("OpenAI auth should have persisted across Stack rebuild")
	}

	// Sanity: the auth file exists at the expected path.
	if _, err := os.Stat(filepath.Join(workspace, "secure", "data", "stackllm_auth.json")); err != nil {
		t.Errorf("expected stackllm_auth.json to exist: %v", err)
	}
}

// smoke test the Close path returns nil with a nil Stack.
func TestStack_CloseNil(t *testing.T) {
	var s *Stack
	if err := s.Close(); err != nil {
		t.Errorf("Close on nil Stack returned err=%v", err)
	}
}

// Ensure that repeated Close calls don't blow up.
func TestStack_CloseTwice(t *testing.T) {
	stack, err := New(Config{WorkspacePath: t.TempDir(), DB: storage.NewTestDB(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := stack.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	// Second close may or may not error depending on driver, but must
	// not panic. Swallow whatever it returns.
	_ = stack.Close()
	_ = fmt.Sprintf("%v", stack) // keep stack referenced
}
