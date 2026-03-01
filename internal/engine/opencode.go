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
	sse          *sseClient   // Persistent SSE connection for real-time streaming
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

	hostname := o.cfg.Hostname
	if hostname == "" {
		hostname = "127.0.0.1"
	}

	o.baseURL = fmt.Sprintf("http://%s:%d", hostname, port)

	log.Printf("Connecting to opencode serve at %s", o.baseURL)

	// Wait for server to be ready
	if err := o.waitForReady(ctx); err != nil {
		return fmt.Errorf("opencode serve failed to become ready: %w", err)
	}

	log.Printf("opencode serve is ready at %s", o.baseURL)

	// Start persistent SSE connection for real-time streaming
	o.sse = newSSEClient(o.baseURL, o.cfg.Password)
	o.sse.Start(ctx)

	return nil
}

// Stop shuts down the SSE client. The OpenCode process itself is managed externally.
func (o *OpenCode) Stop() error {
	if o.sse != nil {
		o.sse.Stop()
	}
	return nil
}

// Send posts a message to a session and streams the response.
// If the SSE client is connected, events are streamed in real-time as parts
// are created/updated. After completion, a GET reconciliation ensures no parts
// were missed. Falls back to the blocking POST+GET path if SSE is unavailable.
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

	// If SSE is not connected, fall back to blocking POST+GET
	if o.sse == nil || !o.sse.IsConnected() {
		return o.sendBlocking(ctx, sessionID, jsonBody)
	}

	return o.sendStreaming(ctx, sessionID, jsonBody)
}

// sendStreaming uses the SSE event stream for real-time part delivery.
func (o *OpenCode) sendStreaming(ctx context.Context, sessionID string, jsonBody []byte) (<-chan Response, error) {
	// Subscribe to SSE events BEFORE sending the POST (so we don't miss early events)
	sub := o.sse.Subscribe(sessionID)

	responseChan := make(chan Response, 32)

	go func() {
		defer close(responseChan)
		defer o.sse.Unsubscribe(sub)

		// Fire POST in background
		type postResult struct {
			anchorID string
			err      error
		}
		postDone := make(chan postResult, 1)
		go func() {
			id, err := o.postMessage(ctx, sessionID, jsonBody)
			postDone <- postResult{id, err}
		}()

		seenParts := make(map[string]bool)
		var userMsgID string // Learned from first message.part.updated (user msg arrives first since we subscribe before POST)
		var anchorID string

		// Process SSE events until session.idle or context cancellation
	eventLoop:
		for {
			select {
			case <-ctx.Done():
				return

			case result := <-postDone:
				if result.err != nil {
					log.Printf("[sse] POST failed for session %s: %v", sessionID, result.err)
					responseChan <- Response{
						Done:      true,
						SessionID: sessionID,
					}
					return
				}
				anchorID = result.anchorID
				// Continue processing SSE events — POST completing doesn't mean AI is done.

			case evt, ok := <-sub.ch:
				if !ok {
					break eventLoop // Channel closed (unsubscribed)
				}

				switch evt.Type {
				case "message.part.updated":
					o.handlePartEvent(evt.Data, sessionID, seenParts, &userMsgID, responseChan)

				case "session.idle":
					// Definitive completion signal
					break eventLoop

				case "session.status", "message.updated":
					// Informational — no action needed
				}
			}
		}

		// Reconciliation: GET resolved messages to catch anything missed by SSE
		o.reconcile(sessionID, anchorID, seenParts, responseChan)

		responseChan <- Response{
			Done:      true,
			SessionID: sessionID,
		}
	}()

	return responseChan, nil
}

