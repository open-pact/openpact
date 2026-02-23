package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// OpenCode implements the Engine interface using `opencode serve` HTTP API.
type OpenCode struct {
	cfg          Config
	systemPrompt string
	baseURL      string       // e.g. "http://127.0.0.1:4098"
	client       *http.Client
	cmd          *exec.Cmd    // opencode serve child process
	mu           sync.Mutex
}

const defaultAIUser = "openpact-ai"

// NewOpenCode creates a new OpenCode engine
func NewOpenCode(cfg Config) (*OpenCode, error) {
	// Default RunAsUser to the standard AI user if it exists on the system
	if cfg.RunAsUser == "" {
		if _, err := user.Lookup(defaultAIUser); err == nil {
			cfg.RunAsUser = defaultAIUser
		}
	}

	return &OpenCode{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for AI responses
		},
	}, nil
}

// Start spawns `opencode serve` as a child process and waits for it to be ready.
func (o *OpenCode) Start(ctx context.Context) error {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode binary not found in PATH: %w", err)
	}
	log.Printf("Found opencode at: %s", path)

	// Pick port
	port := o.cfg.Port
	if port == 0 {
		port, err = findFreePort()
		if err != nil {
			return fmt.Errorf("failed to find free port: %w", err)
		}
	}

	o.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	// Build command
	args := []string{"serve", "--port", fmt.Sprintf("%d", port), "--hostname", "127.0.0.1"}
	cmd := exec.CommandContext(ctx, "opencode", args...)

	// Build filtered environment (security: only allowlisted vars)
	cmd.Env = buildFilteredEnv(o.cfg)

	// Set password if configured
	if o.cfg.Password != "" {
		cmd.Env = append(cmd.Env, "OPENCODE_SERVER_PASSWORD="+o.cfg.Password)
	}

	// Generate OpenCode config to disable built-in tools and configure MCP
	ocConfig := buildOpenCodeConfig(o.cfg)
	if len(ocConfig) > 0 {
		configJSON, err := json.Marshal(ocConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal opencode config: %w", err)
		}
		cmd.Env = append(cmd.Env, "OPENCODE_CONFIG_CONTENT="+string(configJSON))
	}

	// Run as restricted user if configured (Linux user separation)
	if o.cfg.RunAsUser != "" {
		if err := setSysProcCredential(cmd, o.cfg.RunAsUser); err != nil {
			log.Printf("Warning: failed to set run_as_user %q: %v (running as current user)", o.cfg.RunAsUser, err)
		}
	}

	if o.cfg.WorkDir != "" {
		cmd.Dir = o.cfg.WorkDir
	}

	// Pipe stdout/stderr to our logs
	cmd.Stdout = &logWriter{prefix: "opencode-serve"}
	cmd.Stderr = &logWriter{prefix: "opencode-serve"}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start opencode serve: %w", err)
	}

	o.mu.Lock()
	o.cmd = cmd
	o.mu.Unlock()

	log.Printf("opencode serve started (pid %d) on port %d", cmd.Process.Pid, port)

	// Wait for server to be ready
	if err := o.waitForReady(ctx); err != nil {
		// Kill the process if we can't connect
		_ = cmd.Process.Kill()
		return fmt.Errorf("opencode serve failed to become ready: %w", err)
	}

	log.Printf("opencode serve is ready at %s", o.baseURL)

	// Monitor process in background
	go func() {
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			log.Printf("opencode serve exited with error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the opencode serve process.
func (o *OpenCode) Stop() error {
	o.mu.Lock()
	cmd := o.cmd
	o.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	log.Println("Stopping opencode serve...")

	// Try graceful shutdown via signal
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		// If signal fails, force kill
		log.Printf("Failed to send interrupt, killing: %v", err)
		return cmd.Process.Kill()
	}

	// Wait briefly for graceful exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		log.Println("opencode serve stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Println("opencode serve did not stop in time, killing")
		_ = cmd.Process.Kill()
	}

	return nil
}

