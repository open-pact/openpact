package mcp

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"

	"github.com/open-pact/openpact/internal/admin"
)

// SchedulerLookup provides access to schedule management at call time.
type SchedulerLookup interface {
	List() ([]*admin.Schedule, error)
	Get(id string) (*admin.Schedule, error)
	Create(sched *admin.Schedule) (*admin.Schedule, error)
	Update(id string, updates *admin.Schedule) (*admin.Schedule, error)
	Delete(id string) error
	SetEnabled(id string, enabled bool) error
}

// RegisterScheduleTools adds schedule management tools to the MCP server.
func RegisterScheduleTools(s *Server, lookup SchedulerLookup) {
	s.RegisterTool(scheduleListTool(lookup))
	s.RegisterTool(scheduleCreateTool(lookup))
	s.RegisterTool(scheduleUpdateTool(lookup))
	s.RegisterTool(scheduleDeleteTool(lookup))
	s.RegisterTool(scheduleEnableTool(lookup))
	s.RegisterTool(scheduleDisableTool(lookup))
}

func scheduleListTool(lookup SchedulerLookup) *Tool {
	return &Tool{
		Name:        "schedule_list",
		Description: "List all scheduled jobs. Returns each schedule's ID, name, type, cron expression, enabled status, and last run info.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			schedules, err := lookup.List()
			if err != nil {
				return nil, fmt.Errorf("failed to list schedules: %w", err)
			}

			result := make([]map[string]interface{}, len(schedules))
			for i, s := range schedules {
				entry := map[string]interface{}{
					"id":        s.ID,
					"name":      s.Name,
					"cron_expr": s.CronExpr,
					"type":      s.Type,
					"enabled":   s.Enabled,
				}
				if s.ScriptName != "" {
					entry["script_name"] = s.ScriptName
				}
				if s.Prompt != "" {
					entry["prompt"] = s.Prompt
				}
				if s.OutputTarget != nil {
					entry["output_target"] = map[string]string{
						"provider":   s.OutputTarget.Provider,
						"channel_id": s.OutputTarget.ChannelID,
					}
				}
				if s.RunOnce {
					entry["run_once"] = true
				}
				if s.LastRunAt != nil {
					entry["last_run_at"] = s.LastRunAt.Format("2006-01-02T15:04:05Z")
					entry["last_run_status"] = s.LastRunStatus
				}
				result[i] = entry
			}

			return map[string]interface{}{
				"schedules": result,
				"count":     len(schedules),
			}, nil
		},
	}
}

func scheduleCreateTool(lookup SchedulerLookup) *Tool {
	return &Tool{
		Name:        "schedule_create",
		Description: "Create a new scheduled job. Type must be 'script' (requires script_name) or 'agent' (requires prompt). Cron expression uses standard 5-field format (minute hour day-of-month month day-of-week).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Human-readable name for the schedule",
				},
				"cron_expr": map[string]interface{}{
					"type":        "string",
					"description": "Cron expression (5 fields: min hour dom month dow). Examples: '*/5 * * * *' (every 5 min), '0 9 * * 1-5' (weekdays at 9am)",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"script", "agent"},
					"description": "Job type: 'script' runs a Starlark script, 'agent' starts an AI session with a prompt",
				},
				"script_name": map[string]interface{}{
					"type":        "string",
					"description": "Script filename (for type=script), e.g. 'my_script.star'",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Prompt to send to a new AI session (for type=agent)",
				},
				"enabled": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the schedule is active (default: true)",
				},
				"output_provider": map[string]interface{}{
					"type":        "string",
					"description": "Optional: chat provider to send output to (e.g. 'discord')",
				},
				"output_channel": map[string]interface{}{
					"type":        "string",
					"description": "Optional: channel ID to send output to (e.g. 'channel:123456')",
				},
				"run_once": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, the schedule auto-disables after one execution. Useful for deferred one-off tasks.",
				},
			},
			"required": []string{"name", "cron_expr", "type"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			name, _ := args["name"].(string)
			cronExpr, _ := args["cron_expr"].(string)
			jobType, _ := args["type"].(string)
			scriptName, _ := args["script_name"].(string)
			prompt, _ := args["prompt"].(string)
			outputProvider, _ := args["output_provider"].(string)
			outputChannel, _ := args["output_channel"].(string)

			// Validate cron expression
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			if _, err := parser.Parse(cronExpr); err != nil {
				return nil, fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
			}

			enabled := true
			if e, ok := args["enabled"].(bool); ok {
				enabled = e
			}

			runOnce, _ := args["run_once"].(bool)

			sched := &admin.Schedule{
				Name:       name,
				CronExpr:   cronExpr,
				Type:       jobType,
				Enabled:    enabled,
				RunOnce:    runOnce,
				ScriptName: scriptName,
				Prompt:     prompt,
			}

			if outputProvider != "" && outputChannel != "" {
				sched.OutputTarget = &admin.OutputTarget{
					Provider:  outputProvider,
					ChannelID: outputChannel,
				}
			}

			created, err := lookup.Create(sched)
			if err != nil {
				return nil, fmt.Errorf("failed to create schedule: %w", err)
			}

			return map[string]interface{}{
				"status": "created",
				"id":     created.ID,
				"name":   created.Name,
			}, nil
		},
	}
}

