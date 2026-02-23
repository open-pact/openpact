package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrScriptNotFound    = errors.New("script not found")
	ErrScriptExists      = errors.New("script already exists")
	ErrScriptNotApproved = errors.New("script not approved")
	ErrScriptModified    = errors.New("script modified since approval")
)

// ScriptStatus represents the approval status of a script.
type ScriptStatus string

const (
	StatusPending  ScriptStatus = "pending"
	StatusApproved ScriptStatus = "approved"
	StatusRejected ScriptStatus = "rejected"
)

// Script represents a Starlark script with its metadata.
type Script struct {
	Name            string       `json:"name"`
	Source          string       `json:"source,omitempty"` // Only included when requested
	Hash            string       `json:"hash"`
	Status          ScriptStatus `json:"status"`
	Description     string       `json:"description,omitempty"`
	RequiredSecrets []string     `json:"required_secrets,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	ModifiedAt      time.Time    `json:"modified_at"`
	ApprovedAt      *time.Time   `json:"approved_at,omitempty"`
	ApprovedBy      string       `json:"approved_by,omitempty"`
	RejectedAt      *time.Time   `json:"rejected_at,omitempty"`
	RejectedBy      string       `json:"rejected_by,omitempty"`
	RejectReason    string       `json:"reject_reason,omitempty"`
}

// Approval represents the approval state of a script.
type Approval struct {
	Hash         string       `json:"hash"`
	Status       ScriptStatus `json:"status"`
	ApprovedAt   *time.Time   `json:"approved_at,omitempty"`
	ApprovedBy   string       `json:"approved_by,omitempty"`
	RejectedAt   *time.Time   `json:"rejected_at,omitempty"`
	RejectedBy   string       `json:"rejected_by,omitempty"`
	RejectReason string       `json:"reject_reason,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	ModifiedAt   time.Time    `json:"modified_at"`
}

// ScriptStore manages scripts and their approval states.
type ScriptStore struct {
	mu            sync.RWMutex
	scriptsDir    string
	approvalsFile string
	approvals     map[string]*Approval // script name -> approval
	allowlist     map[string]bool      // always-approved scripts
}

// NewScriptStore creates a new script store.
func NewScriptStore(scriptsDir, dataDir string, allowlist []string) (*ScriptStore, error) {
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create scripts directory: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	allowlistMap := make(map[string]bool)
	for _, name := range allowlist {
		allowlistMap[name] = true
	}

	store := &ScriptStore{
		scriptsDir:    scriptsDir,
		approvalsFile: filepath.Join(dataDir, "approvals.json"),
		approvals:     make(map[string]*Approval),
		allowlist:     allowlistMap,
	}

	if err := store.loadApprovals(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load approvals: %w", err)
	}

	return store, nil
}

func (s *ScriptStore) loadApprovals() error {
	data, err := os.ReadFile(s.approvalsFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.approvals)
}

func (s *ScriptStore) saveApprovals() error {
	data, err := json.MarshalIndent(s.approvals, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.approvalsFile, data, 0600)
}

// computeHash computes the SHA256 hash of the script content.
func computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(hash[:])
}

// parseScriptMetadata extracts metadata from script comments.
func parseScriptMetadata(source string) (description string, secrets []string) {
	lines := strings.Split(source, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "@description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "@description:"))
		} else if strings.HasPrefix(line, "@secrets:") {
			secretList := strings.TrimSpace(strings.TrimPrefix(line, "@secrets:"))
			for _, s := range strings.Split(secretList, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					secrets = append(secrets, s)
				}
			}
		}
	}
	return
}

// List returns all scripts.
func (s *ScriptStore) List() ([]*Script, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.scriptsDir)
	if err != nil {
		return nil, err
	}

	var scripts []*Script
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".star") {
			continue
		}

		script, err := s.getScript(entry.Name(), false)
		if err != nil {
			continue
		}
		scripts = append(scripts, script)
	}

	return scripts, nil
}

// Get returns a script by name.
func (s *ScriptStore) Get(name string, includeSource bool) (*Script, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getScript(name, includeSource)
}

func (s *ScriptStore) getScript(name string, includeSource bool) (*Script, error) {
	path := filepath.Join(s.scriptsDir, name)
	source, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrScriptNotFound
		}
		return nil, err
	}

	info, _ := os.Stat(path)
	hash := computeHash(string(source))
	description, secrets := parseScriptMetadata(string(source))

	script := &Script{
		Name:            name,
		Hash:            hash,
		Description:     description,
		RequiredSecrets: secrets,
		ModifiedAt:      info.ModTime(),
	}

	if includeSource {
		script.Source = string(source)
	}

	// Determine status
	if s.allowlist[name] {
		script.Status = StatusApproved
	} else if approval, ok := s.approvals[name]; ok {
		script.Status = approval.Status
		script.ApprovedAt = approval.ApprovedAt
		script.ApprovedBy = approval.ApprovedBy
		script.RejectedAt = approval.RejectedAt
		script.RejectedBy = approval.RejectedBy
		script.RejectReason = approval.RejectReason
		script.CreatedAt = approval.CreatedAt

		// If hash doesn't match approval, status is pending (modified)
		if approval.Status == StatusApproved && approval.Hash != hash {
			script.Status = StatusPending
		}
	} else {
		script.Status = StatusPending
	}

	return script, nil
}

