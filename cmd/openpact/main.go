package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/config"
	"github.com/open-pact/openpact/internal/health"
	"github.com/open-pact/openpact/internal/logging"
	"github.com/open-pact/openpact/internal/orchestrator"
	"github.com/open-pact/openpact/internal/storage"
	"github.com/open-pact/openpact/internal/storage/chatproviders"
	"github.com/open-pact/openpact/internal/storage/kv"
	"github.com/open-pact/openpact/internal/storage/migrate"
	"github.com/open-pact/openpact/internal/storage/secrets"

	version "github.com/open-pact/openpact"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "start":
		start()
	case "version":
		fmt.Printf("openpact v%s\n", version.Get())
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`OpenPact - Secure AI Assistant Framework (v%s)

Usage:
  openpact <command>

Commands:
  start   Start the orchestrator + admin UI
  version Show version
  help    Show this help

Environment Variables:
  DISCORD_TOKEN   Discord bot token (may also be set via the admin UI)
  GITHUB_TOKEN    GitHub token (for the github MCP tool)
  WORKSPACE_PATH  Path to workspace (default: /workspace)
  CONFIG_PATH     Path to config.yaml (default: <workspace>/secure/config.yaml)
  ADMIN_BIND      Admin UI bind address (default from config.yaml)

All LLM provider credentials (OpenAI, Gemini, Copilot, Ollama) are
configured through the admin UI at /engine — no provider env vars are
read by the orchestrator.
`, version.Get())
}

func start() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Workspace.EnsureDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create workspace directories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("OpenPact v%s starting...\n", version.Get())
	fmt.Printf("  Workspace: %s\n", cfg.Workspace.Path)
	fmt.Printf("  Admin UI:  %v\n", cfg.Admin.Enabled)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived %s, shutting down...\n", sig)
		cancel()
	}()

	// Open the shared SQLite database once at the top of the process.
	// Every storage package (users, secrets, schedules, channels, kv,
	// calendars, chat providers, approvals) and the stackllm session
	// store all share this connection — op_* tables for OpenPact,
	// stackllm_* tables for stackllm.
	dbPath := cfg.Engine.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(cfg.Workspace.DataDir(), "stackllm.db")
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := migrate.Run(ctx, db); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run DB migrations: %v\n", err)
		os.Exit(1)
	}
	if err := storage.EnsureFilePerms(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: chmod database file: %v\n", err)
	}

	// Sweep stale JSON files that predated the DB migration. Best-effort
	// — if the workspace was provisioned by an older binary the files
	// are still there but no code reads them; cleaning them up now
	// prevents "why is there an old users.json?" confusion later.
	sweepLegacyFiles(cfg.Workspace.DataDir())

	// Load (or create) the workspace's data encryption key. Used by
	// the secrets store to encrypt op_secrets.value at rest; separate
	// from the JWT signing key so the two can rotate independently.
	encKey, err := secrets.LoadOrCreateKey(cfg.Workspace.DataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load data encryption key: %v\n", err)
		os.Exit(1)
	}
	secretStore, err := secrets.NewStore(db, encKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build secrets store: %v\n", err)
		os.Exit(1)
	}

	// Read advanced settings from the shared DB and actually wire the
	// knobs they represent. Up until this PR, PUT /api/config/advanced
	// persisted a JSON file nothing read — the logger/health/starlark
	// limits came from config.yaml defaults regardless of what the admin
	// UI showed.
	adv, err := kv.LoadAdvancedSettings(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load advanced settings: %v\n", err)
		os.Exit(1)
	}
	configureLogging(adv.Logging)

	// Start the health server on the configured address. handleCheck
	// verifies the DB is still reachable — a trivial ping so load
	// balancers know the binary is actually useful, not just booted.
	var healthSrv *health.Server
	if adv.Server.HealthAddr != "" {
		healthSrv = health.NewServer(adv.Server.HealthAddr)
		healthSrv.RegisterCheck("database", func(ctx context.Context) health.CheckResult {
			if err := db.PingContext(ctx); err != nil {
				return health.CheckResult{Status: health.StatusUnhealthy, Message: err.Error()}
			}
			return health.CheckResult{Status: health.StatusHealthy}
		})
		go func() {
			log.Printf("Health server listening on %s", adv.Server.HealthAddr)
			if err := healthSrv.Start(); err != nil && err != http.ErrServerClosed {
				log.Printf("Warning: health server error: %v", err)
			}
		}()
	}

	// Feed DB-backed Starlark limits into the config consumed by the
	// orchestrator + scheduler. Restart-to-apply — the underlying
	// sandbox reads these values at construction time.
	cfg.Starlark.Enabled = adv.Starlark.Enabled
	cfg.Starlark.MaxExecutionMs = adv.Starlark.MaxExecutionMs
	cfg.Starlark.MaxMemoryMB = adv.Starlark.MaxMemoryMB

	providerStore := chatproviders.NewStore(db)

	orch, err := orchestrator.New(cfg, db, secretStore, providerStore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create orchestrator: %v\n", err)
		os.Exit(1)
	}

	var (
		adminServer *admin.Server
		httpServer  *http.Server
	)
	if cfg.Admin.Enabled {
		adminConfig := admin.Config{
			DB:            db,
			Secrets:       secretStore,
			Bind:          cfg.Admin.Bind,
			DataDir:       cfg.Workspace.DataDir(),
			ScriptsDir:    cfg.Workspace.ScriptsDir(),
			WorkspacePath: cfg.Workspace.Path,
			AIDataDir:     cfg.Workspace.AIDataDir(),
			Allowlist:     cfg.Admin.Allowlist,
			AccessExpiry:  admin.DefaultConfig().AccessExpiry,
			RefreshExpiry: admin.DefaultConfig().RefreshExpiry,
		}

		adminServer, err = admin.NewServer(adminConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create admin server: %v\n", err)
			os.Exit(1)
		}

		// Wire the stackllm web.ManagedHandler under /api/engine/, and the
		// session store behind GET /api/engine/sessions for the admin UI.
		adminServer.SetEngineHandler(orch.Stack().Handler)
		adminServer.SetSessionStore(orch.Stack().Sessions)
		stack := orch.Stack()
		adminServer.SetDefaultModelCheck(func() bool {
			_, ok, err := stack.Manager.Default(context.Background())
			return err == nil && ok
		})
		adminServer.SetProviderManagerAPI(orch)
		adminServer.SetChannelModeAPI(orch)
		adminServer.SetSchedulerAPI(orch)

		handler, err := adminServer.HandlerWithUI()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create admin handler: %v\n", err)
			os.Exit(1)
		}

		httpServer = &http.Server{Addr: cfg.Admin.Bind, Handler: handler}
		go func() {
			fmt.Printf("  Admin UI: http://%s\n", cfg.Admin.Bind)
			if adminServer.SetupRequired() {
				fmt.Printf("  ⚠️  First-run setup required: http://%s/setup\n", cfg.Admin.Bind)
			}
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "Admin server error: %v\n", err)
			}
		}()
	}

	if err := orch.Start(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "Orchestrator error: %v\n", err)
		os.Exit(1)
	}

	if httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}
	if healthSrv != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = healthSrv.Stop(shutdownCtx)
	}

	fmt.Println("OpenPact stopped.")
}

