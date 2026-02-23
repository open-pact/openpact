---
title: Web Fetching
sidebar_position: 8
---

# Web Fetching

OpenPact includes a web fetching tool that allows your AI assistant to retrieve and read content from web pages. This enables research, information gathering, and accessing online resources.

## Overview

The web fetching integration provides:

- **Page Retrieval**: Fetch content from any public URL
- **Content Parsing**: Extract readable text from HTML pages
- **Metadata Extraction**: Capture page titles, descriptions, and more
- **Safe Execution**: Rate limiting and security controls

## How It Works

When the AI needs information from a web page:

1. It calls the `web_fetch` tool with a URL
2. OpenPact retrieves the page content
3. HTML is parsed and converted to readable text
4. The cleaned content is returned to the AI

This allows the AI to read articles, documentation, and other web content to help answer your questions.

## Fetching Web Pages

Use the `web_fetch` tool to retrieve web content.

### Tool Usage

```json
{
  "name": "web_fetch",
  "arguments": {
    "url": "https://example.com/article"
  }
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | The URL to fetch |

### Example

```json
{
  "name": "web_fetch",
  "arguments": {
    "url": "https://go.dev/doc/effective_go"
  }
}
```

### Response Format

The tool returns:

- **title**: Page title (from `<title>` tag)
- **content**: Cleaned text content
- **url**: The fetched URL (may differ if redirected)
- **status**: HTTP status code

Example response:
```json
{
  "title": "Effective Go - The Go Programming Language",
  "content": "Introduction\n\nGo is a new language. Although it borrows ideas from existing languages...",
  "url": "https://go.dev/doc/effective_go",
  "status": 200
}
```

## Content Parsing

OpenPact processes fetched pages to extract meaningful content.

### What Gets Extracted

- **Main content**: Article text, documentation, blog posts
- **Headings**: Page structure preserved
- **Lists**: Bullet points and numbered lists
- **Code blocks**: Programming examples (when identifiable)

### What Gets Removed

- **Navigation**: Menus, sidebars, footers
- **Advertisements**: Ad blocks and promotional content
- **Scripts**: JavaScript code
- **Styles**: CSS styling
- **Hidden elements**: Elements not visible to users

### Text Formatting

The extracted content is formatted for readability:

```
Original HTML:
<article>
  <h1>Getting Started</h1>
  <p>Welcome to the <strong>documentation</strong>.</p>
  <ul>
    <li>First step</li>
    <li>Second step</li>
  </ul>
</article>

Extracted text:
Getting Started

Welcome to the documentation.

- First step
- Second step
```

## Use Cases

### Research and Information Gathering

```
User: "What's new in the latest Go release?"

AI: Let me check the Go blog.
[Uses web_fetch with url "https://go.dev/blog"]

According to the Go blog, the latest release is Go 1.22 which includes:
- Enhanced for loops with integer ranges
- Improved HTTP routing patterns
- Performance improvements...
```

### Documentation Lookup

```
User: "How does Docker's networking work?"

AI: Let me look up Docker's networking documentation.
[Uses web_fetch with url "https://docs.docker.com/network/"]

Docker provides several network drivers:
1. Bridge - default network for containers
2. Host - removes network isolation
3. Overlay - connects multiple daemons
...
```

### News and Updates

```
User: "What's happening in tech news today?"

AI: Let me check a tech news site.
[Uses web_fetch]

Here are today's top stories:
- Company X announces new product
- Security vulnerability discovered in...
...
```

### Reading Articles

```
User: "Can you summarize this article for me? [URL]"

AI: Let me fetch and read that article.
[Uses web_fetch]

Here's a summary of the article:
The article discusses [topic] and makes three main points...
```

## Configuration

Web fetching can be configured in `openpact.yaml`:

```yaml
web:
  enabled: true
  timeout_seconds: 30
  max_size_mb: 5
  user_agent: "OpenPact/1.0"
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | boolean | true | Enable/disable web fetching |
| `timeout_seconds` | number | 30 | Request timeout |
| `max_size_mb` | number | 5 | Maximum response size |
| `user_agent` | string | OpenPact/1.0 | User agent for requests |

## Rate Limiting

Web fetching includes built-in rate limiting to be a good internet citizen.

### Default Limits

- Requests are rate-limited per domain
- Minimum delay between requests to the same domain
- Respects `Retry-After` headers

### Configuration

```yaml
web:
  enabled: true
  rate_limit:
    requests_per_second: 1
    burst: 3
```

## Security Considerations

### URL Restrictions

OpenPact only fetches from safe URLs:

- **Allowed**: `http://` and `https://` protocols
- **Blocked**: `file://`, `ftp://`, and other protocols
- **Private networks**: Internal/private IP ranges are blocked by default

### Content Safety

- Response size is limited to prevent memory issues
- Timeout prevents hanging on slow responses
- Malicious content is sanitized during parsing

### Privacy

When fetching pages:

- Your IP address is visible to the target server
- Some pages may track visitors
- Consider privacy implications for sensitive research

## Limitations

### Dynamic Content

Web fetching retrieves the initial HTML only:

- **Not captured**: Content loaded by JavaScript
- **Single page apps**: May return minimal content
- **Login-required pages**: Cannot authenticate

For JavaScript-heavy sites, the extracted content may be incomplete.

### Rate Limits

External rate limits may apply:

- Some sites block automated access
- APIs may require authentication
- CDNs may impose limits

### Content Types

Best suited for:

- HTML web pages
- Documentation sites
- News articles
- Blog posts

Less suitable for:

- PDFs (not parsed)
- Images (not processed)
- Video content
- Interactive applications

## Troubleshooting

### Empty Content

If `web_fetch` returns empty content:

1. The page may require JavaScript to render
2. The page may block automated access
3. Check if the URL is correct and accessible

### Timeout Errors

If requests time out:

1. The server may be slow or unresponsive
2. Try again later
3. Check network connectivity
4. Increase timeout in configuration

### Blocked Requests

If requests are blocked:

1. The site may block automated access
2. Rate limiting may be in effect
3. The user agent may be blocked
4. Some sites require specific headers

### Garbled Content

If content appears garbled:

1. The page may use unusual encoding
2. The content may be heavily JavaScript-dependent
3. Try a different URL for the same information

## Best Practices

### URL Selection

- Use direct links to content pages
- Avoid URLs that redirect multiple times
- Prefer simple, clean URLs

### Request Frequency

- Don't fetch the same page repeatedly
- Allow time between requests to the same site
- Cache results when appropriate

### Content Verification

- Verify important information from multiple sources
- Be aware of outdated cached content
- Check page dates when relevant

## Related Documentation

- **[MCP Tools Reference](./mcp-tools)** - Complete tool documentation
- **[Configuration Overview](../configuration/overview)** - General configuration
- **[Starlark Scripting](../starlark/overview)** - For custom web integrations
