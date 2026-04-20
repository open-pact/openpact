package migrate

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// openMem returns a fresh in-memory DB for migrator tests. A direct
// sql.Open is used (not the storage package helper) so migrate can
// be unit-tested without importing storage — which would be a cycle.
func openMem(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRun_FreshDB_AppliesAllMigrations(t *testing.T) {
	db := openMem(t)

	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var v int
	if err := db.QueryRow(`SELECT version FROM op_schema_version`).Scan(&v); err != nil {
		t.Fatalf("read op_schema_version: %v", err)
	}
	if v != LatestVersion {
		t.Errorf("version = %d, want %d", v, LatestVersion)
	}
}

func TestRun_Idempotent(t *testing.T) {
	db := openMem(t)

	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	// Exactly one row in op_schema_version, at LatestVersion.
	var count, v int
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(version), 0) FROM op_schema_version`).Scan(&count, &v); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 || v != LatestVersion {
		t.Errorf("count=%d version=%d, want 1/%d", count, v, LatestVersion)
	}
}

func TestRun_RefusesNewerSchema(t *testing.T) {
	db := openMem(t)
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Spoof a version we don't know about.
	if _, err := db.Exec(`UPDATE op_schema_version SET version = ?`, LatestVersion+1); err != nil {
		t.Fatalf("spoof version: %v", err)
	}

	if err := Run(context.Background(), db); err == nil {
		t.Error("expected error on newer schema, got nil")
	}
}

func TestRun_CreatesEveryExpectedTable(t *testing.T) {
	db := openMem(t)
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{
		"op_schema_version",
		"op_users",
		"op_approvals",
		"op_secrets",
		"op_chat_providers",
		"op_schedules",
		"op_kv",
		"op_calendars",
		"op_channel_sessions",
		"op_channel_modes",
	}

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'op_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[n] = true
	}

	for _, name := range want {
		if !got[name] {
			t.Errorf("missing table: %s", name)
		}
	}
}
