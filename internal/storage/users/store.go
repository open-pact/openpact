// Package users persists admin-UI user accounts in the op_users table
// of the shared SQLite database. Rows store the bcrypt hash of the
// password; plaintext never touches disk. Usernames are unique and
// used as the primary key.
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Errors returned by the store. Kept verbatim from the old JSON-backed
// UserStore so callers don't have to re-map.
var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrPasswordTooWeak  = errors.New("password does not meet requirements")
	ErrPasswordMismatch = errors.New("passwords do not match")
)

// User represents an admin account. Zero-valued LastLoginAt means
// "never logged in" — rendered as an empty string in JSON via the
// omitempty tag, matching the previous on-disk shape.
type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
	LastLoginAt  time.Time `json:"last_login_at,omitempty"`
}

// Store wraps the shared *sql.DB for op_users operations. Caller-owned
// DB — the store never closes it.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over the given DB. Migrations should have
// already been applied (op_users exists).
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Create adds a new user with the given username and password.
// Returns ErrUserExists if the username is taken.
func (s *Store) Create(ctx context.Context, username, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("users: hash password: %w", err)
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO op_users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, string(hash), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		// modernc.org/sqlite returns a unique-constraint violation as
		// "constraint failed: UNIQUE constraint failed" or similar —
		// collapse to ErrUserExists so callers don't have to sniff.
		if isUniqueViolation(err) {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("users: insert: %w", err)
	}

	return &User{Username: username, PasswordHash: string(hash), CreatedAt: now}, nil
}

// Get returns a user by username, or ErrUserNotFound.
func (s *Store) Get(ctx context.Context, username string) (*User, error) {
	return s.scanOne(ctx,
		`SELECT username, password_hash, created_at, last_login_at
		 FROM op_users WHERE username = ?`, username)
}

// Validate checks the username/password pair. On success, touches
// last_login_at. A miss-named user still burns a bcrypt compare so
// the caller can't time-discriminate non-existent vs. wrong-password.
func (s *Store) Validate(ctx context.Context, username, password string) (*User, error) {
	user, err := s.Get(ctx, username)
	if errors.Is(err, ErrUserNotFound) {
		// Still do a bcrypt compare to defeat username-existence timing.
		bcrypt.CompareHashAndPassword([]byte("$2a$10$dummy"), []byte(password))
		return nil, ErrInvalidPassword
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	now := time.Now().UTC()
	_, _ = s.db.ExecContext(ctx,
		`UPDATE op_users SET last_login_at = ? WHERE username = ?`,
		now.Format(time.RFC3339Nano), username,
	)
	user.LastLoginAt = now
	return user, nil
}

// Count returns the number of users.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM op_users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("users: count: %w", err)
	}
	return n, nil
}

// HasUsers is a convenience wrapper around Count. Silent on errors —
// the caller is expected to treat DB failures as "no users" which is
// the safer default (blocks the app on the setup screen).
func (s *Store) HasUsers(ctx context.Context) bool {
	n, err := s.Count(ctx)
	if err != nil {
		return false
	}
	return n > 0
}

// scanOne executes a single-row query and returns a *User.
func (s *Store) scanOne(ctx context.Context, query string, args ...interface{}) (*User, error) {
	var u User
	var createdAt string
	var lastLoginAt sql.NullString

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&u.Username, &u.PasswordHash, &createdAt, &lastLoginAt,
	)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("users: scan: %w", err)
	}

	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if lastLoginAt.Valid {
		u.LastLoginAt, _ = time.Parse(time.RFC3339Nano, lastLoginAt.String)
	}
	return &u, nil
}

// isUniqueViolation matches the driver's UNIQUE-constraint error
// message. Kept here (not in a shared helper) so the storage package
// doesn't need to export driver internals.
func isUniqueViolation(err error) bool {
	// modernc.org/sqlite: "constraint failed: UNIQUE constraint failed: op_users.username (2067)"
	// The "2067" is SQLITE_CONSTRAINT_UNIQUE; we just substring-match.
	return err != nil && (contains(err.Error(), "UNIQUE constraint failed") ||
		contains(err.Error(), "2067"))
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// --- Password-complexity helpers (package-level so callers that
//     don't need a Store can still use them) -----------------------------

// ValidatePassword checks if a password meets OpenPact's complexity
// requirements. Either 16+ characters (passphrase), or 12+ with 3 of
// upper/lower/number/symbol.
func ValidatePassword(password string) error {
	if len(password) >= 16 {
		return nil
	}
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

// ValidatePasswords validates password complexity AND that the two
// copies match.
func ValidatePasswords(password, confirmPassword string) error {
	if password != confirmPassword {
		return ErrPasswordMismatch
	}
	return ValidatePassword(password)
}
