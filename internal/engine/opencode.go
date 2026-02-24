package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// OpenCode implements the Engine interface using `opencode serve` HTTP API.
// It connects to an externally-managed OpenCode process (launched by the
// container entrypoint) — it does NOT spawn or manage the process itself.
type OpenCode struct {
	cfg          Config
	systemPrompt string
	baseURL      string       // e.g. "http://127.0.0.1:4098"
	client       *http.Client
	mu           sync.Mutex
}

// DefaultPort is the fixed port used by both the entrypoint (which launches
// OpenCode) and the engine (which connects to it). Both sides must agree.
const DefaultPort = 4098

// NewOpenCode creates a new OpenCode engine
func NewOpenCode(cfg Config) (*OpenCode, error) {
	return &OpenCode{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for AI responses
		},
	}, nil
}

// Start connects to an already-running `opencode serve` instance and waits
// for it to be ready. The process is managed externally (e.g. by the Docker
// entrypoint), so Start does not spawn anything.
func (o *OpenCode) Start(ctx context.Context) error {
	port := o.cfg.Port
	if port == 0 {
		port = DefaultPort
	}

	o.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	log.Printf("Connecting to opencode serve at %s", o.baseURL)

	// Wait for server to be ready
	if err := o.waitForReady(ctx); err != nil {
		return fmt.Errorf("opencode serve failed to become ready: %w", err)
	}

	log.Printf("opencode serve is ready at %s", o.baseURL)
	return nil
}

// Stop is a no-op — the OpenCode process is managed externally.
func (o *OpenCode) Stop() error {
	return nil
}

// Send posts a message to a session and streams the response.
func (o *OpenCode) Send(ctx context.Context, sessionID string, messages []Message) (<-chan Response, error) {
	o.mu.Lock()
	systemPrompt := o.systemPrompt
	provider := o.cfg.Provider
	model := o.cfg.Model
	o.mu.Unlock()

	// Extract the last user message
	var userMsg string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userMsg = messages[i].Content
			break
		}
	}
	if userMsg == "" {
		return nil, fmt.Errorf("no user message found")
	}

	// Build request body
	body := map[string]interface{}{
		"parts": []map[string]string{
			{"type": "text", "text": userMsg},
		},
	}

	if systemPrompt != "" {
		body["system"] = systemPrompt
	}

	// Add model if configured (API expects an object with providerID + modelID)
	if provider != "" && model != "" {
		body["model"] = map[string]string{
			"providerID": provider,
			"modelID":    model,
		}
	} else if model != "" {
		body["model"] = map[string]string{
			"modelID": model,
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/session/%s/message", o.baseURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("opencode API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	responseChan := make(chan Response, 10)

	go func() {
		defer close(responseChan)
		defer resp.Body.Close()

		// Read the full response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response: %v", err)
			return
		}

		// Try to parse as a message object
		var msgResp struct {
			ID    string        `json:"id"`
			Parts []MessagePart `json:"parts"`
		}
		if err := json.Unmarshal(respBody, &msgResp); err == nil && len(msgResp.Parts) > 0 {
			var text string
			var thinking string
			for _, part := range msgResp.Parts {
				if part.Type == "text" && part.Text != "" {
					text += part.Text
				} else if (part.Type == "reasoning" || part.Type == "thinking") && part.Text != "" {
					thinking += part.Text
				}
			}
			if thinking != "" {
				responseChan <- Response{
					Thinking:  thinking,
					SessionID: sessionID,
				}
			}
			if text != "" {
				responseChan <- Response{
					Content:   text,
					SessionID: sessionID,
				}
			}
		} else {
			// Fall back to treating as plain text
			content := string(respBody)
			if content != "" {
				responseChan <- Response{
					Content:   content,
					SessionID: sessionID,
				}
			}
		}

		responseChan <- Response{
			Done:      true,
			SessionID: sessionID,
		}
	}()

	return responseChan, nil
}

// SetSystemPrompt sets the system prompt for context injection.
func (o *OpenCode) SetSystemPrompt(prompt string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.systemPrompt = prompt
}

