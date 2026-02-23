package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGitHubListIssuesTool(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/repos/owner/repo/issues") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected auth header, got: %s", r.Header.Get("Authorization"))
		}

		issues := []Issue{
			{
				Number:    1,
				Title:     "Test Issue",
				State:     "open",
				HTMLURL:   "https://github.com/owner/repo/issues/1",
				CreatedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
				Labels:    []Label{{Name: "bug"}},
			},
			{
				Number:    2,
				Title:     "Another Issue",
				State:     "open",
				HTMLURL:   "https://github.com/owner/repo/issues/2",
				CreatedAt: time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
				Labels:    []Label{},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(issues)
	}))
	defer server.Close()

	// Create client with mock server
	client := &GitHubClient{
		token:      "test-token",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	tool := githubListIssuesTool(client)

	if tool.Name != "github_list_issues" {
		t.Errorf("expected name 'github_list_issues', got '%s'", tool.Name)
	}

	args := map[string]interface{}{
		"owner": "owner",
		"repo":  "repo",
		"state": "open",
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := result.(string)
	if !strings.Contains(resultStr, "#1: Test Issue") {
		t.Errorf("expected issue #1 in result, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "[bug]") {
		t.Errorf("expected label in result, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "#2: Another Issue") {
		t.Errorf("expected issue #2 in result, got: %s", resultStr)
	}
}

func TestGitHubListIssuesToolMissingOwner(t *testing.T) {
	client := NewGitHubClient("test-token")
	tool := githubListIssuesTool(client)

	args := map[string]interface{}{
		"repo": "repo",
	}

	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error for missing owner")
	}
	if !strings.Contains(err.Error(), "owner is required") {
		t.Errorf("expected 'owner is required' error, got: %v", err)
	}
}

func TestGitHubListIssuesToolMissingRepo(t *testing.T) {
	client := NewGitHubClient("test-token")
	tool := githubListIssuesTool(client)

	args := map[string]interface{}{
		"owner": "owner",
	}

	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error for missing repo")
	}
	if !strings.Contains(err.Error(), "repo is required") {
		t.Errorf("expected 'repo is required' error, got: %v", err)
	}
}

func TestGitHubListIssuesToolNoToken(t *testing.T) {
	client := NewGitHubClient("") // Empty token
	tool := githubListIssuesTool(client)

	args := map[string]interface{}{
		"owner": "owner",
		"repo":  "repo",
	}

	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error when token is empty")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestGitHubListIssuesToolDefaultState(t *testing.T) {
	// Mock server that checks the state parameter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "state=open") {
			t.Errorf("expected state=open in query, got: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Issue{})
	}))
	defer server.Close()

	client := &GitHubClient{
		token:      "test-token",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	tool := githubListIssuesTool(client)

	args := map[string]interface{}{
		"owner": "owner",
		"repo":  "repo",
		// state not provided - should default to "open"
	}

	_, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitHubCreateIssueTool(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/repos/owner/repo/issues") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected auth header")
		}

		// Decode request body
		var req CreateIssueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		if req.Title != "New Feature" {
			t.Errorf("expected title 'New Feature', got '%s'", req.Title)
		}
		if req.Body != "Description here" {
			t.Errorf("expected body 'Description here', got '%s'", req.Body)
		}

		// Return created issue
		issue := Issue{
			Number:    42,
			Title:     req.Title,
			Body:      req.Body,
			State:     "open",
			HTMLURL:   "https://github.com/owner/repo/issues/42",
			CreatedAt: time.Now(),
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(issue)
	}))
	defer server.Close()

	client := &GitHubClient{
		token:      "test-token",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	tool := githubCreateIssueTool(client)

	if tool.Name != "github_create_issue" {
		t.Errorf("expected name 'github_create_issue', got '%s'", tool.Name)
	}

	args := map[string]interface{}{
		"owner": "owner",
		"repo":  "repo",
		"title": "New Feature",
		"body":  "Description here",
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := result.(string)
	if !strings.Contains(resultStr, "#42") {
		t.Errorf("expected issue number in result, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "New Feature") {
		t.Errorf("expected title in result, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "https://github.com/owner/repo/issues/42") {
		t.Errorf("expected URL in result, got: %s", resultStr)
	}
}

func TestGitHubCreateIssueToolWithLabels(t *testing.T) {
	var receivedLabels []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CreateIssueRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedLabels = req.Labels

		issue := Issue{
			Number:  1,
			Title:   req.Title,
			HTMLURL: "https://github.com/owner/repo/issues/1",
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(issue)
	}))
	defer server.Close()

	client := &GitHubClient{
		token:      "test-token",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	tool := githubCreateIssueTool(client)

	args := map[string]interface{}{
		"owner":  "owner",
		"repo":   "repo",
		"title":  "Bug Report",
		"labels": []interface{}{"bug", "priority-high"},
	}

	_, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(receivedLabels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(receivedLabels))
	}
	if receivedLabels[0] != "bug" || receivedLabels[1] != "priority-high" {
		t.Errorf("unexpected labels: %v", receivedLabels)
	}
}

func TestGitHubCreateIssueToolMissingTitle(t *testing.T) {
	client := NewGitHubClient("test-token")
	tool := githubCreateIssueTool(client)

	args := map[string]interface{}{
		"owner": "owner",
		"repo":  "repo",
	}

	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("expected 'title is required' error, got: %v", err)
	}
}

func TestGitHubCreateIssueToolNoToken(t *testing.T) {
	client := NewGitHubClient("")
	tool := githubCreateIssueTool(client)

	args := map[string]interface{}{
		"owner": "owner",
		"repo":  "repo",
		"title": "Test",
	}

	_, err := tool.Handler(context.Background(), args)
	if err == nil {
		t.Error("expected error when token is empty")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestRegisterGitHubTools(t *testing.T) {
	s := NewServer(nil, nil)
	cfg := GitHubConfig{Token: "test-token"}

	RegisterGitHubTools(s, cfg)

	tools := s.ListTools()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	if !names["github_list_issues"] {
		t.Error("expected 'github_list_issues' tool")
	}
	if !names["github_create_issue"] {
		t.Error("expected 'github_create_issue' tool")
	}
}

func TestGitHubClientListIssuesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	client := &GitHubClient{
		token:      "test-token",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	_, err := client.ListIssues(context.Background(), "owner", "nonexistent", "open")
	if err == nil {
		t.Error("expected error for API failure")
	}
	if !strings.Contains(err.Error(), "GitHub API error") {
		t.Errorf("expected API error message, got: %v", err)
	}
}

func TestGitHubClientCreateIssueAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "Bad credentials"}`))
	}))
	defer server.Close()

	client := &GitHubClient{
		token:      "bad-token",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	_, err := client.CreateIssue(context.Background(), "owner", "repo", CreateIssueRequest{Title: "Test"})
	if err == nil {
		t.Error("expected error for API failure")
	}
	if !strings.Contains(err.Error(), "GitHub API error") {
		t.Errorf("expected API error message, got: %v", err)
	}
}

func TestNewGitHubClient(t *testing.T) {
	client := NewGitHubClient("my-token")

	if client.token != "my-token" {
		t.Errorf("expected token 'my-token', got '%s'", client.token)
	}
	if client.baseURL != "https://api.github.com" {
		t.Errorf("expected base URL 'https://api.github.com', got '%s'", client.baseURL)
	}
	if client.httpClient == nil {
		t.Error("expected non-nil http client")
	}
}
