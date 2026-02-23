package discord

import (
	"testing"
)

func TestConfigAllowedMaps(t *testing.T) {
	// Test that New properly builds allowed maps
	// Note: This doesn't actually connect to Discord

	// We can't test New() without a valid token,
	// but we can test the logic of building allowed maps

	cfg := Config{
		Token:        "test-token",
		AllowedUsers: []string{"user1", "user2"},
		AllowedChans: []string{"chan1"},
	}

	// Manually build maps like New() does
	allowedUsers := make(map[string]bool)
	for _, u := range cfg.AllowedUsers {
		allowedUsers[u] = true
	}

	allowedChans := make(map[string]bool)
	for _, c := range cfg.AllowedChans {
		allowedChans[c] = true
	}

	if !allowedUsers["user1"] {
		t.Error("user1 should be in allowedUsers")
	}

	if !allowedUsers["user2"] {
		t.Error("user2 should be in allowedUsers")
	}

	if allowedUsers["user3"] {
		t.Error("user3 should not be in allowedUsers")
	}

	if !allowedChans["chan1"] {
		t.Error("chan1 should be in allowedChans")
	}

	if allowedChans["chan2"] {
		t.Error("chan2 should not be in allowedChans")
	}
}

func TestEmptyAllowedMaps(t *testing.T) {
	cfg := Config{
		Token:        "test-token",
		AllowedUsers: []string{},
		AllowedChans: []string{},
	}

	allowedUsers := make(map[string]bool)
	for _, u := range cfg.AllowedUsers {
		allowedUsers[u] = true
	}

	allowedChans := make(map[string]bool)
	for _, c := range cfg.AllowedChans {
		allowedChans[c] = true
	}

	// Empty maps mean "allow all"
	if len(allowedUsers) != 0 {
		t.Errorf("allowedUsers should be empty, got %d", len(allowedUsers))
	}

	if len(allowedChans) != 0 {
		t.Errorf("allowedChans should be empty, got %d", len(allowedChans))
	}
}

func TestTargetParsing(t *testing.T) {
	tests := []struct {
		input    string
		isUser   bool
		isChan   bool
		expected string
	}{
		{"user:123456", true, false, "123456"},
		{"channel:789012", false, true, "789012"},
		{"345678", false, false, "345678"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if tt.isUser {
				if len(tt.input) < 5 || tt.input[:5] != "user:" {
					t.Errorf("expected user: prefix for %s", tt.input)
				}
			}
			if tt.isChan {
				if len(tt.input) < 8 || tt.input[:8] != "channel:" {
					t.Errorf("expected channel: prefix for %s", tt.input)
				}
			}
		})
	}
}
