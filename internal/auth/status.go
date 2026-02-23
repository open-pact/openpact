package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AuthStatus represents the authentication state for an engine.
type AuthStatus struct {
	Authenticated bool       `json:"authenticated"`
	Method        string     `json:"method"`      // "oauth", "env", ""
	EngineType    string     `json:"engine_type"`  // "opencode"
	ExpiresAt     *string    `json:"expires_at"`   // nil if unknown
	Error         string     `json:"error"`        // any error message
}

// CheckAuth checks authentication for the given engine type.
func CheckAuth(engineType string) AuthStatus {
	switch engineType {
	case "opencode":
		return CheckOpenCodeAuth()
	default:
		return AuthStatus{
			EngineType: engineType,
			Error:      "unknown engine type",
		}
	}
}

// CheckOpenCodeAuth checks if OpenCode CLI has valid authentication.
// Checks in order:
//  1. ANTHROPIC_API_KEY / OPENAI_API_KEY / other provider env vars
//  2. ~/.local/share/opencode/auth.json file
func CheckOpenCodeAuth() AuthStatus {
	status := AuthStatus{EngineType: "opencode"}

	// Check provider env vars
	providerKeys := []string{
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"GOOGLE_API_KEY",
		"AZURE_OPENAI_API_KEY",
	}
	for _, key := range providerKeys {
		if v := os.Getenv(key); v != "" {
			status.Authenticated = true
			status.Method = "env"
			return status
		}
	}

	// Check ~/.local/share/opencode/auth.json
	home, err := os.UserHomeDir()
	if err != nil {
		status.Error = "No credentials found"
		return status
	}

	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	data, err := os.ReadFile(authPath)
	if err != nil {
		status.Error = "No credentials found"
		return status
	}

	// Just check that the file is valid JSON with some content
	var authData map[string]interface{}
	if err := json.Unmarshal(data, &authData); err != nil {
		status.Error = "Invalid auth file"
		return status
	}

	if len(authData) == 0 {
		status.Error = "Empty auth file"
		return status
	}

	status.Authenticated = true
	status.Method = "oauth"
	return status
}

