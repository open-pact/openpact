// Package main implements the standalone MCP server binary for OpenPact.
// This binary is launched by OpenCode as a child process. It reads JSON-RPC
// requests from stdin and writes responses to stdout. Configuration is received
// via environment variables from the parent process.
//
// Environment variables:
//
//	OPENPACT_WORKSPACE_PATH - Workspace root directory (all paths derived from this)
//	OPENPACT_FEATURES       - Comma-separated feature flags
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/open-pact/openpact/internal/mcp"
	"github.com/open-pact/openpact/internal/storage"
	"github.com/open-pact/openpact/internal/storage/migrate"
	"github.com/open-pact/openpact/internal/storage/secrets"
)

func main() {
	// Send logs to stderr so they don't interfere with JSON-RPC on stdout
	log.SetOutput(os.Stderr)
	log.SetPrefix("[mcp-server] ")

	workspacePath := os.Getenv("OPENPACT_WORKSPACE_PATH")
	if workspacePath == "" {
		workspacePath = "/workspace"
	}

	features := os.Getenv("OPENPACT_FEATURES")

	aiDataDir := workspacePath + "/ai-data"
	dataDir := workspacePath + "/secure/data"

	log.Printf("Starting MCP server (workspace=%s, ai-data=%s, data=%s)", workspacePath, aiDataDir, dataDir)

	// Share the workspace SQLite database so approvals and secrets line
	// up with whatever the admin UI wrote. We open it read-write (not
	// read-only) — the stdio MCP server may update approvals when the
	// AI writes scripts via future script.write tools.
	dbPath := filepath.Join(dataDir, "stackllm.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("open shared database: %v", err)
	}
	defer db.Close()

	if err := migrate.Run(context.Background(), db); err != nil {
		log.Fatalf("run op_* migrations: %v", err)
	}

	encKey, err := secrets.LoadOrCreateKey(dataDir)
	if err != nil {
		log.Fatalf("load data encryption key: %v", err)
	}
	secretStore, err := secrets.NewStore(db, encKey)
	if err != nil {
		log.Fatalf("build secrets store: %v", err)
	}

	// Create MCP server reading from stdin, writing to stdout
	server := mcp.NewServer(os.Stdin, os.Stdout)

	// Register all tools based on environment config
	mcp.RegisterAllToolsFromEnv(server, workspacePath, features, db, secretStore)

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