// Send posts a message to a session and streams the response.
func (o *OpenCode) Send(ctx context.Context, sessionID string, messages []Message) (<-chan Response, error) {
	o.mu.Lock()
	systemPrompt := o.systemPrompt
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
	if o.cfg.Provider != "" && o.cfg.Model != "" {
		body["model"] = map[string]string{
			"providerID": o.cfg.Provider,
			"modelID":    o.cfg.Model,
		}
	} else if o.cfg.Model != "" {
		body["model"] = map[string]string{
			"modelID": o.cfg.Model,
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

	// Parse the nested provider config structure
	var providers map[string]struct {
		Models map[string]struct {
			Limit struct {
				Context int `json:"context"`
				Output  int `json:"output"`
			} `json:"limit"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return 0, 0
	}

	// Search all providers for the model
	for _, provider := range providers {
		if m, ok := provider.Models[model]; ok {
			return m.Limit.Context, m.Limit.Output
		}
	}

	return 0, 0
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

// findFreePort asks the OS for an available port.
func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// buildFilteredEnv creates an allowlisted environment for the OpenCode process.
// Only system basics, XDG_ variables, and LLM provider keys are passed through.
// Sensitive tokens (DISCORD_TOKEN, GITHUB_TOKEN, etc.) are excluded.
func buildFilteredEnv(cfg Config) []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "USER": true,
		"LANG": true, "TERM": true, "TZ": true, "TMPDIR": true,
	}

	providerKeys := map[string]bool{
		"ANTHROPIC_API_KEY":    true,
		"OPENAI_API_KEY":      true,
		"GOOGLE_API_KEY":      true,
		"AZURE_OPENAI_API_KEY": true,
		"OLLAMA_HOST":         true,
	}

	var env []string
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if allowed[key] || providerKeys[key] || strings.HasPrefix(key, "XDG_") {
			env = append(env, e)
		}
	}

	// Override HOME and USER for the AI user
	if cfg.RunAsUser != "" {
		if u, err := user.Lookup(cfg.RunAsUser); err == nil {
			env = filterEnvKey(env, "HOME")
			env = append(env, "HOME="+u.HomeDir)
			env = filterEnvKey(env, "USER")
			env = append(env, "USER="+cfg.RunAsUser)
		}
	}

	return env
}

// filterEnvKey removes all entries with the given key from an env slice.
func filterEnvKey(env []string, key string) []string {
	prefix := key + "="
	result := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// findMCPBinary locates the mcp-server binary. It looks next to the current
// executable first (they're always built and deployed together), then falls
// back to PATH lookup. Returns an error if the binary cannot be found.
func findMCPBinary() (string, error) {
	// Look next to the current executable (e.g. /app/openpact → /app/mcp-server)
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

// buildOpenCodeConfig generates the OpenCode configuration that disables built-in
// tools and configures our MCP server. Passed via OPENCODE_CONFIG_CONTENT env var.
func buildOpenCodeConfig(cfg Config) map[string]interface{} {
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

	// Auto-discover the MCP server binary
	mcpBinary, err := findMCPBinary()
	if err != nil {
		log.Printf("WARNING: %v — AI will have no tools available", err)
		return config
	}

	mcpEnv := map[string]string{
		"OPENPACT_WORKSPACE_PATH": cfg.WorkDir,
	}
	for k, v := range cfg.MCPEnv {
		mcpEnv[k] = v
	}

	config["mcp"] = map[string]interface{}{
		"openpact": map[string]interface{}{
			"type":        "local",
			"command":     []string{mcpBinary},
			"environment": mcpEnv,
			"enabled":     true,
		},
	}

	return config
}

// setSysProcCredential configures the command to run as a different Linux user.
func setSysProcCredential(cmd *exec.Cmd, username string) error {
	aiUser, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("user %q not found: %w", username, err)
	}

	uid, err := strconv.Atoi(aiUser.Uid)
	if err != nil {
		return fmt.Errorf("invalid uid %q: %w", aiUser.Uid, err)
	}

	gid, err := strconv.Atoi(aiUser.Gid)
	if err != nil {
		return fmt.Errorf("invalid gid %q: %w", aiUser.Gid, err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	return nil
}

// logWriter writes lines to log with a prefix.
type logWriter struct {
	prefix string
	buf    []byte
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		if line != "" {
			log.Printf("[%s] %s", w.prefix, line)
		}
	}
	return len(p), nil
}
