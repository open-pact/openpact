package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// RegisterWebTools adds web-related tools to the server
func RegisterWebTools(s *Server) {
	s.RegisterTool(webFetchTool())
}

// webFetchTool creates a tool for fetching web content
func webFetchTool() *Tool {
	return &Tool{
		Name:        "web_fetch",
		Description: "Fetch content from a URL. Returns plain text with HTML tags stripped.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "URL to fetch",
				},
				"max_length": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum response length in characters (default: 50000)",
				},
			},
			"required": []string{"url"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			url, _ := args["url"].(string)
			maxLenFloat, _ := args["max_length"].(float64)
			maxLen := int(maxLenFloat)
			if maxLen <= 0 {
				maxLen = 50000
			}

			if url == "" {
				return nil, fmt.Errorf("url is required")
			}

			// Validate URL scheme
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				return nil, fmt.Errorf("URL must start with http:// or https://")
			}

			// Create request with timeout
			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("invalid URL: %w", err)
			}

			// Set a reasonable user agent
			req.Header.Set("User-Agent", "OpenPact/0.1 (AI Assistant)")

			resp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("fetch failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}

			// Read body with limit
			limitedReader := io.LimitReader(resp.Body, int64(maxLen*2)) // Allow for HTML overhead
			body, err := io.ReadAll(limitedReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read response: %w", err)
			}

			// Convert to plain text
			text := htmlToText(string(body))

			// Truncate if needed
			if len(text) > maxLen {
				text = text[:maxLen] + "\n\n[Content truncated]"
			}

			return text, nil
		},
	}
}

// htmlToText converts HTML to plain text
func htmlToText(html string) string {
	// Remove scripts and styles
	scriptRe := regexp.MustCompile(`(?is)<script.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	styleRe := regexp.MustCompile(`(?is)<style.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Remove HTML comments
	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	html = commentRe.ReplaceAllString(html, "")

	// Convert common block elements to newlines
	blockTags := []string{"p", "div", "br", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6"}
	for _, tag := range blockTags {
		re := regexp.MustCompile(fmt.Sprintf(`(?i)</?%s[^>]*>`, tag))
		html = re.ReplaceAllString(html, "\n")
	}

	// Remove remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text := tagRe.ReplaceAllString(html, "")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&apos;", "'")

	// Clean up whitespace
	// Multiple spaces to single space
	spaceRe := regexp.MustCompile(`[ \t]+`)
	text = spaceRe.ReplaceAllString(text, " ")

	// Multiple newlines to double newline (paragraph break)
	nlRe := regexp.MustCompile(`\n\s*\n+`)
	text = nlRe.ReplaceAllString(text, "\n\n")

	// Trim each line
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	// Remove leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}
