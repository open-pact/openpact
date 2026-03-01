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

	disabledTools := []string{"bash", "write", "edit", "read", "grep", "glob", "list", "patch", "webfetch", "websearch", "question", "task", "todowrite"}
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

func TestBuildOpenCodeConfig_OverridesDefaultAgentPrompts(t *testing.T) {
	cfg := Config{}
	config := BuildOpenCodeConfig(cfg, "test-token")

	agentSection, ok := config["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("expected agent section in config")
	}

	// Both "build" and "plan" agents should have custom prompts to replace
	// OpenCode's hardcoded defaults (which conflict with OpenPact's security model)
	for _, name := range []string{"build", "plan"} {
		agent, ok := agentSection[name].(map[string]interface{})
		if !ok {
			t.Fatalf("expected %q agent config", name)
		}

		prompt, ok := agent["prompt"].(string)
		if !ok || prompt == "" {
			t.Errorf("expected %q agent to have a non-empty prompt override", name)
		}
	}
}

func TestHandlePartEvent_ToolPreservesRawFields(t *testing.T) {
	o := &OpenCode{}

	// Simulate a tool-type SSE event with "tool" and "state" fields that
	// the typed struct doesn't declare — these must survive in the output.
	sseData := json.RawMessage(`{
		"properties": {
			"part": {
				"id": "part-1",
				"messageID": "msg-assist",
				"type": "tool",
				"tool": "openpact_workspace_write",
				"state": "running",
				"sessionID": "sess-1",
				"time": {"start": 1000}
			}
		}
	}`)

	seenParts := make(map[string]bool)
	userMsgID := "msg-user" // Already learned — so assistant parts pass through
	ch := make(chan Response, 4)

	o.handlePartEvent(sseData, "sess-1", seenParts, &userMsgID, ch)

	if len(ch) != 1 {
		t.Fatalf("expected 1 response, got %d", len(ch))
	}

	resp := <-ch
	if resp.PartType != "tool" {
		t.Fatalf("expected PartType 'tool', got %q", resp.PartType)
	}
	if len(resp.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(resp.Parts))
	}

	// The raw JSON must contain "tool" and "state" fields
	var parsed map[string]interface{}
	if err := json.Unmarshal(resp.Parts[0], &parsed); err != nil {
		t.Fatalf("failed to parse part JSON: %v", err)
	}

	if parsed["tool"] != "openpact_workspace_write" {
		t.Errorf("expected tool field 'openpact_workspace_write', got %v", parsed["tool"])
	}
	if parsed["state"] != "running" {
		t.Errorf("expected state field 'running', got %v", parsed["state"])
	}
}

func TestForwardUnseenParts_UpdatesToolParts(t *testing.T) {
	o := &OpenCode{}

	// Simulate parts from GET reconciliation
	parts := []json.RawMessage{
		// Text part — already seen, should be skipped
		json.RawMessage(`{"id":"p1","type":"text","text":"Hello"}`),
		// Tool part — already seen, should be re-sent with IsUpdate
		json.RawMessage(`{"id":"p2","type":"tool","tool":"openpact_memory_read","state":"completed"}`),
		// New text part — never seen, should be forwarded
		json.RawMessage(`{"id":"p3","type":"text","text":"World"}`),
	}

	seenParts := map[string]bool{
		"p1": true, // Already seen via SSE
		"p2": true, // Already seen via SSE
	}

	ch := make(chan Response, 10)
	o.forwardUnseenParts(parts, "sess-1", seenParts, ch)
	close(ch)

	var responses []Response
	for r := range ch {
		responses = append(responses, r)
	}

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses (tool update + new text), got %d", len(responses))
	}

	// First: tool part re-sent as update
	if responses[0].PartType != "tool" || !responses[0].IsUpdate {
		t.Errorf("expected tool part with IsUpdate=true, got type=%q isUpdate=%v", responses[0].PartType, responses[0].IsUpdate)
	}
	// Verify the tool name is preserved in the raw JSON
	var toolPart map[string]interface{}
	if err := json.Unmarshal(responses[0].Parts[0], &toolPart); err != nil {
		t.Fatalf("failed to parse tool part: %v", err)
	}
	if toolPart["tool"] != "openpact_memory_read" {
		t.Errorf("expected tool 'openpact_memory_read', got %v", toolPart["tool"])
	}

	// Second: new text part (not seen before)
	if responses[1].Content != "World" || responses[1].IsUpdate {
		t.Errorf("expected new text 'World' with IsUpdate=false, got content=%q isUpdate=%v", responses[1].Content, responses[1].IsUpdate)
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
