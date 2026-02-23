package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrPasswordTooWeak  = errors.New("password does not meet requirements")
	ErrPasswordMismatch = errors.New("passwords do not match")
)

// User represents an admin user.
type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
	LastLoginAt  time.Time `json:"last_login_at,omitempty"`
}

// UserStore manages admin users with file-based persistence.
type UserStore struct {
	mu       sync.RWMutex
	users    map[string]*User
	filePath string
}

// NewUserStore creates a new user store backed by a JSON file.
func NewUserStore(dataDir string) (*UserStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	store := &UserStore{
		users:    make(map[string]*User),
		filePath: filepath.Join(dataDir, "users.json"),
	}

	// Load existing users if file exists
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load users: %w", err)
	}

	return store, nil
}

// load reads users from the JSON file.
func (s *UserStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var users []*User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.users = make(map[string]*User)
	for _, u := range users {
		s.users[u.Username] = u
	}

	return nil
}

// save writes users to the JSON file.
// MUST be called with s.mu already held (read or write lock).
func (s *UserStore) saveLocked() error {
	users := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0600)
}

// Create creates a new user with the given username and password.
func (s *UserStore) Create(username, password string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[username]; exists {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User{
		Username:     username,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}

	s.users[username] = user

	if err := s.saveLocked(); err != nil {
		delete(s.users, username)
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	return user, nil
}

// Get retrieves a user by username.
func (s *UserStore) Get(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}

	return user, nil
}

// Validate checks the username and password, returning the user if valid.
func (s *UserStore) Validate(username, password string) (*User, error) {
	s.mu.RLock()
	user, exists := s.users[username]
	s.mu.RUnlock()

	if !exists {
		// Still do a bcrypt compare to prevent timing attacks
		bcrypt.CompareHashAndPassword([]byte("$2a$10$dummy"), []byte(password))
		return nil, ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	// Update last login time
	s.mu.Lock()
	user.LastLoginAt = time.Now()
	s.saveLocked() // Non-critical, ignore error
	s.mu.Unlock()

	return user, nil
}

// Count returns the number of users.
func (s *UserStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

// HasUsers returns true if at least one user exists.
func (s *UserStore) HasUsers() bool {
	return s.Count() > 0
}

// ValidatePassword checks if a password meets the security requirements.
// Requirements:
// - Option 1: 16+ characters (passphrase style)
// - Option 2: 12+ characters with at least 3 of: uppercase, lowercase, number, symbol
func ValidatePassword(password string) error {
	// Option 1: Long password (easy to remember passphrase)
	if len(password) >= 16 {
		return nil
	}

	// Option 2: Complex but shorter (12+ chars)
	if len(password) >= 12 {
		hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
		hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
		hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
		hasSymbol := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{}|;':",.<>?/~` + "`" + `]`).MatchString(password)

		complexity := 0
		if hasUpper {
			complexity++
		}
		if hasLower {
			complexity++
		}
		if hasNumber {
			complexity++
		}
		if hasSymbol {
			complexity++
		}

		if complexity >= 3 {
			return nil
		}
		return fmt.Errorf("%w: must contain at least 3 of: uppercase, lowercase, number, symbol", ErrPasswordTooWeak)
	}

	return fmt.Errorf("%w: must be 16+ characters, or 12+ with mixed character types", ErrPasswordTooWeak)
}

// ValidatePasswords validates that password meets requirements and matches confirmation.
func ValidatePasswords(password, confirmPassword string) error {
	if password != confirmPassword {
		return ErrPasswordMismatch
	}
	return ValidatePassword(password)
}
