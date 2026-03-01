// Package scheduler runs cron-based scheduled jobs.
// Jobs can execute Starlark scripts or start AI agent sessions.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/open-pact/openpact/internal/admin"
	"github.com/open-pact/openpact/internal/engine"
	"github.com/open-pact/openpact/internal/starlark"
)

// ChatAPI sends messages via chat providers.
type ChatAPI interface {
	SendViaProvider(provider, target, content string) error
}

// EngineAPI creates sessions and sends messages.
type EngineAPI interface {
	CreateSession() (*engine.Session, error)
	Send(ctx context.Context, sessionID string, messages []engine.Message) (<-chan engine.Response, error)
}

// Scheduler manages cron-based job scheduling.
type Scheduler struct {
	cron    *cron.Cron
	store   *admin.ScheduleStore
	entries map[string]cron.EntryID // schedule ID -> cron entry ID
	mu      sync.Mutex

	// Script execution
	sandbox        *starlark.Sandbox
	loader         *starlark.Loader
	secretProvider *starlark.SecretProvider
	scriptStore    *admin.ScriptStore

	// Agent execution (set via setter)
	engineAPI EngineAPI

	// Output delivery (set via setter)
	chatAPI ChatAPI
}

// Config holds scheduler configuration.
type Config struct {
	ScriptsDir     string
	MaxExecutionMs int64
	Secrets        map[string]string
	ScriptStore    *admin.ScriptStore
}

// New creates a new Scheduler.
func New(store *admin.ScheduleStore, cfg Config) *Scheduler {
	sandbox := starlark.New(starlark.Config{
		MaxExecutionMs: cfg.MaxExecutionMs,
	})
	loader := starlark.NewLoader(cfg.ScriptsDir, sandbox)

	secretProvider := starlark.NewSecretProvider()
	for name, value := range cfg.Secrets {
		secretProvider.Set(name, value)
	}
	sandbox.InjectSecrets(secretProvider)

	return &Scheduler{
		cron:           cron.New(),
		store:          store,
		entries:        make(map[string]cron.EntryID),
		sandbox:        sandbox,
		loader:         loader,
		secretProvider: secretProvider,
		scriptStore:    cfg.ScriptStore,
	}
}

// SetEngineAPI wires the engine for agent-type jobs.
func (s *Scheduler) SetEngineAPI(api EngineAPI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.engineAPI = api
}

// SetChatAPI wires the chat provider for output delivery.
func (s *Scheduler) SetChatAPI(api ChatAPI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chatAPI = api
}

// Start loads all enabled schedules and starts the cron runner.
func (s *Scheduler) Start(ctx context.Context) error {
	schedules, err := s.store.List()
	if err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}
		if err := s.addCronEntry(sched); err != nil {
			log.Printf("[scheduler] Failed to register schedule %q (%s): %v", sched.Name, sched.ID, err)
		}
	}

	s.cron.Start()
	log.Printf("[scheduler] Started with %d enabled schedules", len(s.entries))
	return nil
}

// Stop halts the cron runner.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("[scheduler] Stopped")
}

// Reload re-reads all schedules from the store and updates the cron entries.
func (s *Scheduler) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove all existing entries
	for id, entryID := range s.entries {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}

	// Re-register enabled schedules
	schedules, err := s.store.List()
	if err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}
		if err := s.addCronEntryLocked(sched); err != nil {
			log.Printf("[scheduler] Failed to register schedule %q (%s): %v", sched.Name, sched.ID, err)
		}
	}

	log.Printf("[scheduler] Reloaded: %d enabled schedules", len(s.entries))
	return nil
}

// RunNow triggers a schedule immediately in a background goroutine.
func (s *Scheduler) RunNow(id string) error {
	sched, err := s.store.Get(id)
	if err != nil {
		return err
	}

	go s.executeJob(sched)
	return nil
}

// addCronEntry registers a schedule with the cron runner (acquires lock).
func (s *Scheduler) addCronEntry(sched *admin.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addCronEntryLocked(sched)
}

// addCronEntryLocked registers a schedule with the cron runner.
// Caller must hold s.mu.
func (s *Scheduler) addCronEntryLocked(sched *admin.Schedule) error {
	// Remove existing entry if present
	if entryID, ok := s.entries[sched.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, sched.ID)
	}

	schedCopy := *sched
	entryID, err := s.cron.AddFunc(sched.CronExpr, func() {
		s.executeJob(&schedCopy)
	})
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", sched.CronExpr, err)
	}

	s.entries[sched.ID] = entryID
	log.Printf("[scheduler] Registered %q (%s) with cron %q", sched.Name, sched.ID, sched.CronExpr)
	return nil
}

