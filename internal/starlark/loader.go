package starlark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Script represents a loaded Starlark script
type Script struct {
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Source      string            `json:"-"`
	Description string            `json:"description,omitempty"`
	Functions   []string          `json:"functions,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Loader loads and caches Starlark scripts from a directory
type Loader struct {
	mu       sync.RWMutex
	baseDir  string
	scripts  map[string]*Script
	sandbox  *Sandbox
}

// NewLoader creates a script loader for the given directory
func NewLoader(baseDir string, sandbox *Sandbox) *Loader {
	return &Loader{
		baseDir: baseDir,
		scripts: make(map[string]*Script),
		sandbox: sandbox,
	}
}

// Load loads a script by name (without .star extension)
func (l *Loader) Load(name string) (*Script, error) {
	l.mu.RLock()
	if script, ok := l.scripts[name]; ok {
		l.mu.RUnlock()
		return script, nil
	}
	l.mu.RUnlock()

	// Try to load from filesystem
	path := filepath.Join(l.baseDir, name+".star")
	return l.LoadFile(path)
}

// LoadFile loads a script from a specific file path
func (l *Loader) LoadFile(path string) (*Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read script: %w", err)
	}

	name := strings.TrimSuffix(filepath.Base(path), ".star")
	source := string(data)

	script := &Script{
		Name:     name,
		Path:     path,
		Source:   source,
		Metadata: make(map[string]string),
	}

	// Parse metadata from comments at the top of the file
	script.parseMetadata()

	// Cache the script
	l.mu.Lock()
	l.scripts[name] = script
	l.mu.Unlock()

	return script, nil
}

// parseMetadata extracts metadata from script comments
// Format: # @key: value
func (s *Script) parseMetadata() {
	lines := strings.Split(s.Source, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			break // Stop at first non-comment line
		}

		line = strings.TrimPrefix(line, "#")
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "@") {
			parts := strings.SplitN(line[1:], ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				s.Metadata[key] = value

				if key == "description" {
					s.Description = value
				}
			}
		}
	}
}

// List returns all available scripts in the directory
func (l *Loader) List() ([]*Script, error) {
	entries, err := os.ReadDir(l.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Script{}, nil
		}
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	var scripts []*Script
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".star") {
			continue
		}

		script, err := l.LoadFile(filepath.Join(l.baseDir, entry.Name()))
		if err != nil {
			continue // Skip scripts that fail to load
		}
		scripts = append(scripts, script)
	}

	return scripts, nil
}

// Reload clears the cache and reloads all scripts
func (l *Loader) Reload() error {
	l.mu.Lock()
	l.scripts = make(map[string]*Script)
	l.mu.Unlock()

	_, err := l.List()
	return err
}

// Get returns a cached script by name
func (l *Loader) Get(name string) (*Script, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	script, ok := l.scripts[name]
	return script, ok
}

// Count returns the number of loaded scripts
func (l *Loader) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.scripts)
}
