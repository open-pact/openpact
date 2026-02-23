package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestRegisterTool(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer(&buf, &buf)

	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return "result", nil
		},
	}

	s.RegisterTool(tool)

	tools := s.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got '%s'", tools[0].Name)
	}
}

func TestHandleInitialize(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer(&buf, &buf)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	result := s.handleInitialize(req)

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if resultMap["protocolVersion"] != "2024-11-05" {
		t.Errorf("unexpected protocol version: %v", resultMap["protocolVersion"])
	}

	serverInfo, ok := resultMap["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("expected serverInfo map")
	}

	if serverInfo["name"] != "openpact-mcp" {
		t.Errorf("expected server name 'openpact-mcp', got '%v'", serverInfo["name"])
	}
}

func TestHandleToolsList(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer(&buf, &buf)

	s.RegisterTool(&Tool{
		Name:        "tool1",
		Description: "First tool",
	})
	s.RegisterTool(&Tool{
		Name:        "tool2",
		Description: "Second tool",
	})

	result := s.handleToolsList()

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	tools, ok := resultMap["tools"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected tools array")
	}

	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestHandleToolCall(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer(&buf, &buf)

	s.RegisterTool(&Tool{
		Name:        "echo",
		Description: "Echo input",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return args["message"], nil
		},
	})

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"message": "hello",
			},
		},
	}

	result, err := s.handleToolCall(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	content, ok := resultMap["content"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected content array")
	}

	if len(content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(content))
	}

	if content[0]["text"] != "hello" {
		t.Errorf("expected 'hello', got '%v'", content[0]["text"])
	}
}

func TestHandleToolCallUnknownTool(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer(&buf, &buf)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "nonexistent",
			"arguments": map[string]interface{}{},
		},
	}

	_, err := s.handleToolCall(context.Background(), req)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestRequestResponse(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      "test-id",
		Method:  "tools/list",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%v'", decoded.ID)
	}

	if decoded.Method != "tools/list" {
		t.Errorf("expected method 'tools/list', got '%s'", decoded.Method)
	}
}

func TestErrorResponse(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Error: &Error{
			Code:    -32601,
			Message: "Method not found",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("expected error in response")
	}

	if decoded.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", decoded.Error.Code)
	}
}
