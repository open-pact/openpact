package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestSSEParsing verifies that the SSE client correctly parses data: lines
// and dispatches events to the right session subscriber.
func TestSSEParsing(t *testing.T) {
	// Create an SSE server that sends a few events then closes
	events := []string{
		`{"type":"server.connected","properties":{}}`,
		`{"type":"message.part.updated","properties":{"part":{"id":"prt_1","sessionID":"ses_abc","type":"text","text":"Hello"},"delta":"Hello"}}`,
		`{"type":"message.part.updated","properties":{"part":{"id":"prt_2","sessionID":"ses_abc","type":"reasoning","text":"Let me think"}}}`,
		`{"type":"session.idle","sessionID":"ses_abc"}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/event" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)

		for _, evt := range events {
			fmt.Fprintf(w, "data: %s\n\n", evt)
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := newSSEClient(server.URL, "")
	sub := client.Subscribe("ses_abc")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.Start(ctx)

	// Collect events
	var received []sseEvent
	timeout := time.After(3 * time.Second)
	for {
		select {
		case evt, ok := <-sub.ch:
			if !ok {
				goto done
			}
			received = append(received, evt)
			// We expect 3 events for ses_abc (server.connected goes to global only)
			if len(received) >= 3 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}
done:

	client.Stop()

	if len(received) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(received))
	}

	// Verify event types
	if received[0].Type != "message.part.updated" {
		t.Errorf("event 0: expected message.part.updated, got %s", received[0].Type)
	}
	if received[1].Type != "message.part.updated" {
		t.Errorf("event 1: expected message.part.updated, got %s", received[1].Type)
	}
	if received[2].Type != "session.idle" {
		t.Errorf("event 2: expected session.idle, got %s", received[2].Type)
	}
}

// TestSSESessionFiltering verifies that events for one session are not
// dispatched to a subscriber for a different session.
func TestSSESessionFiltering(t *testing.T) {
	events := []string{
		`{"type":"message.part.updated","properties":{"part":{"id":"prt_1","sessionID":"ses_one","type":"text","text":"for one"}}}`,
		`{"type":"message.part.updated","properties":{"part":{"id":"prt_2","sessionID":"ses_two","type":"text","text":"for two"}}}`,
		`{"type":"session.idle","sessionID":"ses_one"}`,
		`{"type":"session.idle","sessionID":"ses_two"}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, evt := range events {
			fmt.Fprintf(w, "data: %s\n\n", evt)
			flusher.Flush()
		}
		// Keep connection open so client doesn't reconnect and replay events
		<-r.Context().Done()
	}))
	defer server.Close()

	client := newSSEClient(server.URL, "")
	subOne := client.Subscribe("ses_one")
	subTwo := client.Subscribe("ses_two")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.Start(ctx)

	// Collect from both — expect exactly 2 events per session
	var eventsOne, eventsTwo []sseEvent
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for len(eventsOne) < 2 {
			select {
			case evt, ok := <-subOne.ch:
				if !ok {
					return
				}
				eventsOne = append(eventsOne, evt)
			case <-time.After(3 * time.Second):
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for len(eventsTwo) < 2 {
			select {
			case evt, ok := <-subTwo.ch:
				if !ok {
					return
				}
				eventsTwo = append(eventsTwo, evt)
			case <-time.After(3 * time.Second):
				return
			}
		}
	}()

	wg.Wait()
	client.Stop()

	if len(eventsOne) != 2 {
		t.Errorf("ses_one: expected 2 events, got %d", len(eventsOne))
	}
	if len(eventsTwo) != 2 {
		t.Errorf("ses_two: expected 2 events, got %d", len(eventsTwo))
	}
}

// TestSSESubscribeUnsubscribe verifies that unsubscribing closes the channel
// and prevents further delivery.
func TestSSESubscribeUnsubscribe(t *testing.T) {
	client := newSSEClient("http://localhost:0", "")

	sub := client.Subscribe("ses_test")

	// Unsubscribe should close the channel
	client.Unsubscribe(sub)

	// Channel should be closed
	_, ok := <-sub.ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}

	// Verify subscriber was removed
	client.mu.RLock()
	subs := client.subscribers["ses_test"]
	client.mu.RUnlock()
	if len(subs) != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", len(subs))
	}
}

// TestSSEProcessEvent verifies the event parsing and dispatch logic directly.
func TestSSEProcessEvent(t *testing.T) {
	client := newSSEClient("http://localhost:0", "")
	sub := client.Subscribe("ses_abc")

	// Process a message.part.updated event
	data := `{"type":"message.part.updated","properties":{"part":{"id":"prt_1","sessionID":"ses_abc","type":"text","text":"Hello World"}}}`
	client.processEvent(data)

	select {
	case evt := <-sub.ch:
		if evt.Type != "message.part.updated" {
			t.Errorf("expected type message.part.updated, got %s", evt.Type)
		}
		// Verify the data is preserved
		var parsed struct {
			Properties struct {
				Part struct {
					Text string `json:"text"`
				} `json:"part"`
			} `json:"properties"`
		}
		json.Unmarshal(evt.Data, &parsed)
		if parsed.Properties.Part.Text != "Hello World" {
			t.Errorf("expected text 'Hello World', got %q", parsed.Properties.Part.Text)
		}
	default:
		t.Error("expected event to be dispatched")
	}

	client.Unsubscribe(sub)
}

// TestSSEAuth verifies that the SSE client sends basic auth when configured.
func TestSSEAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		// Send one event then close
		fmt.Fprintf(w, "data: {\"type\":\"server.connected\",\"properties\":{}}\n\n")
	}))
	defer server.Close()

	client := newSSEClient(server.URL, "test-password")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client.Start(ctx)
	time.Sleep(500 * time.Millisecond) // Let it connect
	client.Stop()

	if gotAuth == "" {
		t.Error("expected Authorization header to be set")
	}
}

// TestSSEReconnection verifies that the client reconnects after a connection drops.
func TestSSEReconnection(t *testing.T) {
	var mu sync.Mutex
	connectCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectCount++
		count := connectCount
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		if count == 1 {
			// First connection: send one event then close
			fmt.Fprintf(w, "data: {\"type\":\"server.connected\",\"properties\":{}}\n\n")
			flusher.Flush()
			return // Close connection to trigger reconnect
		}

		// Second connection: send event and keep open briefly
		fmt.Fprintf(w, "data: {\"type\":\"message.part.updated\",\"properties\":{\"part\":{\"id\":\"prt_1\",\"sessionID\":\"ses_recon\",\"type\":\"text\",\"text\":\"reconnected\"}}}\n\n")
		flusher.Flush()

		// Keep connection open for a bit
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := newSSEClient(server.URL, "")
	sub := client.Subscribe("ses_recon")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client.Start(ctx)

	// Wait for event from second connection
	select {
	case evt := <-sub.ch:
		if evt.Type != "message.part.updated" {
			t.Errorf("expected message.part.updated, got %s", evt.Type)
		}
	case <-time.After(8 * time.Second):
		t.Error("timed out waiting for event after reconnection")
	}

	client.Stop()

	mu.Lock()
	if connectCount < 2 {
		t.Errorf("expected at least 2 connections (reconnect), got %d", connectCount)
	}
	mu.Unlock()
}