func scheduleUpdateTool(lookup SchedulerLookup) *Tool {
	return &Tool{
		Name:        "schedule_update",
		Description: "Update an existing scheduled job by ID. Only provided fields are updated.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Schedule ID to update",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "New name",
				},
				"cron_expr": map[string]interface{}{
					"type":        "string",
					"description": "New cron expression",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"script", "agent"},
					"description": "New job type",
				},
				"script_name": map[string]interface{}{
					"type":        "string",
					"description": "New script name (for type=script)",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "New prompt (for type=agent)",
				},
				"output_provider": map[string]interface{}{
					"type":        "string",
					"description": "Chat provider for output delivery",
				},
				"output_channel": map[string]interface{}{
					"type":        "string",
					"description": "Channel ID for output delivery",
				},
				"run_once": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, the schedule auto-disables after one execution",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			id, _ := args["id"].(string)
			if id == "" {
				return nil, fmt.Errorf("id is required")
			}

			// Validate cron if provided
			if cronExpr, ok := args["cron_expr"].(string); ok && cronExpr != "" {
				parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
				if _, err := parser.Parse(cronExpr); err != nil {
					return nil, fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
				}
			}

			runOnce, _ := args["run_once"].(bool)

			updates := &admin.Schedule{
				Name:       strArg(args, "name"),
				CronExpr:   strArg(args, "cron_expr"),
				Type:       strArg(args, "type"),
				ScriptName: strArg(args, "script_name"),
				Prompt:     strArg(args, "prompt"),
				RunOnce:    runOnce,
			}

			outputProvider := strArg(args, "output_provider")
			outputChannel := strArg(args, "output_channel")
			if outputProvider != "" && outputChannel != "" {
				updates.OutputTarget = &admin.OutputTarget{
					Provider:  outputProvider,
					ChannelID: outputChannel,
				}
			}

			updated, err := lookup.Update(id, updates)
			if err != nil {
				return nil, fmt.Errorf("failed to update schedule: %w", err)
			}

			return map[string]interface{}{
				"status": "updated",
				"id":     updated.ID,
				"name":   updated.Name,
			}, nil
		},
	}
}

func scheduleDeleteTool(lookup SchedulerLookup) *Tool {
	return &Tool{
		Name:        "schedule_delete",
		Description: "Delete a scheduled job by ID.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Schedule ID to delete",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			id, _ := args["id"].(string)
			if id == "" {
				return nil, fmt.Errorf("id is required")
			}

			if err := lookup.Delete(id); err != nil {
				return nil, fmt.Errorf("failed to delete schedule: %w", err)
			}

			return map[string]interface{}{
				"status": "deleted",
				"id":     id,
			}, nil
		},
	}
}

func scheduleEnableTool(lookup SchedulerLookup) *Tool {
	return &Tool{
		Name:        "schedule_enable",
		Description: "Enable a scheduled job by ID.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Schedule ID to enable",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			id, _ := args["id"].(string)
			if id == "" {
				return nil, fmt.Errorf("id is required")
			}

			if err := lookup.SetEnabled(id, true); err != nil {
				return nil, fmt.Errorf("failed to enable schedule: %w", err)
			}

			return map[string]interface{}{
				"status": "enabled",
				"id":     id,
			}, nil
		},
	}
}

func scheduleDisableTool(lookup SchedulerLookup) *Tool {
	return &Tool{
		Name:        "schedule_disable",
		Description: "Disable a scheduled job by ID. The job will stop running but its configuration is preserved.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Schedule ID to disable",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			id, _ := args["id"].(string)
			if id == "" {
				return nil, fmt.Errorf("id is required")
			}

			if err := lookup.SetEnabled(id, false); err != nil {
				return nil, fmt.Errorf("failed to disable schedule: %w", err)
			}

			return map[string]interface{}{
				"status": "disabled",
				"id":     id,
			}, nil
		},
	}
}

func strArg(args map[string]interface{}, key string) string {
	v, _ := args[key].(string)
	return v
}
