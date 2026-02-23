// Command admin runs the OpenPact admin server.
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/config"
)

func main() {
	// Load .env file (real env vars take precedence)
	if err := config.LoadDotEnv(); err != nil {
		log.Fatalf("Failed to load .env: %v", err)
	}

	bind := os.Getenv("ADMIN_BIND")
	if bind == "" {
		bind = "localhost:8080"
	}

	dataDir := os.Getenv("ADMIN_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	scriptsDir := os.Getenv("ADMIN_SCRIPTS_DIR")
	if scriptsDir == "" {
		scriptsDir = "./scripts"
	}

	workspacePath := os.Getenv("WORKSPACE_PATH")
	if workspacePath == "" {
		workspacePath = filepath.Dir(dataDir)
	}

	config := admin.Config{
		Bind:          bind,
		DataDir:       dataDir,
		ScriptsDir:    scriptsDir,
		WorkspacePath: workspacePath,
		DevMode:       os.Getenv("ADMIN_DEV_MODE") == "true",
		AccessExpiry:  admin.DefaultConfig().AccessExpiry,
		RefreshExpiry: admin.DefaultConfig().RefreshExpiry,
	}

	server, err := admin.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

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
