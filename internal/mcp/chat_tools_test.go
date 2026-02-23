package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockChatLookup implements ChatProviderLookup for testing.
type mockChatLookup struct {
	activeProviders []string
	sentProvider    string
	sentTarget      string
	sentMessage     string
	sendErr         error
}

func (m *mockChatLookup) GetActiveProviderNames() []string {
	return m.activeProviders
}

func (m *mockChatLookup) SendViaProvider(provider, target, content string) error {
	m.sentProvider = provider
	m.sentTarget = target
	m.sentMessage = content
	return m.sendErr
}

func TestChatSendTool(t *testing.T) {
	lookup := &mockChatLookup{
		activeProviders: []string{"discord", "telegram"},
	}

	tool := chatSendTool(lookup)

	if tool.Name != "chat_send" {
		t.Errorf("expected name 'chat_send', got '%s'", tool.Name)
	}

	// Test sending a message
	args := map[string]interface{}{
		"provider": "discord",
		"target":   "user:123456",
		"message":  "Hello, world!",
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.sentProvider != "discord" {
		t.Errorf("expected provider 'discord', got '%s'", lookup.sentProvider)
	}
	if lookup.sentTarget != "user:123456" {
		t.Errorf("expected target 'user:123456', got '%s'", lookup.sentTarget)
	}
	if lookup.sentMessage != "Hello, world!" {
		t.Errorf("expected message 'Hello, world!', got '%s'", lookup.sentMessage)
	}
	if !strings.Contains(result.(string), "sent") {
		t.Errorf("expected success message, got: %v", result)
	}
}

func TestChatSendToolMissingFields(t *testing.T) {
	lookup := &mockChatLookup{
		activeProviders: []string{"discord"},
	}
	tool := chatSendTool(lookup)

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "missing provider",
			args: map[string]interface{}{"target": "user:123", "message": "Hello"},
		},
		{
			name: "missing target",
			args: map[string]interface{}{"provider": "discord", "message": "Hello"},
		},
		{
			name: "missing message",
			args: map[string]interface{}{"provider": "discord", "target": "user:123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Handler(context.Background(), tt.args)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestChatSendToolSenderError(t *testing.T) {
	lookup := &mockChatLookup{
		activeProviders: []string{"discord"},
		sendErr:         fmt.Errorf("send failed"),
	}
	tool := chatSendTool(lookup)

	args := map[string]interface{}{
		"provider": "discord",
		"target":   "user:123",
		"message":  "Hello",
	}

	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error when sender fails")
	}
	if !strings.Contains(err.Error(), "send failed") {
		t.Errorf("expected 'send failed' in error, got: %v", err)
	}
}

func TestChatSendToolInactiveProvider(t *testing.T) {
	lookup := &mockChatLookup{
		activeProviders: []string{"discord"},
	}
	tool := chatSendTool(lookup)

	args := map[string]interface{}{
		"provider": "telegram",
		"target":   "user:123",
		"message":  "Hello",
	}

	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error for inactive provider")
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("expected 'not active' in error, got: %v", err)
	}
}

func TestRegisterChatTools(t *testing.T) {
	s := NewServer(nil, nil)
	lookup := &mockChatLookup{
		activeProviders: []string{"discord", "telegram"},
	}

	RegisterChatTools(s, lookup)

	tools := s.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "chat_send" {
		t.Errorf("expected 'chat_send' tool, got '%s'", tools[0].Name)
	}
}
