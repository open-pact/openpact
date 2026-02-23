package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer(":8080")
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.addr != ":8080" {
		t.Errorf("addr = %q, want %q", s.addr, ":8080")
	}
}

func TestHealthEndpoint(t *testing.T) {
	s := NewServer(":8080")

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != StatusHealthy {
		t.Errorf("status = %q, want %q", resp.Status, StatusHealthy)
	}
}

func TestHealthEndpointWithCheck(t *testing.T) {
	s := NewServer(":8080")

	// Register a healthy check
	s.RegisterCheck("test", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusHealthy, Message: "all good"}
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != StatusHealthy {
		t.Errorf("status = %q, want %q", resp.Status, StatusHealthy)
	}

	if result, ok := resp.Checks["test"]; !ok {
		t.Error("expected 'test' check in response")
	} else if result.Status != StatusHealthy {
		t.Errorf("check status = %q, want %q", result.Status, StatusHealthy)
	}
}

func TestHealthEndpointUnhealthy(t *testing.T) {
	s := NewServer(":8080")

	// Register an unhealthy check
	s.RegisterCheck("failing", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUnhealthy, Message: "database down"}
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != StatusUnhealthy {
		t.Errorf("status = %q, want %q", resp.Status, StatusUnhealthy)
	}
}

func TestHealthEndpointDegraded(t *testing.T) {
	s := NewServer(":8080")

	s.RegisterCheck("slow", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusDegraded, Message: "slow response"}
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("degraded should return 200, got %d", w.Code)
	}

	var resp HealthResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != StatusDegraded {
		t.Errorf("status = %q, want %q", resp.Status, StatusDegraded)
	}
}

func TestReadyEndpoint(t *testing.T) {
	s := NewServer(":8080")

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	s.handleReady(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != "ready" {
		t.Errorf("status = %q, want %q", resp["status"], "ready")
	}
}

func TestMetricsEndpointJSON(t *testing.T) {
	s := NewServer(":8080")

	// Record some metrics
	s.RecordRequest(true)
	s.RecordRequest(true)
	s.RecordRequest(false)
	s.RecordMessage(false) // received
	s.RecordMessage(true)  // sent
	s.RecordToolCall(true)

	req := httptest.NewRequest("GET", "/metrics?format=json", nil)
	w := httptest.NewRecorder()

	s.handleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp MetricsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Metrics.RequestsTotal != 3 {
		t.Errorf("RequestsTotal = %d, want 3", resp.Metrics.RequestsTotal)
	}
	if resp.Metrics.RequestsSuccess != 2 {
		t.Errorf("RequestsSuccess = %d, want 2", resp.Metrics.RequestsSuccess)
	}
	if resp.Metrics.RequestsError != 1 {
		t.Errorf("RequestsError = %d, want 1", resp.Metrics.RequestsError)
	}
	if resp.Metrics.MessagesReceived != 1 {
		t.Errorf("MessagesReceived = %d, want 1", resp.Metrics.MessagesReceived)
	}
	if resp.Metrics.MessagesSent != 1 {
		t.Errorf("MessagesSent = %d, want 1", resp.Metrics.MessagesSent)
	}
	if resp.Metrics.ToolCallsTotal != 1 {
		t.Errorf("ToolCallsTotal = %d, want 1", resp.Metrics.ToolCallsTotal)
	}
}

func TestMetricsEndpointPrometheus(t *testing.T) {
	s := NewServer(":8080")

	s.RecordRequest(true)
	s.RecordToolCall(false)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	s.handleMetrics(w, req)

	body := w.Body.String()

	// Check Prometheus format
	if !strings.Contains(body, "# HELP openpact_requests_total") {
		t.Error("missing HELP for requests_total")
	}
	if !strings.Contains(body, "# TYPE openpact_requests_total counter") {
		t.Error("missing TYPE for requests_total")
	}
	if !strings.Contains(body, "openpact_requests_total 1") {
		t.Error("missing requests_total value")
	}
	if !strings.Contains(body, "openpact_tool_calls_error 1") {
		t.Error("missing tool_calls_error value")
	}
}

func TestGetMetrics(t *testing.T) {
	s := NewServer(":8080")

	s.RecordRequest(true)
	s.RecordRequest(true)

	metrics := s.GetMetrics()

	if metrics.RequestsTotal != 2 {
		t.Errorf("RequestsTotal = %d, want 2", metrics.RequestsTotal)
	}
	if metrics.RequestsSuccess != 2 {
		t.Errorf("RequestsSuccess = %d, want 2", metrics.RequestsSuccess)
	}
}

func TestMetricsConcurrency(t *testing.T) {
	s := NewServer(":8080")

	// Concurrent metric recording
	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func() {
			s.RecordRequest(true)
			s.RecordMessage(true)
			s.RecordToolCall(true)
			done <- struct{}{}
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	metrics := s.GetMetrics()
	if metrics.RequestsTotal != 100 {
		t.Errorf("RequestsTotal = %d, want 100", metrics.RequestsTotal)
	}
	if metrics.MessagesSent != 100 {
		t.Errorf("MessagesSent = %d, want 100", metrics.MessagesSent)
	}
	if metrics.ToolCallsTotal != 100 {
		t.Errorf("ToolCallsTotal = %d, want 100", metrics.ToolCallsTotal)
	}
}
