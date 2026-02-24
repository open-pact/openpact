package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServerWithTool() *Server {
	s := NewServer(nil, nil)
	s.RegisterTool(&Tool{
		Name:        "echo",
		Description: "Echo input",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return args["message"], nil
		},
	})
	return s
}

func postJSONRPC(t *testing.T, handler http.Handler, req Request) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	handler.ServeHTTP(w, r)
	return w
}

func TestHTTPHandler_Initialize(t *testing.T) {
	s := newTestServerWithTool()
	handler := s.HTTPHandler()

	w := postJSONRPC(t, handler, Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("unexpected protocol version: %v", result["protocolVersion"])
	}
}

func TestHTTPHandler_ToolsList(t *testing.T) {
	s := newTestServerWithTool()
	handler := s.HTTPHandler()

	w := postJSONRPC(t, handler, Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("expected tools array")
	}

	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
}

func TestHTTPHandler_ToolCall(t *testing.T) {
	s := newTestServerWithTool()
	handler := s.HTTPHandler()

	w := postJSONRPC(t, handler, Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "echo",
			"arguments": map[string]interface{}{"message": "hello"},
		},
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("expected non-empty content array")
	}

	item, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected content item to be a map")
	}

	if item["text"] != "hello" {
		t.Errorf("expected text 'hello', got %v", item["text"])
	}
}

func TestHTTPHandler_UnknownMethod(t *testing.T) {
	s := newTestServerWithTool()
	handler := s.HTTPHandler()

	w := postJSONRPC(t, handler, Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "unknown/method",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (JSON-RPC errors are in the body), got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected JSON-RPC error for unknown method")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestHTTPHandler_MethodNotAllowed_GET(t *testing.T) {
	s := newTestServerWithTool()
	handler := s.HTTPHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", w.Code)
	}
}

func TestHTTPHandler_MethodNotAllowed_DELETE(t *testing.T) {
	s := newTestServerWithTool()
	handler := s.HTTPHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for DELETE, got %d", w.Code)
	}
}

func TestBearerTokenMiddleware_AcceptsValidToken(t *testing.T) {
	s := newTestServerWithTool()
	token := "test-secret-token"
	handler := BearerTokenMiddleware(token, s.HTTPHandler())

	body, _ := json.Marshal(Request{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", w.Code)
	}
}

func TestBearerTokenMiddleware_RejectsNoToken(t *testing.T) {
	s := newTestServerWithTool()
	handler := BearerTokenMiddleware("secret", s.HTTPHandler())

	body, _ := json.Marshal(Request{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", w.Code)
	}
}

func TestBearerTokenMiddleware_RejectsWrongToken(t *testing.T) {
	s := newTestServerWithTool()
	handler := BearerTokenMiddleware("correct-token", s.HTTPHandler())

	body, _ := json.Marshal(Request{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer wrong-token")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong token, got %d", w.Code)
	}
}

func TestGenerateToken(t *testing.T) {
	token1, err := GenerateToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if len(token1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected 64-char hex token, got %d chars", len(token1))
	}

	// Tokens should be unique
	token2, err := GenerateToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if token1 == token2 {
		t.Error("expected unique tokens")
	}
}

func TestHTTPHandler_InvalidJSON(t *testing.T) {
	s := newTestServerWithTool()
	handler := s.HTTPHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (JSON-RPC parse error is in body), got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected JSON-RPC parse error")
	}

	if resp.Error.Code != -32700 {
		t.Errorf("expected error code -32700 (parse error), got %d", resp.Error.Code)
	}
}
