package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHtmlToText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "simple paragraph",
			input: "<p>Hello world</p>",
			want:  "Hello world",
		},
		{
			name:  "nested tags",
			input: "<div><p><strong>Bold</strong> text</p></div>",
			want:  "Bold text",
		},
		{
			name:  "script removal",
			input: "<p>Before</p><script>alert('evil');</script><p>After</p>",
			want:  "Before\n\nAfter",
		},
		{
			name:  "style removal",
			input: "<style>.foo { color: red; }</style><p>Content</p>",
			want:  "Content",
		},
		{
			name:  "entity decoding",
			input: "&amp; &lt; &gt; &quot; &nbsp;",
			want:  "& < > \"",
		},
		{
			name:  "whitespace cleanup",
			input: "<p>   Too   many    spaces   </p>",
			want:  "Too many spaces",
		},
		{
			name:  "block elements to newlines",
			input: "<h1>Title</h1><p>Para 1</p><p>Para 2</p>",
			want:  "Title\n\nPara 1\n\nPara 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := htmlToText(tt.input)
			// Normalize for comparison
			got = strings.TrimSpace(got)
			tt.want = strings.TrimSpace(tt.want)
			if got != tt.want {
				t.Errorf("htmlToText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWebFetchTool(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><h1>Test Page</h1><p>Hello from test server</p></body></html>"))
	}))
	defer server.Close()

	tool := webFetchTool()

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"url": server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.(string)
	if !strings.Contains(text, "Test Page") {
		t.Error("expected 'Test Page' in result")
	}
	if !strings.Contains(text, "Hello from test server") {
		t.Error("expected 'Hello from test server' in result")
	}
}

func TestWebFetchToolMissingURL(t *testing.T) {
	tool := webFetchTool()

	_, err := tool.Handler(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestWebFetchToolInvalidScheme(t *testing.T) {
	tool := webFetchTool()

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"url": "ftp://example.com",
	})
	if err == nil {
		t.Error("expected error for invalid scheme")
	}
	if !strings.Contains(err.Error(), "http://") {
		t.Errorf("expected scheme error, got: %v", err)
	}
}

func TestWebFetchToolHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tool := webFetchTool()

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"url": server.URL,
	})
	if err == nil {
		t.Error("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 error, got: %v", err)
	}
}

func TestWebFetchToolMaxLength(t *testing.T) {
	// Create server that returns lots of content
	longContent := strings.Repeat("Hello world. ", 10000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(longContent))
	}))
	defer server.Close()

	tool := webFetchTool()

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"url":        server.URL,
		"max_length": float64(100),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.(string)
	if !strings.Contains(text, "[Content truncated]") {
		t.Error("expected truncation message")
	}
	if len(text) > 200 { // Some buffer for the truncation message
		t.Errorf("content too long: %d chars", len(text))
	}
}

func TestRegisterWebTools(t *testing.T) {
	s := NewServer(nil, nil)
	RegisterWebTools(s)

	tools := s.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "web_fetch" {
		t.Errorf("expected 'web_fetch' tool, got '%s'", tools[0].Name)
	}
}
