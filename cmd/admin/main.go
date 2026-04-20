// Command admin runs the OpenPact admin server without the chat-provider
// orchestrator. Useful for development: you can sign in to providers, set
// a default model, and drive a chat entirely through the web UI without
// running Discord/Slack/Telegram.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/config"
	"github.com/open-pact/openpact/internal/engine"
	"github.com/open-pact/openpact/internal/mcp"
)

func main() {
	if err := config.LoadDotEnv(); err != nil {
		log.Fatalf("Failed to load .env: %v", err)
	}

	bind := os.Getenv("ADMIN_BIND")
	if bind == "" {
		bind = "localhost:8888"
	}

	workspacePath := os.Getenv("WORKSPACE_PATH")
	if workspacePath == "" {
		workspacePath = "/workspace"
	}

	dataDir := workspacePath + "/secure/data"
	scriptsDir := workspacePath + "/ai-data/scripts"
	aiDataDir := workspacePath + "/ai-data"

	cfg := admin.Config{
		Bind:          bind,
		DataDir:       dataDir,
		ScriptsDir:    scriptsDir,
		WorkspacePath: workspacePath,
		AIDataDir:     aiDataDir,
		AccessExpiry:  admin.DefaultConfig().AccessExpiry,
		RefreshExpiry: admin.DefaultConfig().RefreshExpiry,
	}

	server, err := admin.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Minimal MCP server for the tool registry. Without the orchestrator we
	// can only register the workspace + memory tools (chat/model/schedule
	// tools all depend on orchestrator wiring).
	mcpSrv := mcp.NewServer(nil, nil)
	mcp.RegisterDefaultTools(mcpSrv, aiDataDir, nil)

	stack, err := engine.New(engine.Config{
		WorkspacePath: workspacePath,
		Tools:         mcpSrv,
	})
	if err != nil {
		log.Fatalf("Failed to build engine stack: %v", err)
	}
	defer stack.Close()
	server.SetEngineHandler(stack.Handler)
	server.SetSessionStore(stack.Sessions)

	handler, err := server.HandlerWithUI()
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	log.Printf("OpenPact Admin UI starting on http://%s", bind)
	if server.SetupRequired() {
		log.Printf("First-run setup required - visit http://%s/setup", bind)
	}

	if err := http.ListenAndServe(bind, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
