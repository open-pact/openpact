package mcp

import (
	"context"
	"fmt"
	"strings"
)

// ChatProviderLookup resolves active chat providers at call time.
type ChatProviderLookup interface {
	GetActiveProviderNames() []string
	SendViaProvider(provider, target, content string) error
}

// RegisterChatTools adds the unified chat_send tool to the MCP server.
// The tool is always registered; providers are resolved dynamically at call time.
func RegisterChatTools(s *Server, lookup ChatProviderLookup) {
	s.RegisterTool(chatSendTool(lookup))
}

func chatSendTool(lookup ChatProviderLookup) *Tool {
	return &Tool{
		Name: "chat_send",
		Description: "Send a message via a chat provider. " +
			"Use 'user:<id>' for DMs or 'channel:<id>' for channels. " +
			"Target ID format depends on the provider.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider": map[string]interface{}{
					"type":        "string",
					"description": "Chat provider to send through (e.g., 'discord', 'telegram', 'slack')",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Target: 'user:<id>' for DM, 'channel:<id>' for channel, or just '<id>'",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Message content to send",
				},
			},
			"required": []string{"provider", "target", "message"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			provider, _ := args["provider"].(string)
			target, _ := args["target"].(string)
			message, _ := args["message"].(string)

			if provider == "" || target == "" || message == "" {
				return nil, fmt.Errorf("provider, target, and message are all required")
			}

			// Validate provider is active
			active := lookup.GetActiveProviderNames()
			found := false
			for _, name := range active {
				if name == provider {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("provider %q is not active (available: %s)", provider, strings.Join(active, ", "))
			}

			if err := lookup.SendViaProvider(provider, target, message); err != nil {
				return nil, fmt.Errorf("failed to send message: %w", err)
			}

			return fmt.Sprintf("Message sent via %s to %s", provider, target), nil
		},
	}
}
