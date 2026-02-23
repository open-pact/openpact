package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VaultConfig holds vault configuration
type VaultConfig struct {
	Path     string // Local path to vault
	GitRepo  string // Git repository URL (optional)
	AutoSync bool   // Whether to auto git pull/push
}

// RegisterVaultTools adds vault-related tools to the server
func RegisterVaultTools(s *Server, cfg VaultConfig) {
	s.RegisterTool(vaultReadTool(cfg))
	s.RegisterTool(vaultWriteTool(cfg))
	s.RegisterTool(vaultListTool(cfg))
	s.RegisterTool(vaultSearchTool(cfg))
}

// vaultReadTool creates a tool for reading vault files
func vaultReadTool(cfg VaultConfig) *Tool {
	return &Tool{
		Name:        "vault_read",
		Description: "Read a file from the Obsidian vault",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path relative to vault root (e.g., 'Projects/OpenPact.md')",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return nil, fmt.Errorf("path is required")
			}

			// Security: prevent path traversal
			fullPath := filepath.Join(cfg.Path, path)
			if !strings.HasPrefix(fullPath, cfg.Path) {
				return nil, fmt.Errorf("path escapes vault")
			}

			// Auto-sync if configured
			if cfg.AutoSync && cfg.GitRepo != "" {
				if err := gitPull(cfg.Path); err != nil {
					// Log but don't fail - file might still be readable
					fmt.Printf("Warning: git pull failed: %v\n", err)
				}
			}

			data, err := os.ReadFile(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("file not found: %s", path)
				}
				return nil, fmt.Errorf("failed to read file: %w", err)
			}

			return string(data), nil
		},
	}
}

// vaultWriteTool creates a tool for writing vault files
func vaultWriteTool(cfg VaultConfig) *Tool {
	return &Tool{
		Name:        "vault_write",
		Description: "Write a file to the Obsidian vault. Creates directories as needed.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path relative to vault root (e.g., 'Projects/OpenPact.md')",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write (markdown)",
				},
				"commit_message": map[string]interface{}{
					"type":        "string",
					"description": "Git commit message (optional, defaults to 'Update {filename}')",
				},
			},
			"required": []string{"path", "content"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			commitMsg, _ := args["commit_message"].(string)

			if path == "" {
				return nil, fmt.Errorf("path is required")
			}

			// Security: prevent path traversal
			fullPath := filepath.Join(cfg.Path, path)
			if !strings.HasPrefix(fullPath, cfg.Path) {
				return nil, fmt.Errorf("path escapes vault")
			}

			// Create parent directories
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}

			// Write file
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write file: %w", err)
			}

			// Auto-sync if configured
			if cfg.AutoSync && cfg.GitRepo != "" {
				if commitMsg == "" {
					commitMsg = fmt.Sprintf("Update %s", filepath.Base(path))
				}
				if err := gitCommitAndPush(cfg.Path, path, commitMsg); err != nil {
					return nil, fmt.Errorf("file written but git sync failed: %w", err)
				}
				return fmt.Sprintf("Wrote and synced %s", path), nil
			}

			return fmt.Sprintf("Wrote %s", path), nil
		},
	}
}

// vaultListTool creates a tool for listing vault files
func vaultListTool(cfg VaultConfig) *Tool {
	return &Tool{
		Name:        "vault_list",
		Description: "List files in a vault directory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path relative to vault root (default: root)",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "List recursively (default: false)",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			recursive, _ := args["recursive"].(bool)

			if path == "" {
				path = "."
			}

			// Security: prevent path traversal
			fullPath := filepath.Join(cfg.Path, path)
			if !strings.HasPrefix(fullPath, cfg.Path) {
				return nil, fmt.Errorf("path escapes vault")
			}

			var files []string

			if recursive {
				err := filepath.Walk(fullPath, func(p string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					// Skip hidden files/dirs (like .git, .obsidian)
					if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					// Get relative path from vault root
					relPath, _ := filepath.Rel(cfg.Path, p)
					if info.IsDir() {
						files = append(files, relPath+"/")
					} else if strings.HasSuffix(info.Name(), ".md") {
						files = append(files, relPath)
					}
					return nil
				})
				if err != nil {
					return nil, fmt.Errorf("failed to list directory: %w", err)
				}
			} else {
				entries, err := os.ReadDir(fullPath)
				if err != nil {
					return nil, fmt.Errorf("failed to list directory: %w", err)
				}
				for _, entry := range entries {
					// Skip hidden files/dirs
					if strings.HasPrefix(entry.Name(), ".") {
						continue
					}
					name := entry.Name()
					if entry.IsDir() {
						name += "/"
					}
					files = append(files, name)
				}
			}

			if len(files) == 0 {
				return "No files found", nil
			}

			return strings.Join(files, "\n"), nil
		},
	}
}

// vaultSearchTool creates a tool for searching vault content
func vaultSearchTool(cfg VaultConfig) *Tool {
	return &Tool{
		Name:        "vault_search",
		Description: "Search for text in vault files",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Text to search for (case-insensitive)",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Limit search to this directory (optional)",
				},
			},
			"required": []string{"query"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			query, _ := args["query"].(string)
			path, _ := args["path"].(string)

			if query == "" {
				return nil, fmt.Errorf("query is required")
			}

			searchPath := cfg.Path
			if path != "" {
				searchPath = filepath.Join(cfg.Path, path)
				if !strings.HasPrefix(searchPath, cfg.Path) {
					return nil, fmt.Errorf("path escapes vault")
				}
			}

			queryLower := strings.ToLower(query)
			var results []string

			err := filepath.Walk(searchPath, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				// Skip hidden and non-markdown
				if strings.HasPrefix(info.Name(), ".") {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
					return nil
				}

				// Read and search file
				data, err := os.ReadFile(p)
				if err != nil {
					return nil
				}

				if strings.Contains(strings.ToLower(string(data)), queryLower) {
					relPath, _ := filepath.Rel(cfg.Path, p)
					results = append(results, relPath)
				}

				return nil
			})

			if err != nil {
				return nil, fmt.Errorf("search failed: %w", err)
			}

			if len(results) == 0 {
				return fmt.Sprintf("No files containing '%s'", query), nil
			}

			return fmt.Sprintf("Found in %d files:\n%s", len(results), strings.Join(results, "\n")), nil
		},
	}
}

// gitPull runs git pull in the given directory
func gitPull(dir string) error {
	cmd := exec.Command("git", "pull", "--rebase")
	cmd.Dir = dir
	return cmd.Run()
}

// gitCommitAndPush commits a file and pushes
func gitCommitAndPush(dir, file, message string) error {
	// git add
	addCmd := exec.Command("git", "add", file)
	addCmd.Dir = dir
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// git commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = dir
	if err := commitCmd.Run(); err != nil {
		// Not an error if nothing to commit
		return nil
	}

	// git push
	pushCmd := exec.Command("git", "push")
	pushCmd.Dir = dir
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	return nil
}
