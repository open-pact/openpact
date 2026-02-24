package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// MCPPort is the fixed port for the in-process MCP HTTP server.
const MCPPort = 3100

// HTTPHandler returns an http.Handler implementing Streamable HTTP transport
// for the MCP server. Only POST is supported (JSON-RPC request → JSON response).
func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("MCP HTTP: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		if r.Method != http.MethodPost {
			log.Printf("MCP HTTP: rejecting %s (only POST allowed)", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read and parse JSON-RPC request
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("MCP HTTP: failed to read body: %v", err)
			writeJSONRPCError(w, nil, -32700, "Failed to read request body")
			return
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			log.Printf("MCP HTTP: parse error: %v (body: %s)", err, truncate(string(body), 200))
			writeJSONRPCError(w, nil, -32700, "Parse error")
			return
		}

		log.Printf("MCP HTTP: request method=%s id=%v", req.Method, req.ID)

		// Process the request using the shared logic
		resp := s.processRequest(r.Context(), req)

		if resp.Error != nil {
			log.Printf("MCP HTTP: response error code=%d msg=%s", resp.Error.Code, resp.Error.Message)
		} else {
			log.Printf("MCP HTTP: response OK for method=%s", req.Method)
		}

		// Write JSON-RPC response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("MCP HTTP: failed to write response: %v", err)
		}
	})
}

// truncate truncates a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// BearerTokenMiddleware returns middleware that validates Bearer token auth.
func BearerTokenMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + token
		if !strings.EqualFold(auth, expected) {
			log.Printf("MCP HTTP: auth rejected from %s (got %q)", r.RemoteAddr, truncate(auth, 20))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GenerateToken generates a cryptographically random hex-encoded token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// writeJSONRPCError writes a JSON-RPC error response.
func writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("MCP HTTP: failed to write error response: %v", err)
	}
}
