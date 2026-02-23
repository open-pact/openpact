package orchestrator

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-pact/openpact/internal/admin"
)

// seedContextTemplates copies default context templates into the workspace
// if they don't already exist. This ensures the AI has initial SOUL.md,
// USER.md, and MEMORY.md files to work with.
// Placeholder tokens ({{AGENT_NAME}}, etc.) are replaced with generic defaults.
func seedContextTemplates(workspacePath string) {
	templates := map[string]string{
		"SOUL.md":   admin.DefaultSoulTemplate,
		"USER.md":   admin.DefaultUserTemplate,
		"MEMORY.md": admin.DefaultMemoryTemplate,
	}

	// Replace placeholders with generic defaults for seeding
	replacer := strings.NewReplacer(
		"{{AGENT_NAME}}", "(your name)",
		"{{AGENT_VIBE}}", "(how you communicate)",
		"{{USER_NAME}}", "(their name)",
		"{{USER_TIMEZONE}}", "(e.g., Europe/London)",
	)

	for name, content := range templates {
		path := filepath.Join(workspacePath, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			resolved := replacer.Replace(content)
			if err := os.WriteFile(path, []byte(resolved), 0644); err != nil {
				log.Printf("Warning: failed to seed %s: %v", name, err)
			} else {
				log.Printf("Seeded %s from template", name)
			}
		}
	}
}
