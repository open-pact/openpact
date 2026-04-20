package storage

import (
	"context"
	"database/sql"
	"net/url"
	"testing"

	"github.com/open-pact/openpact/internal/storage/migrate"
)

// NewTestDB opens an in-memory SQLite DB with the same pragmas Open
// uses, runs op_* migrations, and registers t.Cleanup to close the
// handle. Intended for unit tests that used to tmpDir+*.json — swap
// the constructor for this and you get a clean, isolated DB per test.
//
// Each call produces an independent in-memory DB (via the
// `mode=memory&cache=shared` trick with a unique identifier is *not*
// used — we want per-test isolation, not sharing). The default
// ":memory:" DSN is already per-connection, and we pin MaxOpenConns
// to 1 so every statement hits the same connection.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()

	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "busy_timeout(5000)")
	// journal_mode=WAL is silently ignored by :memory: (SQLite forces
	// journal_mode=MEMORY there), so we just leave it off. synchronous
	// is irrelevant in-memory.

	db, err := sql.Open("sqlite", "file::memory:?"+params.Encode())
	if err != nil {
		t.Fatalf("storage: open in-memory sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)

	if err := migrate.Run(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("storage: run migrations: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })
	return db
}
