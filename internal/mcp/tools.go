package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ContextReloader is a callback to reload context files after memory writes.
type ContextReloader func() error

// contextFiles are files that, when written, should trigger a context reload
// so the system prompt stays in sync with the AI's memory.
var contextFiles = map[string]bool{
	"MEMORY.md": true,
	"SOUL.md":   true,
	"USER.md":   true,
}

// RegisterDefaultTools adds the built-in tools to the server.
// aiDataPath is the AI-accessible data directory; all workspace tools
// are scoped exclusively to this path.
func RegisterDefaultTools(s *Server, aiDataPath string, reloadContext ContextReloader) {
	s.RegisterTool(workspaceReadTool(aiDataPath))
	s.RegisterTool(workspaceWriteTool(aiDataPath, reloadContext))
	s.RegisterTool(workspaceListTool(aiDataPath))
}

// workspaceReadTool creates the workspace_read tool
func workspaceReadTool(basePath string) *Tool {
	return &Tool{
		Name:        "workspace_read",
		Description: "Read a file from the workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path relative to workspace root",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return nil, fmt.Errorf("path is required")
			}

			// Security: ensure path stays within workspace
			fullPath := filepath.Join(basePath, path)
			if !strings.HasPrefix(fullPath, basePath) {
				return nil, fmt.Errorf("path escapes workspace")
			}

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file: %w", err)
			}

			return string(data), nil
		},
	}
}

// workspaceWriteTool creates the workspace_write tool.
// When a context file (SOUL.md, USER.md, MEMORY.md) or any file under
// memory/ (daily memory files) is written, the context is automatically
// reloaded so the system prompt stays current.
func workspaceWriteTool(basePath string, reloadContext ContextReloader) *Tool {
	return &Tool{
		Name:        "workspace_write",
		Description: "Write a file to the workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path relative to workspace root",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write",
				},
			},
			"required": []string{"path", "content"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)

			if path == "" {
				return nil, fmt.Errorf("path is required")
			}

			// Security: ensure path stays within workspace
			fullPath := filepath.Join(basePath, path)
			if !strings.HasPrefix(fullPath, basePath) {
				return nil, fmt.Errorf("path escapes workspace")
			}

			// Create parent directories if needed
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}

			if err := os.WriteFile(fullPath, []byte(content), 0664); err != nil {
				return nil, fmt.Errorf("failed to write file: %w", err)
			}

			// Auto-reload context when a context file is written.
			// This includes the root context files (MEMORY.md, SOUL.md, USER.md)
			// and any file under the memory/ directory (daily memory files).
			shouldReload := contextFiles[filepath.Base(fullPath)] ||
				strings.HasPrefix(fullPath, filepath.Join(basePath, "memory")+string(filepath.Separator))
			if reloadContext != nil && shouldReload {
				if err := reloadContext(); err != nil {
					log.Printf("Warning: failed to reload context after writing %s: %v", path, err)
				}
			}

			return fmt.Sprintf("Wrote %d bytes to %s", len(content), path), nil
		},
	}
}

// workspaceListTool creates the workspace_list tool
func workspaceListTool(basePath string) *Tool {
	return &Tool{
		Name:        "workspace_list",
		Description: "List files in a workspace directory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path relative to workspace root (default: root)",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}

			// Security: ensure path stays within workspace
			fullPath := filepath.Join(basePath, path)
			if !strings.HasPrefix(fullPath, basePath) {
				return nil, fmt.Errorf("path escapes workspace")
			}

			entries, err := os.ReadDir(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to list directory: %w", err)
			}

			var files []string
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() {
					name += "/"
				}
				files = append(files, name)
			}

			return strings.Join(files, "\n"), nil
		},
	}
}