// handlePartEvent processes a message.part.updated SSE event and sends it to the response channel.
// Filters out user message parts by tracking the user's messageID. Since we subscribe before POST,
// the first messageID seen belongs to the user message — all subsequent messageIDs are assistant.
func (o *OpenCode) handlePartEvent(data json.RawMessage, sessionID string, seenParts map[string]bool, userMsgID *string, ch chan<- Response) {
	var evt struct {
		Properties struct {
			Part struct {
				ID        string `json:"id"`
				MessageID string `json:"messageID"`
				Type      string `json:"type"`
				Text      string `json:"text"`
				SessionID string `json:"sessionID"`
				Time      struct {
					Start int64  `json:"start"`
					End   *int64 `json:"end"`
				} `json:"time"`
			} `json:"part"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &evt); err != nil {
		return
	}

	// Second parse: capture the raw part JSON so tool/file/snapshot parts
	// retain ALL original fields (e.g. "tool", "state") that the typed
	// struct above doesn't declare.
	var rawEvt struct {
		Properties struct {
			Part json.RawMessage `json:"part"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &rawEvt); err != nil {
		return
	}

	part := evt.Properties.Part
	if part.ID == "" {
		return
	}

	// Learn the user message ID from the first part event (user message is always created first
	// because we subscribe before POST). Then skip all parts from that message.
	if *userMsgID == "" && part.MessageID != "" {
		*userMsgID = part.MessageID
	}
	if part.MessageID != "" && part.MessageID == *userMsgID {
		return
	}

	isUpdate := seenParts[part.ID]
	seenParts[part.ID] = true

	switch part.Type {
	case "reasoning", "thinking":
		ch <- Response{
			Thinking:  part.Text,
			SessionID: sessionID,
			PartID:    part.ID,
			PartType:  part.Type,
			IsUpdate:  isUpdate,
		}

	case "text":
		ch <- Response{
			Content:   part.Text,
			SessionID: sessionID,
			PartID:    part.ID,
			PartType:  part.Type,
			IsUpdate:  isUpdate,
		}

	case "step-start", "step-finish":
		// Skip operational markers

	default:
		// tool, file, snapshot, etc. — forward the raw JSON (preserves all fields)
		ch <- Response{
			Parts:     []json.RawMessage{rawEvt.Properties.Part},
			SessionID: sessionID,
			PartID:    part.ID,
			PartType:  part.Type,
			IsUpdate:  isUpdate,
		}
	}
}

// postMessage sends the POST /session/:id/message and returns the anchor message ID.
func (o *OpenCode) postMessage(ctx context.Context, sessionID string, jsonBody []byte) (string, error) {
	url := fmt.Sprintf("%s/session/%s/message", o.baseURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	o.setAuth(req)

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("opencode API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var postMsg struct {
		Info struct {
			ID string `json:"id"`
		} `json:"info"`
	}
	json.Unmarshal(respBody, &postMsg)

	return postMsg.Info.ID, nil
}

// reconcile fetches resolved messages via GET and forwards any parts not already
// sent via SSE. This catches tool output, file parts, and anything else that
// may have been missed during streaming.
func (o *OpenCode) reconcile(sessionID, anchorID string, seenParts map[string]bool, ch chan<- Response) {
	if anchorID == "" {
		return
	}

	var turnMessages []MessageInfo
	for _, limit := range []int{10, 50, 200} {
		messages, err := o.GetMessages(sessionID, limit)
		if err != nil {
			log.Printf("[reconcile] Error fetching messages (limit %d): %v", limit, err)
			break
		}

		turnMessages = o.extractTurn(messages, anchorID)
		if turnMessages != nil {
			break
		}
	}

	for _, msg := range turnMessages {
		if msg.Role != "assistant" {
			continue
		}
		o.forwardUnseenParts(msg.Parts, sessionID, seenParts, ch)
	}
}

// forwardUnseenParts sends parts that weren't already delivered via SSE, and
// re-sends tool/file/snapshot parts as updates (since SSE may have delivered
// them with incomplete fields like missing "tool" name).
func (o *OpenCode) forwardUnseenParts(parts []json.RawMessage, sessionID string, seenParts map[string]bool, ch chan<- Response) {
	for _, raw := range parts {
		var peek struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil {
			continue
		}

		// Skip operational markers
		if peek.Type == "step-start" || peek.Type == "step-finish" {
			continue
		}

		alreadySeen := peek.ID != "" && seenParts[peek.ID]

		// Text/thinking are fully represented in SSE — skip if already seen
		if alreadySeen && (peek.Type == "text" || peek.Type == "reasoning" || peek.Type == "thinking") {
			continue
		}

		// Tool/file/snapshot — re-send with IsUpdate so frontend replaces incomplete SSE data
		if alreadySeen {
			ch <- Response{Parts: []json.RawMessage{raw}, SessionID: sessionID, PartID: peek.ID, PartType: peek.Type, IsUpdate: true}
			continue
		}

		switch peek.Type {
		case "reasoning", "thinking":
			ch <- Response{Thinking: peek.Text, SessionID: sessionID, PartID: peek.ID, PartType: peek.Type}
		case "text":
			ch <- Response{Content: peek.Text, SessionID: sessionID, PartID: peek.ID, PartType: peek.Type}
		default:
			ch <- Response{Parts: []json.RawMessage{raw}, SessionID: sessionID, PartID: peek.ID, PartType: peek.Type}
		}
	}
}

// sendBlocking is the original POST+GET fallback when SSE is unavailable.
func (o *OpenCode) sendBlocking(ctx context.Context, sessionID string, jsonBody []byte) (<-chan Response, error) {
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

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response: %v", err)
			return
		}

		var postMsg struct {
			Info struct {
				ID string `json:"id"`
			} `json:"info"`
		}
		json.Unmarshal(respBody, &postMsg)

		var turnMessages []MessageInfo
		for _, limit := range []int{10, 50, 200} {
			messages, err := o.GetMessages(sessionID, limit)
			if err != nil {
				log.Printf("Error fetching messages (limit %d): %v", limit, err)
				break
			}

			turnMessages = o.extractTurn(messages, postMsg.Info.ID)
			if turnMessages != nil {
				break
			}
		}

		for _, msg := range turnMessages {
			if msg.Role != "assistant" {
				continue
			}
			o.forwardMessageParts(msg.Parts, sessionID, responseChan)
		}

		responseChan <- Response{
			Done:      true,
			SessionID: sessionID,
		}
	}()

	return responseChan, nil
}

