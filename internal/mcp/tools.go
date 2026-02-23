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

// RegisterDefaultTools adds the built-in tools to the server
func RegisterDefaultTools(s *Server, workspacePath string, reloadContext ContextReloader) {
	// Workspace tools
	s.RegisterTool(workspaceReadTool(workspacePath))
	s.RegisterTool(workspaceWriteTool(workspacePath))
	s.RegisterTool(workspaceListTool(workspacePath))

	// Memory tools
	s.RegisterTool(memoryReadTool(workspacePath))
	s.RegisterTool(memoryWriteTool(workspacePath, reloadContext))
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

// workspaceWriteTool creates the workspace_write tool
func workspaceWriteTool(basePath string) *Tool {
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

			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write file: %w", err)
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

// memoryReadTool creates a tool for reading memory files
func memoryReadTool(basePath string) *Tool {
	return &Tool{
		Name:        "memory_read",
		Description: "Read a context or memory file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file": map[string]interface{}{
					"type":        "string",
					"description": "Memory file: 'long-term' for MEMORY.md, 'soul' for SOUL.md, 'user-profile' for USER.md, or date like '2026-02-03' for daily",
				},
			},
			"required": []string{"file"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			file, _ := args["file"].(string)

			var path string
			if file == "long-term" {
				path = filepath.Join(basePath, "MEMORY.md")
			} else if file == "soul" {
				path = filepath.Join(basePath, "SOUL.md")
			} else if file == "user-profile" {
				path = filepath.Join(basePath, "USER.md")
			} else {
				// Assume it's a date
				path = filepath.Join(basePath, "memory", file+".md")
			}

			// Security check
			if !strings.HasPrefix(path, basePath) {
				return nil, fmt.Errorf("invalid memory path")
			}

			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return "", nil // Return empty for non-existent memory
				}
				return nil, fmt.Errorf("failed to read memory: %w", err)
			}

			return string(data), nil
		},
	}
}

// memoryWriteTool creates a tool for writing memory files
func memoryWriteTool(basePath string, reloadContext ContextReloader) *Tool {
	return &Tool{
		Name:        "memory_write",
		Description: "Write to a context or memory file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file": map[string]interface{}{
					"type":        "string",
					"description": "Memory file: 'long-term' for MEMORY.md, 'soul' for SOUL.md, 'user-profile' for USER.md, or date like '2026-02-03' for daily",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write",
				},
			},
			"required": []string{"file", "content"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			file, _ := args["file"].(string)
			content, _ := args["content"].(string)

			var path string
			if file == "long-term" {
				path = filepath.Join(basePath, "MEMORY.md")
			} else if file == "soul" {
				path = filepath.Join(basePath, "SOUL.md")
			} else if file == "user-profile" {
				path = filepath.Join(basePath, "USER.md")
			} else {
				// Assume it's a date - ensure memory directory exists
				memDir := filepath.Join(basePath, "memory")
				if err := os.MkdirAll(memDir, 0755); err != nil {
					return nil, fmt.Errorf("failed to create memory directory: %w", err)
				}
				path = filepath.Join(memDir, file+".md")
			}

			// Security check
			if !strings.HasPrefix(path, basePath) {
				return nil, fmt.Errorf("invalid memory path")
			}

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write memory: %w", err)
			}

			// Auto-reload context when writing files that are part of the system prompt
			if reloadContext != nil && (file == "soul" || file == "user-profile" || file == "long-term") {
				if err := reloadContext(); err != nil {
					log.Printf("Warning: failed to reload context after memory write: %v", err)
				}
			}

			return fmt.Sprintf("Wrote memory to %s", file), nil
		},
	}
}
