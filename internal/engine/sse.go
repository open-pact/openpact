package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// sseEvent represents a parsed SSE event from the OpenCode /event stream.
type sseEvent struct {
	Type string          `json:"type"` // e.g. "message.part.updated", "session.idle"
	Data json.RawMessage // The full JSON payload
}

// sseSubscription is a per-session subscription to SSE events.
type sseSubscription struct {
	sessionID string
	ch        chan sseEvent
}

// sseClient connects to OpenCode's GET /event SSE stream and fans out events
// to per-session subscribers. It reconnects automatically on disconnect.
type sseClient struct {
	baseURL  string
	password string
	client   *http.Client // No timeout — long-lived SSE connection

	mu          sync.RWMutex
	subscribers map[string][]*sseSubscription // sessionID → subs

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	connected   bool
	connectedMu sync.RWMutex
}

// newSSEClient creates a new SSE client (does not start it).
func newSSEClient(baseURL, password string) *sseClient {
	return &sseClient{
		baseURL:     baseURL,
		password:    password,
		client:      &http.Client{}, // No timeout for long-lived SSE
		subscribers: make(map[string][]*sseSubscription),
	}
}

// Start launches the SSE connection goroutine.
func (s *sseClient) Start(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(1)
	go s.run()
}

// Stop cancels the SSE connection and waits for the goroutine to exit.
func (s *sseClient) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// Subscribe returns a subscription for events matching the given sessionID.
// The returned channel receives events until Unsubscribe is called.
func (s *sseClient) Subscribe(sessionID string) *sseSubscription {
	sub := &sseSubscription{
		sessionID: sessionID,
		ch:        make(chan sseEvent, 64),
	}
	s.mu.Lock()
	s.subscribers[sessionID] = append(s.subscribers[sessionID], sub)
	s.mu.Unlock()
	return sub
}

// Unsubscribe removes a subscription and closes its channel.
func (s *sseClient) Unsubscribe(sub *sseSubscription) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs := s.subscribers[sub.sessionID]
	for i, existing := range subs {
		if existing == sub {
			s.subscribers[sub.sessionID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(s.subscribers[sub.sessionID]) == 0 {
		delete(s.subscribers, sub.sessionID)
	}
	close(sub.ch)
}

// IsConnected returns whether the SSE client has an active connection.
func (s *sseClient) IsConnected() bool {
	s.connectedMu.RLock()
	defer s.connectedMu.RUnlock()
	return s.connected
}

func (s *sseClient) setConnected(v bool) {
	s.connectedMu.Lock()
	s.connected = v
	s.connectedMu.Unlock()
}

// run is the main loop: connect, read events, reconnect on failure.
func (s *sseClient) run() {
	defer s.wg.Done()

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-s.ctx.Done():
			s.setConnected(false)
			return
		default:
		}

		err := s.connect()
		s.setConnected(false)

		if s.ctx.Err() != nil {
			return // Context cancelled — clean shutdown
		}

		if err != nil {
			log.Printf("[sse] connection lost: %v — reconnecting in %s", err, backoff)
		}

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Exponential backoff, capped
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// connect opens a single SSE connection and reads events until it closes or errors.
func (s *sseClient) connect() error {
	url := s.baseURL + "/event"
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	if s.password != "" {
		req.SetBasicAuth("opencode", s.password)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &sseConnError{StatusCode: resp.StatusCode}
	}

	// Reset backoff on successful connect
	s.setConnected(true)
	log.Printf("[sse] connected to %s", url)

	scanner := bufio.NewScanner(resp.Body)
	// Allow up to 1MB per line (SSE events can be large with tool output)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			if data != "" {
				dataLines = append(dataLines, data)
			}
		} else if line == "" && len(dataLines) > 0 {
			// Empty line = event delimiter. Process accumulated data lines.
			fullData := strings.Join(dataLines, "\n")
			dataLines = dataLines[:0]
			s.processEvent(fullData)
		}
		// Ignore lines starting with ":" (SSE comments) or other prefixes
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil // EOF — server closed the connection
}

// processEvent parses a JSON SSE payload and dispatches it to subscribers.
func (s *sseClient) processEvent(data string) {
	// Parse just the type and session ID
	var envelope struct {
		Type       string `json:"type"`
		Properties struct {
			Part struct {
				SessionID string `json:"sessionID"`
			} `json:"part"`
			SessionID string `json:"sessionID"` // some events have sessionID at properties level
		} `json:"properties"`
		// session.idle has sessionID at properties.sessionID
		SessionID string `json:"sessionID"` // some events have it at top level
		Status    *struct {
			Type string `json:"type"`
		} `json:"status"` // for session.status events
	}

	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return // Skip unparseable events
	}

	// Determine the session ID from whichever field has it
	sessionID := envelope.Properties.Part.SessionID
	if sessionID == "" {
		sessionID = envelope.Properties.SessionID
	}
	if sessionID == "" {
		sessionID = envelope.SessionID
	}

	evt := sseEvent{
		Type: envelope.Type,
		Data: json.RawMessage(data),
	}

	// Dispatch to subscribers for this session
	if sessionID != "" {
		s.dispatch(sessionID, evt)
	}

	// Also dispatch to anyone subscribed with empty sessionID (global listeners)
	s.dispatch("", evt)
}

// dispatch sends an event to all subscribers for a given sessionID.
func (s *sseClient) dispatch(sessionID string, evt sseEvent) {
	s.mu.RLock()
	subs := s.subscribers[sessionID]
	s.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- evt:
		default:
			// Channel full — drop event to avoid blocking
			log.Printf("[sse] dropping event for session %s (channel full)", sessionID)
		}
	}
}

// sseConnError represents an SSE connection error with an HTTP status code.
type sseConnError struct {
	StatusCode int
}

func (e *sseConnError) Error() string {
	return "SSE connection failed with status " + http.StatusText(e.StatusCode)
}
