// Package context handles loading and injecting workspace context files
// into the AI's system prompt (SOUL.md, USER.md, MEMORY.md, etc.)
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Loader loads context files from the workspace
type Loader struct {
	workspacePath string
}

// NewLoader creates a context loader for the given workspace
func NewLoader(workspacePath string) *Loader {
	return &Loader{workspacePath: workspacePath}
}

// Load reads all context files and returns the combined system prompt
func (l *Loader) Load() (string, error) {
	var parts []string

	// Load SOUL.md - core identity/personality
	soul, err := l.loadFile("SOUL.md")
	if err == nil && soul != "" {
		parts = append(parts, wrapSection("Identity", soul))
	}

	// Load USER.md - user preferences
	user, err := l.loadFile("USER.md")
	if err == nil && user != "" {
		parts = append(parts, wrapSection("User Profile", user))
	}

	// Load MEMORY.md - long-term memory
	memory, err := l.loadFile("MEMORY.md")
	if err == nil && memory != "" {
		parts = append(parts, wrapSection("Long-Term Memory", memory))
	}

	// Load today's daily memory
	today := time.Now().Format("2006-01-02")
	dailyPath := filepath.Join("memory", today+".md")
	daily, err := l.loadFile(dailyPath)
	if err == nil && daily != "" {
		parts = append(parts, wrapSection(fmt.Sprintf("Today's Memory (%s)", today), daily))
	}

	if len(parts) == 0 {
		return "", nil
	}

	return strings.Join(parts, "\n\n"), nil
}

// LoadFile loads a single context file by name
func (l *Loader) LoadFile(name string) (string, error) {
	return l.loadFile(name)
}

// loadFile reads a file from the workspace, returning empty string on not found
func (l *Loader) loadFile(name string) (string, error) {
	path := filepath.Join(l.workspacePath, name)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read %s: %w", name, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// wrapSection wraps content in a named section
func wrapSection(name, content string) string {
	return fmt.Sprintf("<%s>\n%s\n</%s>", toTagName(name), content, toTagName(name))
}

// toTagName converts a section name to a lowercase tag name
func toTagName(name string) string {
	// Remove special chars, lowercase, replace spaces with hyphens
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "(", "")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, "'", "")
	return name
}

// GetDailyMemoryPath returns the path for a given date's memory file
func (l *Loader) GetDailyMemoryPath(date string) string {
	return filepath.Join(l.workspacePath, "memory", date+".md")
}

// GetLongTermMemoryPath returns the path to MEMORY.md
func (l *Loader) GetLongTermMemoryPath() string {
	return filepath.Join(l.workspacePath, "MEMORY.md")
}

// ListDailyMemories returns available daily memory files
func (l *Loader) ListDailyMemories() ([]string, error) {
	memoryDir := filepath.Join(l.workspacePath, "memory")

	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list memory directory: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			date := strings.TrimSuffix(entry.Name(), ".md")
			dates = append(dates, date)
		}
	}

	return dates, nil
}
