package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	ErrScheduleNotFound = errors.New("schedule not found")
	ErrScheduleExists   = errors.New("schedule already exists")
	maxScheduleNameLen  = 128
	maxPromptLen        = 8192
	maxOutputLen        = 2000
)

// Schedule represents a scheduled job.
type Schedule struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	CronExpr      string        `json:"cron_expr"`
	Type          string        `json:"type"` // "script" or "agent"
	Enabled       bool          `json:"enabled"`
	ScriptName    string        `json:"script_name,omitempty"`
	Prompt        string        `json:"prompt,omitempty"`
	OutputTarget  *OutputTarget `json:"output_target,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	LastRunAt     *time.Time    `json:"last_run_at,omitempty"`
	LastRunStatus string        `json:"last_run_status,omitempty"`
	LastRunError  string        `json:"last_run_error,omitempty"`
	LastRunOutput string        `json:"last_run_output,omitempty"`
}

// OutputTarget specifies where to send job output.
type OutputTarget struct {
	Provider  string `json:"provider"`
	ChannelID string `json:"channel_id"`
}

// schedulesFile is the on-disk JSON format.
type schedulesFile struct {
	Schedules map[string]*Schedule `json:"schedules"`
}

// ScheduleStore manages schedule persistence.
type ScheduleStore struct {
	dataDir string
	mu      sync.RWMutex
}

// NewScheduleStore creates a new schedule store.
func NewScheduleStore(dataDir string) *ScheduleStore {
	return &ScheduleStore{dataDir: dataDir}
}

func (s *ScheduleStore) filePath() string {
	return filepath.Join(s.dataDir, "schedules.json")
}

func (s *ScheduleStore) load() (*schedulesFile, error) {
	sf := &schedulesFile{Schedules: make(map[string]*Schedule)}

	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return sf, nil
		}
		return nil, fmt.Errorf("failed to read schedules: %w", err)
	}

	if err := json.Unmarshal(data, sf); err != nil {
		return nil, fmt.Errorf("failed to parse schedules: %w", err)
	}

	if sf.Schedules == nil {
		sf.Schedules = make(map[string]*Schedule)
	}

	return sf, nil
}

func (s *ScheduleStore) save(sf *schedulesFile) error {
	if err := os.MkdirAll(s.dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	if err := os.WriteFile(s.filePath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write schedules: %w", err)
	}

	return nil
}

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// List returns all schedules sorted by name.
func (s *ScheduleStore) List() ([]*Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sf, err := s.load()
	if err != nil {
		return nil, err
	}

	schedules := make([]*Schedule, 0, len(sf.Schedules))
	for _, sched := range sf.Schedules {
		copy := *sched
		schedules = append(schedules, &copy)
	}

	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].Name < schedules[j].Name
	})

	return schedules, nil
}

// Get returns a single schedule by ID.
func (s *ScheduleStore) Get(id string) (*Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sf, err := s.load()
	if err != nil {
		return nil, err
	}

	sched, ok := sf.Schedules[id]
	if !ok {
		return nil, ErrScheduleNotFound
	}

	copy := *sched
	return &copy, nil
}

// Create adds a new schedule.
func (s *ScheduleStore) Create(sched *Schedule) (*Schedule, error) {
	if sched.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(sched.Name) > maxScheduleNameLen {
		return nil, fmt.Errorf("name exceeds %d characters", maxScheduleNameLen)
	}
	if sched.CronExpr == "" {
		return nil, fmt.Errorf("cron_expr is required")
	}
	if sched.Type != "script" && sched.Type != "agent" {
		return nil, fmt.Errorf("type must be 'script' or 'agent'")
	}
	if sched.Type == "script" && sched.ScriptName == "" {
		return nil, fmt.Errorf("script_name is required for type 'script'")
	}
	if sched.Type == "agent" && sched.Prompt == "" {
		return nil, fmt.Errorf("prompt is required for type 'agent'")
	}
	if sched.Type == "agent" && len(sched.Prompt) > maxPromptLen {
		return nil, fmt.Errorf("prompt exceeds %d characters", maxPromptLen)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return nil, err
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}

	now := time.Now().UTC()
	newSched := &Schedule{
		ID:           id,
		Name:         sched.Name,
		CronExpr:     sched.CronExpr,
		Type:         sched.Type,
		Enabled:      sched.Enabled,
		ScriptName:   sched.ScriptName,
		Prompt:       sched.Prompt,
		OutputTarget: sched.OutputTarget,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	sf.Schedules[id] = newSched
	if err := s.save(sf); err != nil {
		return nil, err
	}

	copy := *newSched
	return &copy, nil
}

// Update modifies an existing schedule.
func (s *ScheduleStore) Update(id string, updates *Schedule) (*Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return nil, err
	}

	existing, ok := sf.Schedules[id]
	if !ok {
		return nil, ErrScheduleNotFound
	}

	if updates.Name != "" {
		if len(updates.Name) > maxScheduleNameLen {
			return nil, fmt.Errorf("name exceeds %d characters", maxScheduleNameLen)
		}
		existing.Name = updates.Name
	}
	if updates.CronExpr != "" {
		existing.CronExpr = updates.CronExpr
	}
	if updates.Type != "" {
		if updates.Type != "script" && updates.Type != "agent" {
			return nil, fmt.Errorf("type must be 'script' or 'agent'")
		}
		existing.Type = updates.Type
	}
	if updates.ScriptName != "" {
		existing.ScriptName = updates.ScriptName
	}
	if updates.Prompt != "" {
		if len(updates.Prompt) > maxPromptLen {
			return nil, fmt.Errorf("prompt exceeds %d characters", maxPromptLen)
		}
		existing.Prompt = updates.Prompt
	}
	// OutputTarget can be set to nil to clear it
	existing.OutputTarget = updates.OutputTarget
	existing.UpdatedAt = time.Now().UTC()

	sf.Schedules[id] = existing
	if err := s.save(sf); err != nil {
		return nil, err
	}

	copy := *existing
	return &copy, nil
}

// Delete removes a schedule.
func (s *ScheduleStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return err
	}

	if _, ok := sf.Schedules[id]; !ok {
		return ErrScheduleNotFound
	}

	delete(sf.Schedules, id)
	return s.save(sf)
}

// SetEnabled enables or disables a schedule.
func (s *ScheduleStore) SetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return err
	}

	sched, ok := sf.Schedules[id]
	if !ok {
		return ErrScheduleNotFound
	}

	sched.Enabled = enabled
	sched.UpdatedAt = time.Now().UTC()
	sf.Schedules[id] = sched

	return s.save(sf)
}

// UpdateLastRun records the result of a job execution.
func (s *ScheduleStore) UpdateLastRun(id, status, errMsg, output string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return err
	}

	sched, ok := sf.Schedules[id]
	if !ok {
		return ErrScheduleNotFound
	}

	now := time.Now().UTC()
	sched.LastRunAt = &now
	sched.LastRunStatus = status
	sched.LastRunError = errMsg
	if len(output) > maxOutputLen {
		output = output[:maxOutputLen]
	}
	sched.LastRunOutput = output

	sf.Schedules[id] = sched
	return s.save(sf)
}
