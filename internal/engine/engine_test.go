package engine

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "default to opencode",
			cfg:  Config{},
		},
		{
			name: "explicit opencode",
			cfg:  Config{Type: "opencode"},
		},
		{
			name: "unknown defaults to opencode",
			cfg:  Config{Type: "unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng, err := New(tt.cfg)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			if _, ok := eng.(*OpenCode); !ok {
				t.Errorf("New() returned %T, want *engine.OpenCode", eng)
			}
		})
	}
}

func TestOpenCodeSystemPrompt(t *testing.T) {
	eng, _ := NewOpenCode(Config{})

	prompt := "You are a helpful assistant."
	eng.SetSystemPrompt(prompt)

	if eng.systemPrompt != prompt {
		t.Errorf("SetSystemPrompt() = %v, want %v", eng.systemPrompt, prompt)
	}
}

func TestResponseStruct(t *testing.T) {
	resp := Response{
		Content: "Hello!",
		ToolCalls: []ToolCall{
			{
				ID:   "call_123",
				Name: "read_file",
				Arguments: map[string]string{
					"path": "/test.txt",
				},
			},
		},
		Done: true,
	}

	if resp.Content != "Hello!" {
		t.Errorf("Response.Content = %v, want 'Hello!'", resp.Content)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("Response.ToolCalls length = %d, want 1", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("ToolCall.Name = %v, want 'read_file'", resp.ToolCalls[0].Name)
	}

	if !resp.Done {
		t.Error("Response.Done = false, want true")
	}
}

func TestMessageStruct(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "What's the weather?",
	}

	if msg.Role != "user" {
		t.Errorf("Message.Role = %v, want 'user'", msg.Role)
	}

	if msg.Content != "What's the weather?" {
		t.Errorf("Message.Content = %v, want 'What's the weather?'", msg.Content)
	}
}
