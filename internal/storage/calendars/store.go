// Package calendars backs the calendar-feed list under the
// Integrations admin page. Feeds are an ordered list of (name, url)
// records, so they get their own table (op_calendars) rather than
// packing into op_kv — ordered lookups are a per-row operation and
// the admin UI needs to render them in the order the user added.
//
// The exposed surface is small: read the full list, replace the full
// list atomically. The admin UI always sends the whole array on
// save, so per-row add/remove would be dead code.
package calendars

import (
	"context"
	"database/sql"
	"fmt"
)

// Feed is one calendar feed record. JSON tags match the wire format
// the admin UI sends and receives.
type Feed struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Store wraps a *sql.DB and scopes every query to op_calendars.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store for the given shared DB.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// List returns every feed, ordered by position. Empty slice (not
// nil) is returned when there are none — so JSON marshal produces
// [] and the frontend doesn't have to special-case null.
func (s *Store) List(ctx context.Context) ([]Feed, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, url FROM op_calendars ORDER BY position, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("calendars: list: %w", err)
	}
	defer rows.Close()

	out := []Feed{}
	for rows.Next() {
		var f Feed
		if err := rows.Scan(&f.Name, &f.URL); err != nil {
			return nil, fmt.Errorf("calendars: scan: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Replace atomically swaps the entire list for the given slice. The
// admin UI's IntegrationsView sends the authoritative array on every
// save, and list lengths are tiny (a handful of feeds at most), so
// delete-then-insert is simpler than diff-and-patch and avoids
// tombstone edge cases when a feed is removed. Transactional — a
// mid-write failure leaves the previous list intact.
func (s *Store) Replace(ctx context.Context, list []Feed) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("calendars: replace begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM op_calendars`); err != nil {
		return fmt.Errorf("calendars: delete: %w", err)
	}
	for i, f := range list {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO op_calendars (position, name, url) VALUES (?, ?, ?)`,
			i, f.Name, f.URL,
		); err != nil {
			return fmt.Errorf("calendars: insert %s: %w", f.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("calendars: commit: %w", err)
	}
	return nil
}
