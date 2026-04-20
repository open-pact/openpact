package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

// Errors. Kept compatible with the old admin-package errors so the
// handler's errors.Is() checks keep working after the swap.
var (
	ErrSecretNotFound = errors.New("secret not found")
	ErrSecretExists   = errors.New("secret already exists")
	ErrInvalidName    = errors.New("invalid secret name")
	ErrInvalidValue   = errors.New("invalid secret value")
)

var (
	secretNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	maxSecretNameLen  = 64
	maxSecretValueLen = 4096
)

// Entry is a secret's metadata — never the value. Matches the wire
// shape the admin /api/secrets endpoint has always returned.
type Entry struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store manages encrypted secrets in op_secrets. Values are stored
// as base64(nonce(12) || ciphertext) in a TEXT column so sqlite3
// dumps and copy-paste work without hex gymnastics.
type Store struct {
	db     *sql.DB
	aesgcm cipher.AEAD
}

// NewStore returns a Store that encrypts/decrypts values with the
// given 32-byte key. Returns an error if the key length is wrong or
// AES init fails.
func NewStore(db *sql.DB, key []byte) (*Store, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("secrets: key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: gcm: %w", err)
	}
	return &Store{db: db, aesgcm: gcm}, nil
}

// List returns metadata for every secret, sorted by name.
func (s *Store) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, created_at, updated_at FROM op_secrets`,
	)
	if err != nil {
		return nil, fmt.Errorf("secrets: list: %w", err)
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		var createdAt, updatedAt string
		if err := rows.Scan(&e.Name, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		e.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, rows.Err()
}

// Get returns the decrypted value for a secret, or ErrSecretNotFound.
func (s *Store) Get(ctx context.Context, name string) (string, error) {
	var cipherB64 string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM op_secrets WHERE name = ?`, name,
	).Scan(&cipherB64)
	switch {
	case err == sql.ErrNoRows:
		return "", ErrSecretNotFound
	case err != nil:
		return "", fmt.Errorf("secrets: get %s: %w", name, err)
	}
	return s.decrypt(cipherB64)
}

// All returns a name→plaintext map. Used by the scheduler + MCP
// script tool to hydrate the Starlark secret provider on boot.
func (s *Store) All(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, value FROM op_secrets`)
	if err != nil {
		return nil, fmt.Errorf("secrets: all: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var name, cipherB64 string
		if err := rows.Scan(&name, &cipherB64); err != nil {
			return nil, err
		}
		plain, err := s.decrypt(cipherB64)
		if err != nil {
			return nil, fmt.Errorf("secrets: decrypt %s: %w", name, err)
		}
		out[name] = plain
	}
	return out, rows.Err()
}

// Set creates or updates a secret. Validates name + value before
// writing. Re-encrypts the value (fresh nonce) on every call.
func (s *Store) Set(ctx context.Context, name, value string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateValue(value); err != nil {
		return err
	}
	cipherB64, err := s.encrypt(value)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO op_secrets (name, value, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
		    value = excluded.value,
		    updated_at = excluded.updated_at
	`, name, cipherB64, now, now)
	if err != nil {
		return fmt.Errorf("secrets: set %s: %w", name, err)
	}
	return nil
}

// Create adds a new secret. Returns ErrSecretExists if the name is
// already taken — differs from Set, which upserts.
func (s *Store) Create(ctx context.Context, name, value string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateValue(value); err != nil {
		return err
	}

	// Check existence in a single round-trip via insert-only.
	cipherB64, err := s.encrypt(value)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO op_secrets (name, value, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		name, cipherB64, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrSecretExists
		}
		return fmt.Errorf("secrets: create %s: %w", name, err)
	}
	return nil
}

// Update mutates an existing secret. Returns ErrSecretNotFound if
// the name isn't present.
func (s *Store) Update(ctx context.Context, name, value string) error {
	if err := validateValue(value); err != nil {
		return err
	}
	cipherB64, err := s.encrypt(value)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		`UPDATE op_secrets SET value = ?, updated_at = ? WHERE name = ?`,
		cipherB64, now, name,
	)
	if err != nil {
		return fmt.Errorf("secrets: update %s: %w", name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSecretNotFound
	}
	return nil
}

// Delete removes a secret by name.
func (s *Store) Delete(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM op_secrets WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("secrets: delete %s: %w", name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSecretNotFound
	}
	return nil
}

// --- crypto helpers ------------------------------------------------------

func (s *Store) encrypt(plain string) (string, error) {
	nonce := make([]byte, s.aesgcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secrets: nonce: %w", err)
	}
	ct := s.aesgcm.Seal(nil, nonce, []byte(plain), nil)
	out := make([]byte, 0, len(nonce)+len(ct))
	out = append(out, nonce...)
	out = append(out, ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *Store) decrypt(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("secrets: decode: %w", err)
	}
	if len(raw) < s.aesgcm.NonceSize() {
		return "", fmt.Errorf("secrets: ciphertext too short")
	}
	nonce := raw[:s.aesgcm.NonceSize()]
	ct := raw[s.aesgcm.NonceSize():]
	plain, err := s.aesgcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("secrets: decrypt: %w", err)
	}
	return string(plain), nil
}

func validateName(name string) error {
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

func validateValue(value string) error {
	if value == "" {
		return fmt.Errorf("%w: value cannot be empty", ErrInvalidValue)
	}
	if len(value) > maxSecretValueLen {
		return fmt.Errorf("%w: value exceeds %d characters", ErrInvalidValue, maxSecretValueLen)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	return err != nil && (stringsContains(err.Error(), "UNIQUE constraint failed") ||
		stringsContains(err.Error(), "2067"))
}

func stringsContains(s, needle string) bool {
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
