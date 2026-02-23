package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitHubConfig holds GitHub API configuration
type GitHubConfig struct {
	Token string // GitHub personal access token
}

// GitHubClient is a simple client for GitHub API operations
type GitHubClient struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

// NewGitHubClient creates a new GitHub API client
func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.github.com",
	}
}

// Issue represents a GitHub issue
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	Labels    []Label   `json:"labels"`
}

// Label represents a GitHub label
type Label struct {
	Name string `json:"name"`
}

// CreateIssueRequest represents a request to create an issue
type CreateIssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// doRequest performs an authenticated HTTP request to GitHub API
func (c *GitHubClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

// ListIssues lists issues for a repository
func (c *GitHubClient) ListIssues(ctx context.Context, owner, repo string, state string) ([]Issue, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues?state=%s&per_page=30", owner, repo, state)

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(body))
	}

	var issues []Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return issues, nil
}

// CreateIssue creates a new issue in a repository
func (c *GitHubClient) CreateIssue(ctx context.Context, owner, repo string, issue CreateIssueRequest) (*Issue, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues", owner, repo)

	body, err := json.Marshal(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", path, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	var created Issue
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &created, nil
}

// RegisterGitHubTools adds GitHub-related tools to the server
func RegisterGitHubTools(s *Server, cfg GitHubConfig) {
	client := NewGitHubClient(cfg.Token)

	s.RegisterTool(githubListIssuesTool(client))
	s.RegisterTool(githubCreateIssueTool(client))
}

// githubListIssuesTool creates a tool for listing GitHub issues
func githubListIssuesTool(client *GitHubClient) *Tool {
	return &Tool{
		Name:        "github_list_issues",
		Description: "List issues in a GitHub repository. Returns up to 30 issues.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"state": map[string]interface{}{
					"type":        "string",
					"description": "Issue state: 'open', 'closed', or 'all' (default: 'open')",
					"enum":        []string{"open", "closed", "all"},
				},
			},
			"required": []string{"owner", "repo"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			owner, _ := args["owner"].(string)
			repo, _ := args["repo"].(string)
			state, _ := args["state"].(string)

			if owner == "" {
				return nil, fmt.Errorf("owner is required")
			}
			if repo == "" {
				return nil, fmt.Errorf("repo is required")
			}
			if state == "" {
				state = "open"
			}

			if client == nil || client.token == "" {
				return nil, fmt.Errorf("GitHub not configured: token required")
			}

			issues, err := client.ListIssues(ctx, owner, repo, state)
			if err != nil {
				return nil, fmt.Errorf("failed to list issues: %w", err)
			}

			// Format output
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Issues in %s/%s (%s):\n\n", owner, repo, state))

			if len(issues) == 0 {
				sb.WriteString("No issues found.")
				return sb.String(), nil
			}

			for _, issue := range issues {
				labels := ""
				if len(issue.Labels) > 0 {
					labelNames := make([]string, len(issue.Labels))
					for i, l := range issue.Labels {
						labelNames[i] = l.Name
					}
					labels = fmt.Sprintf(" [%s]", strings.Join(labelNames, ", "))
				}
				sb.WriteString(fmt.Sprintf("#%d: %s%s\n", issue.Number, issue.Title, labels))
				sb.WriteString(fmt.Sprintf("    URL: %s\n", issue.HTMLURL))
				sb.WriteString(fmt.Sprintf("    Created: %s\n\n", issue.CreatedAt.Format("2006-01-02")))
			}

			return sb.String(), nil
		},
	}
}

// githubCreateIssueTool creates a tool for creating GitHub issues
func githubCreateIssueTool(client *GitHubClient) *Tool {
	return &Tool{
		Name:        "github_create_issue",
		Description: "Create a new issue in a GitHub repository.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Issue title",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Issue body/description (optional, supports Markdown)",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"description": "Labels to apply to the issue (optional)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"owner", "repo", "title"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			owner, _ := args["owner"].(string)
			repo, _ := args["repo"].(string)
			title, _ := args["title"].(string)
			body, _ := args["body"].(string)

			if owner == "" {
				return nil, fmt.Errorf("owner is required")
			}
			if repo == "" {
				return nil, fmt.Errorf("repo is required")
			}
			if title == "" {
				return nil, fmt.Errorf("title is required")
			}

			if client == nil || client.token == "" {
				return nil, fmt.Errorf("GitHub not configured: token required")
			}

			// Handle labels array
			var labels []string
			if labelsRaw, ok := args["labels"].([]interface{}); ok {
				for _, l := range labelsRaw {
					if s, ok := l.(string); ok {
						labels = append(labels, s)
					}
				}
			}

			req := CreateIssueRequest{
				Title:  title,
				Body:   body,
				Labels: labels,
			}

			issue, err := client.CreateIssue(ctx, owner, repo, req)
			if err != nil {
				return nil, fmt.Errorf("failed to create issue: %w", err)
			}

			return fmt.Sprintf("Created issue #%d: %s\nURL: %s", issue.Number, issue.Title, issue.HTMLURL), nil
		},
	}
}
