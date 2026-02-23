// Package health provides health checking and metrics endpoints.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Status represents the health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check is a function that returns a health check result
type Check func(ctx context.Context) CheckResult

// CheckResult is the result of a health check
type CheckResult struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthResponse is the JSON response for the health endpoint
type HealthResponse struct {
	Status    Status                  `json:"status"`
	Timestamp string                  `json:"timestamp"`
	Uptime    string                  `json:"uptime"`
	Checks    map[string]CheckResult  `json:"checks,omitempty"`
}

// Metrics holds runtime metrics
type Metrics struct {
	RequestsTotal    uint64 `json:"requests_total"`
	RequestsSuccess  uint64 `json:"requests_success"`
	RequestsError    uint64 `json:"requests_error"`
	MessagesReceived uint64 `json:"messages_received"`
	MessagesSent     uint64 `json:"messages_sent"`
	ToolCallsTotal   uint64 `json:"tool_calls_total"`
	ToolCallsSuccess uint64 `json:"tool_calls_success"`
	ToolCallsError   uint64 `json:"tool_calls_error"`
}

// MetricsResponse is the JSON response for the metrics endpoint
type MetricsResponse struct {
	Timestamp string  `json:"timestamp"`
	Uptime    string  `json:"uptime"`
	Metrics   Metrics `json:"metrics"`
}

// Server provides health check and metrics endpoints
type Server struct {
	mu        sync.RWMutex
	checks    map[string]Check
	startTime time.Time
	addr      string
	server    *http.Server

	// Metrics counters (atomic)
	requestsTotal    uint64
	requestsSuccess  uint64
	requestsError    uint64
	messagesReceived uint64
	messagesSent     uint64
	toolCallsTotal   uint64
	toolCallsSuccess uint64
	toolCallsError   uint64
}

// NewServer creates a new health/metrics server
func NewServer(addr string) *Server {
	s := &Server{
		checks:    make(map[string]Check),
		startTime: time.Now(),
		addr:      addr,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/healthz", s.handleHealth) // k8s style
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/readyz", s.handleReady) // k8s style
	mux.HandleFunc("/metrics", s.handleMetrics)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	return s
}

// RegisterCheck registers a health check
func (s *Server) RegisterCheck(name string, check Check) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks[name] = check
}

// Start starts the health server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleHealth returns the overall health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	s.mu.RLock()
	checks := make(map[string]Check, len(s.checks))
	for k, v := range s.checks {
		checks[k] = v
	}
	s.mu.RUnlock()

	// Run all health checks
	results := make(map[string]CheckResult)
	overallStatus := StatusHealthy

	for name, check := range checks {
		result := check(ctx)
		results[name] = result

		// Determine worst status
		if result.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if result.Status == StatusDegraded && overallStatus != StatusUnhealthy {
			overallStatus = StatusDegraded
		}
	}

	resp := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    time.Since(s.startTime).Round(time.Second).String(),
		Checks:    results,
	}

	w.Header().Set("Content-Type", "application/json")
	if overallStatus == StatusUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(resp)
}

// handleReady returns readiness (simpler than health)
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}

