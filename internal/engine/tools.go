package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/open-pact/openpact/internal/mcp"
	"github.com/stack-bound/stackllm/tools"
)

// RegisterMCPTools copies every tool registered on the MCP server into the
// stackllm tool registry via a thin adapter. The adapter preserves the
// MCP handler verbatim — schemas, security scoping (AI-data boundary),
// secret redaction and every other invariant the MCP tools already
// enforce continue to apply because the same Go function runs. The only
// transport change is that the LLM is now in-process and calls the
// handler through stackllm's dispatcher, so there is no HTTP hop.
func RegisterMCPTools(registry *tools.Registry, srv *mcp.Server) {
	if registry == nil || srv == nil {
		return
	}
	for _, t := range srv.ListTools() {
		registry.Add(newMCPToolAdapter(t))
	}
}

// mcpToolAdapter implements tools.Tool on top of an mcp.Tool. The schema
// is passed through verbatim — our MCP Tool.InputSchema uses the same
// JSON Schema shape stackllm expects in tools.Definition.Parameters.
type mcpToolAdapter struct {
	def     tools.Definition
	handler mcp.ToolHandler
}

func newMCPToolAdapter(t *mcp.Tool) *mcpToolAdapter {
	params := t.InputSchema
	if params == nil {
		params = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	return &mcpToolAdapter{
		def: tools.Definition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		},
		handler: t.Handler,
	}
}

// Definition is the tool description stackllm sends to the model.
func (a *mcpToolAdapter) Definition() tools.Definition { return a.def }

// Call is invoked by the stackllm agent loop when the model requests the
// tool. It parses the JSON argument blob, dispatches to the MCP handler,
// and stringifies the result for the tool_result block.
func (a *mcpToolAdapter) Call(ctx context.Context, arguments string) (string, error) {
	var args map[string]interface{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("engine: parse tool args for %q: %w", a.def.Name, err)
		}
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	result, err := a.handler(ctx, args)
	if err != nil {
		return "", err
	}

	return stringifyToolResult(result), nil
}

// stringifyToolResult converts arbitrary handler output to the string
// payload a tool_result block carries. Strings and byte slices pass
// through. Anything else is JSON-encoded so the model sees structured
// data rather than a Go %v rendering.
func stringifyToolResult(result interface{}) string {
	switch v := result.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	default:
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Sprintf("%v", result)
		}
		return string(data)
	}
}