// configureLogging routes the stdlib log package through the
// internal/logging Logger configured per admin-UI settings. The
// stdlib log package is what most of the codebase uses directly
// (log.Printf); wrapping its writer gives us level + JSON formatting
// without rewriting every callsite.
func configureLogging(s kv.LoggingSettings) {
	logger := logging.New(logging.Config{
		Level:      logging.ParseLevel(s.Level),
		Output:     os.Stderr,
		JSONFormat: s.JSON,
	})
	// The stdlib log package writes pre-formatted lines; we forward
	// them verbatim at Info level so filtering by the configured level
	// still works for the subset of calls explicitly using logging
	// methods, while legacy log.Printf continues to appear.
	log.SetFlags(0)
	log.SetOutput(loggerWriter{logger})
}

// loggerWriter adapts an io.Writer interface onto logging.Logger,
// treating every write as an Info-level structured entry.
type loggerWriter struct{ l *logging.Logger }

func (w loggerWriter) Write(p []byte) (int, error) {
	msg := string(p)
	if n := len(msg); n > 0 && msg[n-1] == '\n' {
		msg = msg[:n-1]
	}
	w.l.Info("%s", msg)
	return len(p), nil
}

// sweepLegacyFiles removes JSON state files from the pre-migration
// layout. Errors are ignored — a missing file is the expected state
// for a fresh workspace, and a permission error means operator
// intervention is needed regardless.
func sweepLegacyFiles(dataDir string) {
	legacy := []string{
		"users.json",
		"approvals.json",
		"starlark_secrets.json",
		"chat_providers.json",
		"schedules.json",
		"setup_state.json",
		"advanced_settings.json",
		"integrations.json",
		"channel_sessions.json",
		"channel_modes.json",
		"model_preference.json", // never existed post-stackllm but harmless to probe
	}
	for _, name := range legacy {
		_ = os.Remove(filepath.Join(dataDir, name))
	}
}
