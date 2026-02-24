package admin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ModelPreference is the persisted default model preference.
type ModelPreference struct {
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ModelPreferenceStore manages persistence of the default model preference.
type ModelPreferenceStore struct {
	dataDir string
	mu      sync.RWMutex
}

// NewModelPreferenceStore creates a new model preference store.
func NewModelPreferenceStore(dataDir string) *ModelPreferenceStore {
	return &ModelPreferenceStore{dataDir: dataDir}
}

func (s *ModelPreferenceStore) filePath() string {
	return filepath.Join(s.dataDir, "model_preference.json")
}

// Get returns the saved model preference, or nil if none has been saved.
func (s *ModelPreferenceStore) Get() (*ModelPreference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read model preference: %w", err)
	}

	var pref ModelPreference
	if err := json.Unmarshal(data, &pref); err != nil {
		return nil, fmt.Errorf("failed to parse model preference: %w", err)
	}

	return &pref, nil
}

// Set saves the default model preference.
func (s *ModelPreferenceStore) Set(provider, model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	pref := ModelPreference{
		Provider:  provider,
		Model:     model,
		UpdatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(pref, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal model preference: %w", err)
	}

	if err := os.WriteFile(s.filePath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write model preference: %w", err)
	}

	return nil
}
