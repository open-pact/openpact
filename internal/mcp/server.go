// Package mcp implements the Model Context Protocol server for OpenPact.
// This is the security boundary - all AI capabilities are exposed through MCP tools.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
)

// Tool represents an MCP tool that can be called by the AI
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     ToolHandler            `json:"-"`
}

// ToolHandler is the function signature for tool implementations
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// Server is the MCP server that exposes tools to the AI
type Server struct {
	tools   map[string]*Tool
	reader  io.Reader
	writer  io.Writer
	mu      sync.RWMutex
	running bool
}

// NewServer creates a new MCP server
func NewServer(r io.Reader, w io.Writer) *Server {
	return &Server{
		tools:  make(map[string]*Tool),
		reader: r,
		writer: w,
	}
}

// RegisterTool adds a tool to the server
func (s *Server) RegisterTool(tool *Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
	log.Printf("MCP: Registered tool '%s'", tool.Name)
}

// ListTools returns all registered tools
func (s *Server) ListTools() []*Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]*Tool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}
	return tools
}

// Start begins processing MCP requests
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	decoder := json.NewDecoder(s.reader)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			log.Printf("MCP: Error decoding request: %v", err)
			continue
		}

		go s.handleRequest(ctx, req)
	}
}

// Stop stops the MCP server
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}

// processRequest processes a single MCP request and returns the response.
// Used by both the stdio path (handleRequest) and the HTTP handler.
func (s *Server) processRequest(ctx context.Context, req Request) Response {
	var resp Response
	resp.ID = req.ID
	resp.JSONRPC = "2.0"

	switch req.Method {
	case "initialize":
		resp.Result = s.handleInitialize(req)
	case "tools/list":
		resp.Result = s.handleToolsList()
	case "tools/call":
		result, err := s.handleToolCall(ctx, req)
		if err != nil {
			resp.Error = &Error{
				Code:    -32000,
				Message: err.Error(),
			}
		} else {
			resp.Result = result
		}
	default:
		resp.Error = &Error{
			Code:    -32601,
			Message: fmt.Sprintf("Unknown method: %s", req.Method),
		}
	}

	return resp
}

// handleRequest processes a single MCP request (stdio path).
func (s *Server) handleRequest(ctx context.Context, req Request) {
	resp := s.processRequest(ctx, req)
	s.sendResponse(resp)
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(req Request) interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "openpact-mcp",
			"version": "0.1.0",
		},
	}
}

// handleToolsList returns the list of available tools
func (s *Server) handleToolsList() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]map[string]interface{}, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}

	return map[string]interface{}{
		"tools": tools,
	}
}

// handleToolCall executes a tool
func (s *Server) handleToolCall(ctx context.Context, req Request) (interface{}, error) {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid params")
	}

	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]interface{})

	s.mu.RLock()
	tool, exists := s.tools[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	log.Printf("MCP: Calling tool '%s' with args: %v", name, args)

	result, err := tool.Handler(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("tool error: %w", err)
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("%v", result),
			},
		},
	}, nil
}

// sendResponse sends a JSON-RPC response
func (s *Server) sendResponse(resp Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("MCP: Error marshaling response: %v", err)
		return
	}

	if _, err := s.writer.Write(append(data, '\n')); err != nil {
		log.Printf("MCP: Error writing response: %v", err)
	}
}

// Request represents an MCP JSON-RPC request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
