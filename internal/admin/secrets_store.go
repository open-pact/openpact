package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

var (
	ErrSecretNotFound   = errors.New("secret not found")
	ErrSecretExists     = errors.New("secret already exists")
	ErrInvalidName      = errors.New("invalid secret name")
	ErrInvalidValue     = errors.New("invalid secret value")
	secretNamePattern   = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	maxSecretNameLen    = 64
	maxSecretValueLen   = 4096
)

// SecretEntry represents a stored secret's metadata.
type SecretEntry struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// secretsFile is the on-disk JSON format.
type secretsFile struct {
	Secrets map[string]secretRecord `json:"secrets"`
}

type secretRecord struct {
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SecretStore manages secret persistence.
type SecretStore struct {
	dataDir string
	mu      sync.RWMutex
}

// NewSecretStore creates a new secret store.
func NewSecretStore(dataDir string) *SecretStore {
	return &SecretStore{dataDir: dataDir}
}

func (s *SecretStore) filePath() string {
	return filepath.Join(s.dataDir, "starlark_secrets.json")
}

func (s *SecretStore) load() (*secretsFile, error) {
	sf := &secretsFile{Secrets: make(map[string]secretRecord)}

	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return sf, nil
		}
		return nil, fmt.Errorf("failed to read secrets: %w", err)
	}

	if err := json.Unmarshal(data, sf); err != nil {
		return nil, fmt.Errorf("failed to parse secrets: %w", err)
	}

	if sf.Secrets == nil {
		sf.Secrets = make(map[string]secretRecord)
	}

	return sf, nil
}

func (s *SecretStore) save(sf *secretsFile) error {
	if err := os.MkdirAll(s.dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	if err := os.WriteFile(s.filePath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write secrets: %w", err)
	}

	return nil
}

func validateSecretName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidName)
	}
	if len(name) > maxSecretNameLen {
		return fmt.Errorf("%w: name exceeds %d characters", ErrInvalidName, maxSecretNameLen)
	}
	if !secretNamePattern.MatchString(name) {
		return fmt.Errorf("%w: must match ^[A-Z][A-Z0-9_]*$", ErrInvalidName)
	}
	return nil
}

func validateSecretValue(value string) error {
	if value == "" {
		return fmt.Errorf("%w: value cannot be empty", ErrInvalidValue)
	}
	if len(value) > maxSecretValueLen {
		return fmt.Errorf("%w: value exceeds %d characters", ErrInvalidValue, maxSecretValueLen)
	}
	return nil
}

// List returns secret metadata (names and timestamps), never values.
func (s *SecretStore) List() ([]SecretEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sf, err := s.load()
	if err != nil {
		return nil, err
	}

	entries := make([]SecretEntry, 0, len(sf.Secrets))
	for name, rec := range sf.Secrets {
		entries = append(entries, SecretEntry{
			Name:      name,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return entries, nil
}

// Get returns the value for a single secret.
func (s *SecretStore) Get(name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sf, err := s.load()
	if err != nil {
		return "", err
	}

	rec, ok := sf.Secrets[name]
	if !ok {
		return "", ErrSecretNotFound
	}

	return rec.Value, nil
}

// Set creates or updates a secret.
func (s *SecretStore) Set(name, value string) error {
	if err := validateSecretName(name); err != nil {
		return err
	}
	if err := validateSecretValue(value); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	rec, exists := sf.Secrets[name]
	if exists {
		rec.Value = value
		rec.UpdatedAt = now
	} else {
		rec = secretRecord{
			Value:     value,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	sf.Secrets[name] = rec

	return s.save(sf)
}

// Create adds a new secret. Returns ErrSecretExists if name is already taken.
func (s *SecretStore) Create(name, value string) error {
	if err := validateSecretName(name); err != nil {
		return err
	}
	if err := validateSecretValue(value); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return err
	}

	if _, exists := sf.Secrets[name]; exists {
		return ErrSecretExists
	}

	now := time.Now().UTC()
	sf.Secrets[name] = secretRecord{
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return s.save(sf)
}

// Update updates an existing secret's value. Returns ErrSecretNotFound if it doesn't exist.
func (s *SecretStore) Update(name, value string) error {
	if err := validateSecretValue(value); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return err
	}

	rec, exists := sf.Secrets[name]
	if !exists {
		return ErrSecretNotFound
	}

	rec.Value = value
	rec.UpdatedAt = time.Now().UTC()
	sf.Secrets[name] = rec

	return s.save(sf)
}

// Delete removes a secret.
func (s *SecretStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.load()
	if err != nil {
		return err
	}

	if _, exists := sf.Secrets[name]; !exists {
		return ErrSecretNotFound
	}

	delete(sf.Secrets, name)
	return s.save(sf)
}

// All returns all name→value pairs (for loading into SecretProvider).
func (s *SecretStore) All() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sf, err := s.load()
	if err != nil {
		return nil, err
	}

	secrets := make(map[string]string, len(sf.Secrets))
	for name, rec := range sf.Secrets {
		secrets[name] = rec.Value
	}

	return secrets, nil
}