// CreateSession creates a new opencode session.
func (o *OpenCode) CreateSession() (*Session, error) {
	url := fmt.Sprintf("%s/session", o.baseURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create session failed (status %d): %s", resp.StatusCode, string(body))
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return &session, nil
}

// ListSessions returns all opencode sessions.
func (o *OpenCode) ListSessions() ([]Session, error) {
	url := fmt.Sprintf("%s/session", o.baseURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list sessions failed (status %d): %s", resp.StatusCode, string(body))
	}

	var sessions []Session
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("failed to decode sessions: %w", err)
	}

	return sessions, nil
}

// GetSession returns a specific session by ID.
func (o *OpenCode) GetSession(id string) (*Session, error) {
	url := fmt.Sprintf("%s/session/%s", o.baseURL, id)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get session failed (status %d): %s", resp.StatusCode, string(body))
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return &session, nil
}

// DeleteSession removes a session by ID.
func (o *OpenCode) DeleteSession(id string) error {
	url := fmt.Sprintf("%s/session/%s", o.baseURL, id)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete session failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// AbortSession aborts a running session.
func (o *OpenCode) AbortSession(id string) error {
	url := fmt.Sprintf("%s/session/%s/abort", o.baseURL, id)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to abort session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("abort session failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetMessages returns messages for a session.
func (o *OpenCode) GetMessages(sessionID string, limit int) ([]MessageInfo, error) {
	url := fmt.Sprintf("%s/session/%s/message", o.baseURL, sessionID)
	if limit > 0 {
		url += fmt.Sprintf("?limit=%d", limit)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get messages failed (status %d): %s", resp.StatusCode, string(body))
	}

	// OpenCode wraps each message in {"info": {...}, "parts": [...]}
	var wrapped []struct {
		Info  MessageInfo   `json:"info"`
		Parts []MessagePart `json:"parts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	messages := make([]MessageInfo, len(wrapped))
	for i, w := range wrapped {
		messages[i] = w.Info
		messages[i].Parts = w.Parts
	}

	return messages, nil
}

// GetContextUsage fetches token usage data for a session from the OpenCode API.
func (o *OpenCode) GetContextUsage(sessionID string) (*ContextUsage, error) {
	// Fetch all messages for the session (raw JSON to access token fields)
	url := fmt.Sprintf("%s/session/%s/message", o.baseURL, sessionID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get messages failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse messages — OpenCode wraps each message in {"info": {...}, "parts": [...]}
	var messages []struct {
		Info struct {
			Role       string `json:"role"`
			ModelID    string `json:"modelID"`
			ProviderID string `json:"providerID"`
			Tokens     struct {
				Input  int `json:"input"`
				Output int `json:"output"`
				Reasoning int `json:"reasoning"`
				Cache  struct {
					Read  int `json:"read"`
					Write int `json:"write"`
				} `json:"cache"`
			} `json:"tokens"`
			Cost float64 `json:"cost"`
		} `json:"info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	usage := &ContextUsage{}

	for _, msg := range messages {
		if msg.Info.Role != "assistant" {
			continue
		}
		usage.MessageCount++
		usage.CurrentContext = msg.Info.Tokens.Input // overwrite each time; last one is current
		usage.TotalOutput += msg.Info.Tokens.Output
		usage.TotalReasoning += msg.Info.Tokens.Reasoning
		usage.CacheRead += msg.Info.Tokens.Cache.Read
		usage.CacheWrite += msg.Info.Tokens.Cache.Write
		usage.TotalCost += msg.Info.Cost

		if msg.Info.ModelID != "" {
			usage.Model = msg.Info.ModelID
		}
	}

	// Fetch model limits (best-effort)
	contextLimit, outputLimit := o.getModelLimits(usage.Model)
	usage.ContextLimit = contextLimit
	usage.OutputLimit = outputLimit

	return usage, nil
}

// getModelLimits fetches the context and output limits for a model from the OpenCode config API.
// Returns (0, 0) on any error — limits are optional display info.
func (o *OpenCode) getModelLimits(model string) (contextLimit, outputLimit int) {
	if model == "" {
		return 0, 0
	}

	url := fmt.Sprintf("%s/config/providers", o.baseURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, 0
	}
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0
	}

	// Parse the config/providers response: { providers: Provider[], default: {...} }
	var configResp struct {
		Providers []struct {
			ID     string `json:"id"`
			Models map[string]struct {
				Limit struct {
					Context int `json:"context"`
					Output  int `json:"output"`
				} `json:"limit"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return 0, 0
	}

	// Search all providers for the model
	for _, provider := range configResp.Providers {
		if m, ok := provider.Models[model]; ok {
			return m.Limit.Context, m.Limit.Output
		}
	}

	return 0, 0
}

// ListModels fetches all available models from the OpenCode config API.
func (o *OpenCode) ListModels() ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/config/providers", o.baseURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch providers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("providers API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the config/providers response: { providers: Provider[], default: {...} }
	var configResp struct {
		Providers []struct {
			ID     string `json:"id"`
			Models map[string]struct {
				Limit struct {
					Context int `json:"context"`
					Output  int `json:"output"`
				} `json:"limit"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return nil, fmt.Errorf("failed to decode providers: %w", err)
	}

	var models []ModelInfo
	for _, provider := range configResp.Providers {
		for modelID, m := range provider.Models {
			models = append(models, ModelInfo{
				ProviderID: provider.ID,
				ModelID:    modelID,
				Context:    m.Limit.Context,
				Output:     m.Limit.Output,
			})
		}
	}

	return models, nil
}

// GetDefaultModel returns the currently configured default provider and model.
func (o *OpenCode) GetDefaultModel() (provider, model string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.cfg.Provider, o.cfg.Model
}

// SetDefaultModel updates the default provider and model for new sessions.
func (o *OpenCode) SetDefaultModel(provider, model string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.cfg.Provider = provider
	o.cfg.Model = model
}

// waitForReady polls the server until it responds or context is cancelled.
func (o *OpenCode) waitForReady(ctx context.Context) error {
	healthURL := fmt.Sprintf("%s/global/health", o.baseURL)

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			continue
		}
		o.setAuth(req)

		resp, err := o.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}
	}

	return fmt.Errorf("opencode serve did not become ready within 15 seconds")
}

// setAuth adds authentication to a request if a password is configured.
func (o *OpenCode) setAuth(req *http.Request) {
	if o.cfg.Password != "" {
		req.SetBasicAuth("opencode", o.cfg.Password)
	}
}

// FindMCPBinary locates the mcp-server binary. It looks next to the current
// executable first (they're always built and deployed together), then falls
// back to PATH lookup. Returns an error if the binary cannot be found.
func FindMCPBinary() (string, error) {
	// Look next to the current executable (e.g. /app/openpact -> /app/mcp-server)
	exe, err := os.Executable()
	if err == nil {
		sibling := filepath.Join(filepath.Dir(exe), "mcp-server")
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}

	// Fall back to PATH
	if p, err := exec.LookPath("mcp-server"); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("mcp-server binary not found (looked next to %s and in PATH)", exe)
}

// MCPPort is the fixed port for the in-process MCP HTTP server.
// Must match the port the orchestrator binds on (mcp.MCPPort).
const MCPPort = 3100

// BuildOpenCodeConfig generates the OpenCode configuration that disables built-in
// tools and configures our remote MCP server. Used by the opencode-config subcommand
// to produce JSON passed via OPENCODE_CONFIG_CONTENT env var.
//
// mcpToken is the bearer token for authenticating with the MCP HTTP server.
func BuildOpenCodeConfig(cfg Config, mcpToken string) map[string]interface{} {
	config := map[string]interface{}{
		// Disable ALL built-in filesystem/shell tools
		"tools": map[string]bool{
			"bash": false, "write": false, "edit": false, "read": false,
			"grep": false, "glob": false, "list": false, "patch": false,
			"webfetch": false, "websearch": false,
		},
		// Auto-allow our MCP tools
		"permission": map[string]string{
			"openpact_*": "allow",
		},
	}

	if mcpToken == "" {
		log.Printf("WARNING: no MCP token provided — AI will have no tools available")
		return config
	}

	config["mcp"] = map[string]interface{}{
		"openpact": map[string]interface{}{
			"type":    "remote",
			"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp", MCPPort),
			"headers": map[string]string{"Authorization": "Bearer " + mcpToken},
			"enabled": true,
		},
	}

	return config
}