// handleMetrics returns Prometheus-style metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Accept header check for JSON vs Prometheus format
	accept := r.Header.Get("Accept")

	metrics := Metrics{
		RequestsTotal:    atomic.LoadUint64(&s.requestsTotal),
		RequestsSuccess:  atomic.LoadUint64(&s.requestsSuccess),
		RequestsError:    atomic.LoadUint64(&s.requestsError),
		MessagesReceived: atomic.LoadUint64(&s.messagesReceived),
		MessagesSent:     atomic.LoadUint64(&s.messagesSent),
		ToolCallsTotal:   atomic.LoadUint64(&s.toolCallsTotal),
		ToolCallsSuccess: atomic.LoadUint64(&s.toolCallsSuccess),
		ToolCallsError:   atomic.LoadUint64(&s.toolCallsError),
	}

	if accept == "application/json" || r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MetricsResponse{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Uptime:    time.Since(s.startTime).Round(time.Second).String(),
			Metrics:   metrics,
		})
		return
	}

	// Prometheus format
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	uptime := time.Since(s.startTime).Seconds()
	fmt.Fprintf(w, "# HELP openpact_uptime_seconds Time since server start\n")
	fmt.Fprintf(w, "# TYPE openpact_uptime_seconds gauge\n")
	fmt.Fprintf(w, "openpact_uptime_seconds %.2f\n\n", uptime)

	fmt.Fprintf(w, "# HELP openpact_requests_total Total number of requests\n")
	fmt.Fprintf(w, "# TYPE openpact_requests_total counter\n")
	fmt.Fprintf(w, "openpact_requests_total %d\n\n", metrics.RequestsTotal)

	fmt.Fprintf(w, "# HELP openpact_requests_success Successful requests\n")
	fmt.Fprintf(w, "# TYPE openpact_requests_success counter\n")
	fmt.Fprintf(w, "openpact_requests_success %d\n\n", metrics.RequestsSuccess)

	fmt.Fprintf(w, "# HELP openpact_requests_error Failed requests\n")
	fmt.Fprintf(w, "# TYPE openpact_requests_error counter\n")
	fmt.Fprintf(w, "openpact_requests_error %d\n\n", metrics.RequestsError)

	fmt.Fprintf(w, "# HELP openpact_messages_received Messages received\n")
	fmt.Fprintf(w, "# TYPE openpact_messages_received counter\n")
	fmt.Fprintf(w, "openpact_messages_received %d\n\n", metrics.MessagesReceived)

	fmt.Fprintf(w, "# HELP openpact_messages_sent Messages sent\n")
	fmt.Fprintf(w, "# TYPE openpact_messages_sent counter\n")
	fmt.Fprintf(w, "openpact_messages_sent %d\n\n", metrics.MessagesSent)

	fmt.Fprintf(w, "# HELP openpact_tool_calls_total Total tool calls\n")
	fmt.Fprintf(w, "# TYPE openpact_tool_calls_total counter\n")
	fmt.Fprintf(w, "openpact_tool_calls_total %d\n\n", metrics.ToolCallsTotal)

	fmt.Fprintf(w, "# HELP openpact_tool_calls_success Successful tool calls\n")
	fmt.Fprintf(w, "# TYPE openpact_tool_calls_success counter\n")
	fmt.Fprintf(w, "openpact_tool_calls_success %d\n\n", metrics.ToolCallsSuccess)

	fmt.Fprintf(w, "# HELP openpact_tool_calls_error Failed tool calls\n")
	fmt.Fprintf(w, "# TYPE openpact_tool_calls_error counter\n")
	fmt.Fprintf(w, "openpact_tool_calls_error %d\n", metrics.ToolCallsError)
}

// Metric recording methods

// RecordRequest records a request
func (s *Server) RecordRequest(success bool) {
	atomic.AddUint64(&s.requestsTotal, 1)
	if success {
		atomic.AddUint64(&s.requestsSuccess, 1)
	} else {
		atomic.AddUint64(&s.requestsError, 1)
	}
}

// RecordMessage records a message
func (s *Server) RecordMessage(sent bool) {
	if sent {
		atomic.AddUint64(&s.messagesSent, 1)
	} else {
		atomic.AddUint64(&s.messagesReceived, 1)
	}
}

// RecordToolCall records a tool call
func (s *Server) RecordToolCall(success bool) {
	atomic.AddUint64(&s.toolCallsTotal, 1)
	if success {
		atomic.AddUint64(&s.toolCallsSuccess, 1)
	} else {
		atomic.AddUint64(&s.toolCallsError, 1)
	}
}

// GetMetrics returns current metrics snapshot
func (s *Server) GetMetrics() Metrics {
	return Metrics{
		RequestsTotal:    atomic.LoadUint64(&s.requestsTotal),
		RequestsSuccess:  atomic.LoadUint64(&s.requestsSuccess),
		RequestsError:    atomic.LoadUint64(&s.requestsError),
		MessagesReceived: atomic.LoadUint64(&s.messagesReceived),
		MessagesSent:     atomic.LoadUint64(&s.messagesSent),
		ToolCallsTotal:   atomic.LoadUint64(&s.toolCallsTotal),
		ToolCallsSuccess: atomic.LoadUint64(&s.toolCallsSuccess),
		ToolCallsError:   atomic.LoadUint64(&s.toolCallsError),
	}
}