// Create creates a new script.
func (s *ScriptStore) Create(name, source, createdBy string) (*Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !strings.HasSuffix(name, ".star") {
		name = name + ".star"
	}

	path := filepath.Join(s.scriptsDir, name)
	if _, err := os.Stat(path); err == nil {
		return nil, ErrScriptExists
	}

	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		return nil, err
	}

	now := time.Now()
	hash := computeHash(source)

	s.approvals[name] = &Approval{
		Hash:       hash,
		Status:     StatusPending,
		CreatedAt:  now,
		ModifiedAt: now,
	}

	if err := s.saveApprovals(); err != nil {
		os.Remove(path)
		delete(s.approvals, name)
		return nil, err
	}

	return s.getScript(name, false)
}

// Update updates an existing script.
func (s *ScriptStore) Update(name, source, updatedBy string) (*Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.scriptsDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrScriptNotFound
	}

	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		return nil, err
	}

	now := time.Now()
	hash := computeHash(source)

	// Reset to pending status when content changes
	if approval, ok := s.approvals[name]; ok {
		approval.Hash = hash
		approval.Status = StatusPending
		approval.ModifiedAt = now
		approval.ApprovedAt = nil
		approval.ApprovedBy = ""
	} else {
		s.approvals[name] = &Approval{
			Hash:       hash,
			Status:     StatusPending,
			CreatedAt:  now,
			ModifiedAt: now,
		}
	}

	if err := s.saveApprovals(); err != nil {
		return nil, err
	}

	return s.getScript(name, false)
}

// Delete removes a script.
func (s *ScriptStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.scriptsDir, name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrScriptNotFound
		}
		return err
	}

	delete(s.approvals, name)
	s.saveApprovals() // Ignore error, script is already deleted

	return nil
}

// Approve approves a script.
func (s *ScriptStore) Approve(name, approvedBy string) (*Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	script, err := s.getScript(name, false)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	if approval, ok := s.approvals[name]; ok {
		approval.Status = StatusApproved
		approval.Hash = script.Hash
		approval.ApprovedAt = &now
		approval.ApprovedBy = approvedBy
		approval.ModifiedAt = now
		approval.RejectedAt = nil
		approval.RejectedBy = ""
		approval.RejectReason = ""
	} else {
		s.approvals[name] = &Approval{
			Hash:       script.Hash,
			Status:     StatusApproved,
			ApprovedAt: &now,
			ApprovedBy: approvedBy,
			CreatedAt:  now,
			ModifiedAt: now,
		}
	}

	if err := s.saveApprovals(); err != nil {
		return nil, err
	}

	return s.getScript(name, false)
}

// Reject rejects a script.
func (s *ScriptStore) Reject(name, rejectedBy, reason string) (*Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	script, err := s.getScript(name, false)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	if approval, ok := s.approvals[name]; ok {
		approval.Status = StatusRejected
		approval.RejectedAt = &now
		approval.RejectedBy = rejectedBy
		approval.RejectReason = reason
		approval.ModifiedAt = now
	} else {
		s.approvals[name] = &Approval{
			Hash:         script.Hash,
			Status:       StatusRejected,
			RejectedAt:   &now,
			RejectedBy:   rejectedBy,
			RejectReason: reason,
			CreatedAt:    now,
			ModifiedAt:   now,
		}
	}

	if err := s.saveApprovals(); err != nil {
		return nil, err
	}

	return s.getScript(name, false)
}

// CanExecute checks if a script can be executed.
func (s *ScriptStore) CanExecute(name string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Allowlisted scripts can always execute
	if s.allowlist[name] {
		return nil
	}

	script, err := s.getScript(name, false)
	if err != nil {
		return err
	}

	if script.Status != StatusApproved {
		return ErrScriptNotApproved
	}

	// Check hash matches (script wasn't modified since approval)
	if approval, ok := s.approvals[name]; ok {
		if approval.Hash != script.Hash {
			return ErrScriptModified
		}
	}

	return nil
}

// IsAllowlisted returns true if the script is in the allowlist.
func (s *ScriptStore) IsAllowlisted(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allowlist[name]
}