// extractTurn finds the POST response message by ID in the message list, then
// scans backwards to the preceding user message. Returns the slice of messages
// that make up this turn (between the user message and the POST message,
// inclusive of the POST message but exclusive of the user message).
// Returns nil if anchorID is not found in the list.
func (o *OpenCode) extractTurn(messages []MessageInfo, anchorID string) []MessageInfo {
	// Find the anchor message (the POST response)
	anchorIdx := -1
	for i, m := range messages {
		if m.ID == anchorID {
			anchorIdx = i
			break
		}
	}
	if anchorIdx < 0 {
		return nil
	}

	// Scan backwards from the anchor to find the user message that started
	// this turn — everything between it and the anchor is the turn.
	startIdx := 0
	for i := anchorIdx - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			startIdx = i + 1
			break
		}
	}

	return messages[startIdx : anchorIdx+1]
}

// forwardMessageParts extracts text, thinking, and other parts from raw message
// parts and sends them through the response channel in display order.
func (o *OpenCode) forwardMessageParts(parts []json.RawMessage, sessionID string, ch chan<- Response) {
	var text string
	var thinking string
	var extraParts []json.RawMessage

	for _, raw := range parts {
		var peek struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil {
			continue
		}
		switch peek.Type {
		case "text":
			text += peek.Text
		case "reasoning", "thinking":
			thinking += peek.Text
		case "step-start", "step-finish":
			// Operational markers from OpenCode — skip, the resolved tool/file
			// parts contain the real data.
			continue
		default:
			// tool, file, snapshot, etc.
			extraParts = append(extraParts, raw)
		}
	}

	if thinking != "" {
		ch <- Response{Thinking: thinking, SessionID: sessionID}
	}
	if len(extraParts) > 0 {
		ch <- Response{Parts: extraParts, SessionID: sessionID}
	}
	if text != "" {
		ch <- Response{Content: text, SessionID: sessionID}
	}
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
		Info  MessageInfo       `json:"info"`
		Parts []json.RawMessage `json:"parts"`
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
		// Disable ALL built-in tools — OpenPact provides capabilities via MCP
		"tools": map[string]bool{
			"bash": false, "write": false, "edit": false, "read": false,
			"grep": false, "glob": false, "list": false, "patch": false,
			"webfetch": false, "websearch": false,
			"question": false, "task": false, "todowrite": false,
		},
		// Auto-allow our MCP tools
		"permission": map[string]string{
			"openpact_*": "allow",
		},
		// Override the default "build" agent prompt. OpenCode's hardcoded default
		// tells the AI it's a CLI coding agent with shell/file access, which
		// directly contradicts OpenPact's security model. We replace it with a
		// minimal prompt — the real context (identity, tools, memory) is injected
		// by OpenPact via the "system" parameter on each message.
		"agent": map[string]interface{}{
			"build": map[string]interface{}{
				"prompt": "You are an AI assistant. Your capabilities, identity, and instructions are provided in the system message with each conversation. Follow those instructions.",
			},
			"plan": map[string]interface{}{
				"prompt": "You are an AI assistant in planning mode. Your capabilities, identity, and instructions are provided in the system message with each conversation. Follow those instructions. In this mode, focus on analysis and planning rather than taking actions.",
			},
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
