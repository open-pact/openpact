// Package storage owns the shared SQLite connection that backs every
// piece of OpenPact runtime state (users, secrets, schedules, channel
// state, calendars, integrations, advanced settings). The same file
// also hosts stackllm's session tables; stackllm reserves the
// `stackllm_` prefix and OpenPact reserves `op_`.
//
// The single *sql.DB is owned at the top of the process (cmd/openpact
// main) and passed into every store and into the engine.Stack. Stores
// are package-per-domain under internal/storage/<domain>/ and never
// open their own connection.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at path, applying the
// same DSN-level pragmas stackllm uses — so either package can be the
// "owner" of the file without surprises. The parent directory is
// created 0o700 on demand.
//
// The returned *sql.DB is the single pool for the whole process: pass
// it into session.NewSQLiteStore to get stackllm's session surface,
// and into each internal/storage/<domain>.NewStore for OpenPact's.
//
// Callers that need an in-memory database for tests should use
// NewTestDB instead — it wires the same pragmas for a ":memory:" DSN.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("storage: Open requires a path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("storage: create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		return nil, fmt.Errorf("storage: open sqlite: %w", err)
	}
	// A single-connection pool prevents the "two transactions racing
	// for the same WAL slot" problem modernc.org/sqlite is prone to
	// under concurrent writers; this DB backs admin writes and
	// stackllm session saves on the same file, and serialization at
	// the Go layer matches SQLite's own write serialization.
	db.SetMaxOpenConns(1)
	return db, nil
}

// buildDSN mirrors stackllm's own DSN so every connection in the pool
// gets the same pragmas. foreign_keys is on, WAL journaling for
// concurrency, NORMAL sync for durable-but-fast, 5s busy_timeout.
func buildDSN(path string) string {
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "journal_mode(wal)")
	params.Add("_pragma", "synchronous(normal)")
	params.Add("_pragma", "busy_timeout(5000)")
	return "file:" + path + "?" + params.Encode()
}

// EnsureFilePerms chmods the SQLite file (and its WAL/SHM sidecars if
// present) to 0o600. SQLite creates the database file 0o644 by
// default; we want 0o600 because it holds session history, user rows,
// encrypted secrets, and provider tokens. Idempotent — safe to call
// on every boot.
func EnsureFilePerms(path string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		p := path + suffix
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := os.Chmod(p, 0o600); err != nil {
			return fmt.Errorf("storage: chmod %s: %w", p, err)
		}
	}
	return nil
}
