// Package migrate applies OpenPact schema migrations to a shared
// SQLite database. The pattern mirrors stackllm's own runMigrations:
// migrations are ordered string literals, each applied in its own
// transaction, and the current version is tracked in a dedicated
// table (op_schema_version) so re-running against an up-to-date DB
// is a no-op.
//
// OpenPact tables all use the op_ prefix; stackllm owns stackllm_*.
// The two migrators coexist on the same *sql.DB without seeing each
// other's version rows.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Run applies every pending migration. Safe to call on every boot —
// it reads op_schema_version and only executes versions above the
// stored one. Running against a DB whose recorded version is newer
// than LatestVersion is a hard error: downgrades would drop columns
// the deployed binary doesn't know exist.
func Run(ctx context.Context, db *sql.DB) error {
	current, err := readVersion(ctx, db)
	if err != nil {
		return err
	}

	if current > LatestVersion {
		return fmt.Errorf("storage: op_schema_version %d is newer than supported %d — downgrade not allowed", current, LatestVersion)
	}

	for v := current; v < LatestVersion; v++ {
		if err := applyMigration(ctx, db, v); err != nil {
			return err
		}
	}
	return nil
}

// readVersion returns the recorded op_schema_version. A missing
// table means we're at version 0 (fresh DB).
func readVersion(ctx context.Context, db *sql.DB) (int, error) {
	var current int
	err := db.QueryRowContext(ctx, `SELECT version FROM op_schema_version`).Scan(&current)
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil && strings.Contains(err.Error(), "no such table"):
		return 0, nil
	case err != nil:
		return 0, fmt.Errorf("storage: read op_schema_version: %w", err)
	}
	return current, nil
}

// applyMigration runs migrations[v] (which upgrades v → v+1) inside a
// single transaction, then upserts the version row. All-or-nothing:
// a mid-migration failure rolls the transaction back and the version
// row is left at v.
func applyMigration(ctx context.Context, db *sql.DB, v int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin migration %d: %w", v+1, err)
	}
	if _, err := tx.ExecContext(ctx, migrations[v]); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("storage: apply migration %d: %w", v+1, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM op_schema_version`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("storage: clear op_schema_version: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO op_schema_version(version) VALUES(?)`, v+1); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("storage: record op_schema_version %d: %w", v+1, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit migration %d: %w", v+1, err)
	}
	return nil
}