// executeJob runs a single scheduled job with panic recovery.
func (s *Scheduler) executeJob(sched *admin.Schedule) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduler] Panic in job %q (%s): %v", sched.Name, sched.ID, r)
			s.store.UpdateLastRun(sched.ID, "error", fmt.Sprintf("panic: %v", r), "")
		}
	}()

	log.Printf("[scheduler] Executing job %q (%s) type=%s", sched.Name, sched.ID, sched.Type)

	var output string
	var execErr error

	switch sched.Type {
	case "script":
		output, execErr = s.executeScript(sched)
	case "agent":
		output, execErr = s.executeAgent(sched)
	default:
		execErr = fmt.Errorf("unknown job type: %s", sched.Type)
	}

	status := "success"
	errMsg := ""
	if execErr != nil {
		status = "error"
		errMsg = execErr.Error()
		log.Printf("[scheduler] Job %q (%s) failed: %v", sched.Name, sched.ID, execErr)
	} else {
		log.Printf("[scheduler] Job %q (%s) completed successfully", sched.Name, sched.ID)
	}

	if err := s.store.UpdateLastRun(sched.ID, status, errMsg, output); err != nil {
		log.Printf("[scheduler] Failed to update last run for %q: %v", sched.Name, err)
	}

	// Auto-disable run-once schedules after execution.
	// Re-read from store to get the current state (not the cached copy).
	if current, err := s.store.Get(sched.ID); err == nil && current.RunOnce {
		log.Printf("[scheduler] Run-once job %q (%s) completed, auto-disabling", sched.Name, sched.ID)
		if err := s.store.SetEnabled(sched.ID, false); err != nil {
			log.Printf("[scheduler] Failed to auto-disable run-once job %q: %v", sched.Name, err)
		} else {
			s.Reload()
		}
	}

	// Send output to target if configured and there's output
	if sched.OutputTarget != nil && output != "" {
		s.sendOutput(sched, output, execErr)
	}
}

// executeScript runs a Starlark script.
func (s *Scheduler) executeScript(sched *admin.Schedule) (string, error) {
	scriptName := sched.ScriptName

	// Check approval (uses full filename with .star extension)
	if s.scriptStore != nil {
		approvalName := scriptName
		if !strings.HasSuffix(approvalName, ".star") {
			approvalName += ".star"
		}
		if err := s.scriptStore.CanExecute(approvalName); err != nil {
			return "", fmt.Errorf("script not approved: %w", err)
		}
	}

	// loader.Load expects name without .star extension
	loadName := scriptName
	if strings.HasSuffix(loadName, ".star") {
		loadName = loadName[:len(loadName)-5]
	}

	// Load script
	script, err := s.loader.Load(loadName)
	if err != nil {
		return "", fmt.Errorf("failed to load script %q: %w", sched.ScriptName, err)
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result := s.sandbox.Execute(ctx, script.Name, script.Source)

	// Sanitize output
	result = starlark.SanitizeResult(result, s.secretProvider)

	if result.Error != "" {
		return "", fmt.Errorf("script error: %s", result.Error)
	}

	return fmt.Sprintf("%v", result.Value), nil
}

// executeAgent creates a new AI session and sends the prompt.
func (s *Scheduler) executeAgent(sched *admin.Schedule) (string, error) {
	s.mu.Lock()
	eng := s.engineAPI
	s.mu.Unlock()

	if eng == nil {
		return "", fmt.Errorf("engine API not available")
	}

	session, err := eng.CreateSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	messages := []engine.Message{
		{Role: "user", Content: sched.Prompt},
	}

	responses, err := eng.Send(ctx, session.ID, messages)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	// Drain response channel and collect text
	var textParts []string
	for resp := range responses {
		if resp.Content != "" {
			textParts = append(textParts, resp.Content)
		}
	}

	// The last text part should contain the full response (SSE streaming sends full text)
	if len(textParts) > 0 {
		return textParts[len(textParts)-1], nil
	}

	return "", nil
}

// sendOutput delivers job output to the configured chat channel.
func (s *Scheduler) sendOutput(sched *admin.Schedule, output string, execErr error) {
	s.mu.Lock()
	chat := s.chatAPI
	s.mu.Unlock()

	if chat == nil {
		log.Printf("[scheduler] Chat API not available, cannot deliver output for %q", sched.Name)
		return
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("**Scheduled Job: %s**\n", sched.Name))
	if execErr != nil {
		msg.WriteString(fmt.Sprintf("Status: error\nError: %s\n", execErr.Error()))
	} else {
		msg.WriteString("Status: success\n")
	}
	if output != "" {
		// Truncate for chat delivery
		if len(output) > 1800 {
			output = output[:1800] + "... (truncated)"
		}
		msg.WriteString(fmt.Sprintf("Output:\n```\n%s\n```", output))
	}

	if err := chat.SendViaProvider(sched.OutputTarget.Provider, sched.OutputTarget.ChannelID, msg.String()); err != nil {
		log.Printf("[scheduler] Failed to deliver output for %q: %v", sched.Name, err)
	}
}

// Store returns the underlying schedule store.
func (s *Scheduler) Store() *admin.ScheduleStore {
	return s.store
}
