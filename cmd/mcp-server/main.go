// Package main implements the standalone MCP server binary for OpenPact.
// This binary is launched by OpenCode as a child process. It reads JSON-RPC
// requests from stdin and writes responses to stdout. Configuration is received
// via environment variables from the parent process.
//
// Environment variables:
//   OPENPACT_WORKSPACE_PATH - Workspace root directory
//   OPENPACT_DATA_DIR       - Data directory for secrets/approvals
//   OPENPACT_FEATURES       - Comma-separated feature flags
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/open-pact/openpact/internal/mcp"
)

func main() {
	// Send logs to stderr so they don't interfere with JSON-RPC on stdout
	log.SetOutput(os.Stderr)
	log.SetPrefix("[mcp-server] ")

	workspacePath := os.Getenv("OPENPACT_WORKSPACE_PATH")
	if workspacePath == "" {
		workspacePath = "/workspace"
	}

	dataDir := os.Getenv("OPENPACT_DATA_DIR")
	if dataDir == "" {
		dataDir = workspacePath + "/data"
	}

	features := os.Getenv("OPENPACT_FEATURES")

	log.Printf("Starting MCP server (workspace=%s, data=%s)", workspacePath, dataDir)

	// Create MCP server reading from stdin, writing to stdout
	server := mcp.NewServer(os.Stdin, os.Stdout)

	// Register all tools based on environment config
	mcp.RegisterAllToolsFromEnv(server, workspacePath, dataDir, features)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received %s, shutting down", sig)
		cancel()
	}()

	// Start processing JSON-RPC requests (blocks until stdin closes or context cancelled)
	if err := server.Start(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}

	log.Println("MCP server stopped")
}
