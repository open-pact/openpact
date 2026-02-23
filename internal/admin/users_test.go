package admin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		// Long passwords (16+ chars) - should pass regardless of complexity
		{"long simple password", "thisisaverylongpassword", false},
		{"long with spaces", "correct horse battery", false}, // 21 chars with spaces

		// Complex passwords (12+ chars with 3/4 categories)
		{"12 chars with upper/lower/number", "Password1234", false},
		{"12 chars with upper/lower/symbol", "Password!@#$", false},
		{"12 chars with lower/number/symbol", "password123!", false},
		{"12 chars all categories", "Password123!", false},

		// Should fail - too short
		{"too short simple", "short", true},
		{"11 chars complex", "Password12!", true}, // Only 11 chars

		// Should fail - 12+ chars but only 2 categories
		{"12 chars only lower", "abcdefghijkl", true},
		{"12 chars lower/number only", "abcdefgh1234", true},
		{"12 chars upper/lower only", "AbCdEfGhIjKl", true},

		// Edge cases
		{"exactly 16 chars simple", "1234567890123456", false},
		{"exactly 12 chars 3 categories", "Abcdefgh123!", false},
		{"15 chars simple", "123456789012345", true}, // 15 chars, no complexity
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePasswords(t *testing.T) {
	t.Run("matching valid passwords", func(t *testing.T) {
		err := ValidatePasswords("thisisaverylongpassword", "thisisaverylongpassword")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("mismatched passwords", func(t *testing.T) {
		err := ValidatePasswords("password123456789", "different123456789")
		if err != ErrPasswordMismatch {
			t.Errorf("Expected ErrPasswordMismatch, got %v", err)
		}
	})

	t.Run("matching but weak passwords", func(t *testing.T) {
		err := ValidatePasswords("weak", "weak")
		if err == nil {
			t.Error("Expected error for weak password")
		}
	})
}

func TestUserStore_CreateAndValidate(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewUserStore(tmpDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}

	// Create a user
	user, err := store.Create("admin", "testpassword123!")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if user.Username != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", user.Username)
	}

	if user.PasswordHash == "testpassword123!" {
		t.Error("Password should be hashed, not stored in plain text")
	}

	// Validate correct password
	validatedUser, err := store.Validate("admin", "testpassword123!")
	if err != nil {
		t.Fatalf("Validate failed with correct password: %v", err)
	}

	if validatedUser.Username != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", validatedUser.Username)
	}

	// Validate wrong password
	_, err = store.Validate("admin", "wrongpassword")
	if err != ErrInvalidPassword {
		t.Errorf("Expected ErrInvalidPassword, got %v", err)
	}

	// Validate non-existent user
	_, err = store.Validate("nonexistent", "password")
	if err != ErrInvalidPassword {
		t.Errorf("Expected ErrInvalidPassword for non-existent user, got %v", err)
	}
}

func TestUserStore_DuplicateUser(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewUserStore(tmpDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}

	_, err = store.Create("admin", "password1234567890")
	if err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	_, err = store.Create("admin", "differentpassword12")
	if err != ErrUserExists {
		t.Errorf("Expected ErrUserExists, got %v", err)
	}
}

func TestUserStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and add user
	store1, err := NewUserStore(tmpDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}

	_, err = store1.Create("admin", "password1234567890")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create new store instance - should load existing users
	store2, err := NewUserStore(tmpDir)
	if err != nil {
		t.Fatalf("NewUserStore (2nd instance) failed: %v", err)
	}

	// Validate user still exists
	_, err = store2.Validate("admin", "password1234567890")
	if err != nil {
		t.Errorf("User should persist across store instances: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(filepath.Join(tmpDir, "users.json"))
	if err != nil {
		t.Fatalf("Failed to stat users.json: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestUserStore_Get(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewUserStore(tmpDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}

	_, err = store.Create("admin", "password1234567890")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	user, err := store.Get("admin")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if user.Username != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", user.Username)
	}

	_, err = store.Get("nonexistent")
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_Count(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewUserStore(tmpDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}

	if store.Count() != 0 {
		t.Errorf("Expected count 0, got %d", store.Count())
	}

	if store.HasUsers() {
		t.Error("Expected HasUsers to be false")
	}

	store.Create("user1", "password1234567890")

	if store.Count() != 1 {
		t.Errorf("Expected count 1, got %d", store.Count())
	}

	if !store.HasUsers() {
		t.Error("Expected HasUsers to be true")
	}

	store.Create("user2", "password0987654321")

	if store.Count() != 2 {
		t.Errorf("Expected count 2, got %d", store.Count())
	}
}
