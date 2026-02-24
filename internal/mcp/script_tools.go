package mcp

import (
	"context"
	"fmt"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/starlark"
)

// ScriptConfig configures script execution tools
type ScriptConfig struct {
	ScriptsDir     string             // Directory containing .star scripts
	MaxExecutionMs int64              // Max execution time
	Secrets        map[string]string  // Secrets to inject into scripts (name -> value)
	ScriptStore    *admin.ScriptStore // Optional: script store for approval checking
}

// RegisterScriptTools registers Starlark script execution tools
func RegisterScriptTools(srv *Server, cfg ScriptConfig) {
	sandbox := starlark.New(starlark.Config{
		MaxExecutionMs: cfg.MaxExecutionMs,
	})
	loader := starlark.NewLoader(cfg.ScriptsDir, sandbox)

	// Create secret provider
	secretProvider := starlark.NewSecretProvider()
	for name, value := range cfg.Secrets {
		secretProvider.Set(name, value)
	}

	// Inject secrets into sandbox
	sandbox.InjectSecrets(secretProvider)

	srv.RegisterTool(scriptListTool(loader, cfg.ScriptStore))
	srv.RegisterTool(scriptRunTool(sandbox, loader, secretProvider, cfg.ScriptStore))
	srv.RegisterTool(scriptExecTool(sandbox, secretProvider))
	srv.RegisterTool(scriptReloadTool(loader))
}

// scriptListTool creates the script_list tool
func scriptListTool(loader *starlark.Loader, scriptStore *admin.ScriptStore) *Tool {
	return &Tool{
		Name:        "script_list",
		Description: "List all available Starlark scripts with their approval status",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			scripts, err := loader.List()
			if err != nil {
				return nil, err
			}

			// Return script metadata including required secrets and approval status
			result := make([]map[string]interface{}, len(scripts))
			for i, s := range scripts {
				requiredSecrets := starlark.ExtractRequiredSecrets(s.Source)
				scriptResult := map[string]interface{}{
					"name":             s.Name,
					"description":      s.Description,
					"metadata":         s.Metadata,
					"required_secrets": requiredSecrets,
				}

				// Add approval status if script store is available
				if scriptStore != nil {
					status := "unknown"
					if err := scriptStore.CanExecute(s.Name); err == nil {
						status = "approved"
					} else if err == admin.ErrScriptNotApproved {
						status = "pending"
					} else if err == admin.ErrScriptModified {
						status = "modified"
					}
					scriptResult["approval_status"] = status
				}

				result[i] = scriptResult
			}

			return map[string]interface{}{
				"scripts": result,
				"count":   len(scripts),
			}, nil
		},
	}
}

// scriptRunTool creates the script_run tool
func scriptRunTool(sandbox *starlark.Sandbox, loader *starlark.Loader, secretProvider *starlark.SecretProvider, scriptStore *admin.ScriptStore) *Tool {
	return &Tool{
		Name:        "script_run",
		Description: "Execute a Starlark script by name. Scripts must be approved before execution. Scripts can access secrets via secrets.get('NAME'). Results are sanitized to prevent secret leakage.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the script to run (without .star extension)",
				},
				"function": map[string]interface{}{
					"type":        "string",
					"description": "Optional: specific function to call within the script",
				},
				"args": map[string]interface{}{
					"type":        "array",
					"description": "Optional: arguments to pass to the function",
					"items":       map[string]interface{}{},
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			name, _ := args["name"].(string)
			function, _ := args["function"].(string)
			funcArgs, _ := args["args"].([]interface{})

			// Check script approval if script store is available
			scriptName := name
			if !hasStarExtension(name) {
				scriptName = name + ".star"
			}

			if scriptStore != nil {
				if err := scriptStore.CanExecute(scriptName); err != nil {
					if err == admin.ErrScriptNotApproved {
						return map[string]interface{}{
							"error":   "script_not_approved",
							"message": fmt.Sprintf("Script '%s' is pending approval. An administrator must review and approve this script before it can be executed.", scriptName),
							"script":  scriptName,
							"status":  "pending",
						}, nil
					}
					if err == admin.ErrScriptModified {
						return map[string]interface{}{
							"error":   "script_modified",
							"message": fmt.Sprintf("Script '%s' has been modified since approval. Re-approval required.", scriptName),
							"script":  scriptName,
							"status":  "modified",
						}, nil
					}
					// Other errors (not found, etc.)
					return nil, fmt.Errorf("script not available: %s", name)
				}
			}

			// Load the script
			script, err := loader.Load(name)
			if err != nil {
				return nil, fmt.Errorf("script not found: %s", name)
			}

			var result starlark.Result

			if function != "" {
				// Convert args to []any
				goArgs := make([]any, len(funcArgs))
				for i, a := range funcArgs {
					goArgs[i] = a
				}
				// Run specific function
				result = sandbox.ExecuteFunction(ctx, script.Name, script.Source, function, goArgs)
			} else {
				// Run entire script
				result = sandbox.Execute(ctx, script.Name, script.Source)
			}

			// CRITICAL: Sanitize result to prevent secret leakage
			result = starlark.SanitizeResult(result, secretProvider)

			return map[string]interface{}{
				"value":       result.Value,
				"error":       result.Error,
				"duration_ms": result.Duration.Milliseconds(),
			}, nil
		},
	}
}

func hasStarExtension(name string) bool {
	return len(name) > 5 && name[len(name)-5:] == ".star"
}

// scriptExecTool creates the script_exec tool
func scriptExecTool(sandbox *starlark.Sandbox, secretProvider *starlark.SecretProvider) *Tool {
	return &Tool{
		Name:        "script_exec",
		Description: "Execute arbitrary Starlark code. Scripts can access secrets via secrets.get('NAME'). Results are sanitized to prevent secret leakage.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"code": map[string]interface{}{
					"type":        "string",
					"description": "Starlark code to execute. Set 'result' variable for return value, or define main() function.",
				},
			},
			"required": []string{"code"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			code, _ := args["code"].(string)

			result := sandbox.Execute(ctx, "exec", code)

			// CRITICAL: Sanitize result to prevent secret leakage
			result = starlark.SanitizeResult(result, secretProvider)

			return map[string]interface{}{
				"value":       result.Value,
				"error":       result.Error,
				"duration_ms": result.Duration.Milliseconds(),
			}, nil
		},
	}
}

// scriptReloadTool creates the script_reload tool
func scriptReloadTool(loader *starlark.Loader) *Tool {
	return &Tool{
		Name:        "script_reload",
		Description: "Reload all scripts from the scripts directory. Use after adding or modifying script files.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			if err := loader.Reload(); err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"message": "Scripts reloaded successfully",
				"count":   loader.Count(),
			}, nil
		},
	}
}
