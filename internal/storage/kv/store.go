// Package kv provides scoped scalar settings persistence backed by
// the op_kv table in the shared OpenPact SQLite database.
//
// "Scoped" means every row belongs to a named scope (e.g.
// "advanced_settings", "integrations", "setup_state"); within a scope
// keys are dotted paths (e.g. "logging.level", "vault.path"). Values
// are always TEXT — booleans encoded as "true"/"false", integers as
// base-10 strings, floats as %g. That makes the table trivially
// inspectable from sqlite3 CLI.
//
// Lists of records (like calendar feeds) do NOT live here; they get
// their own tables (see internal/storage/calendars). kv is for
// scalar settings only.
//
// Typed helpers (AdvancedSettings / Integrations.Vault / SetupState)
// pack and unpack these rows into the Go shape the admin handlers
// already consume, so the handlers don't ad-hoc-decode strings.
package kv

import (
	"context"
	"database/sql"
	"fmt"
)

// Scope names in use today. Declared as constants so a typo in one
// callsite doesn't silently orphan rows in a new scope.
const (
	ScopeAdvancedSettings = "advanced_settings"
	ScopeIntegrations     = "integrations"
	ScopeSetupState       = "setup_state"
)

// Store is a thin handle around the shared DB. Every method is
// context-aware; the DB itself is caller-owned.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store that reads/writes op_kv on the given DB.
// The caller is expected to have already run op_* migrations.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Get returns the value at (scope, key). Returns ("", false, nil) if
// the row does not exist — that's not an error, it's "caller should
// apply a default." Genuine driver errors come through err.
func (s *Store) Get(ctx context.Context, scope, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM op_kv WHERE scope = ? AND key = ?`, scope, key,
	).Scan(&v)
	switch {
	case err == sql.ErrNoRows:
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("kv: get %s/%s: %w", scope, key, err)
	}
	return v, true, nil
}

// GetAll returns every row in the given scope as a key→value map.
// An empty map (not nil) is returned when the scope has no rows,
// so callers can range over the result without a nil check.
func (s *Store) GetAll(ctx context.Context, scope string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM op_kv WHERE scope = ?`, scope,
	)
	if err != nil {
		return nil, fmt.Errorf("kv: getall %s: %w", scope, err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("kv: scan %s: %w", scope, err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// Set upserts a single row.
func (s *Store) Set(ctx context.Context, scope, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO op_kv (scope, key, value) VALUES (?, ?, ?)
		ON CONFLICT(scope, key) DO UPDATE SET value = excluded.value
	`, scope, key, value)
	if err != nil {
		return fmt.Errorf("kv: set %s/%s: %w", scope, key, err)
	}
	return nil
}

// ReplaceScope atomically rewrites every row in scope to match the
// given map. Rows absent from the map are deleted. Matches the
// "PUT /api/config/advanced" semantics where the whole struct is
// the authoritative payload. Runs in a transaction so a partial
// failure leaves the DB unchanged.
func (s *Store) ReplaceScope(ctx context.Context, scope string, pairs map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("kv: replace %s begin: %w", scope, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM op_kv WHERE scope = ?`, scope); err != nil {
		return fmt.Errorf("kv: replace %s delete: %w", scope, err)
	}
	for k, v := range pairs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO op_kv (scope, key, value) VALUES (?, ?, ?)`,
			scope, k, v,
		); err != nil {
			return fmt.Errorf("kv: replace %s insert %s: %w", scope, k, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("kv: replace %s commit: %w", scope, err)
	}
	return nil
}

// Delete removes a single row. Absent rows are not an error.
func (s *Store) Delete(ctx context.Context, scope, key string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM op_kv WHERE scope = ? AND key = ?`, scope, key,
	)
	if err != nil {
		return fmt.Errorf("kv: delete %s/%s: %w", scope, key, err)
	}
	return nil
}
