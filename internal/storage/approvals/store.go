// Package approvals persists script-approval metadata in the
// op_approvals table. The approval row tracks which hash was
// approved so a script that's been edited after approval
// automatically falls back to pending (via the hash-mismatch check
// in the scripts handler).
//
// Script source itself still lives as .star files on disk — see
// ai/specs/starklark-to-db.md for the planned migration that moves
// source into the DB with versioning and diffs.
package approvals

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Status mirrors the three approval states the admin UI surfaces.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// Approval is the persisted state for a single script name.
type Approval struct {
	ScriptName   string     `json:"script_name"`
	Hash         string     `json:"hash"`
	Status       Status     `json:"status"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	ApprovedBy   string     `json:"approved_by,omitempty"`
	RejectedAt   *time.Time `json:"rejected_at,omitempty"`
	RejectedBy   string     `json:"rejected_by,omitempty"`
	RejectReason string     `json:"reject_reason,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ModifiedAt   time.Time  `json:"modified_at"`
}

// Store is a thin handle around the shared DB for op_approvals.
type Store struct {
	db *sql.DB
}

// NewStore returns an approvals.Store over the given shared DB.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Get returns the approval row for a script name. If no row exists,
// returns (nil, nil) — callers treat that as "implicit pending."
func (s *Store) Get(ctx context.Context, name string) (*Approval, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT script_name, hash, status, approved_at, approved_by,
		       rejected_at, rejected_by, reject_reason, created_at, modified_at
		  FROM op_approvals WHERE script_name = ?`, name)
	return scanApproval(row)
}

// Upsert replaces the row for approval.ScriptName, or inserts a new
// one if none exists. Both CreatedAt and ModifiedAt are populated by
// the caller so tests with deterministic time can pin them.
func (s *Store) Upsert(ctx context.Context, a Approval) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO op_approvals (script_name, hash, status, approved_at, approved_by,
		                          rejected_at, rejected_by, reject_reason, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(script_name) DO UPDATE SET
		    hash = excluded.hash,
		    status = excluded.status,
		    approved_at = excluded.approved_at,
		    approved_by = excluded.approved_by,
		    rejected_at = excluded.rejected_at,
		    rejected_by = excluded.rejected_by,
		    reject_reason = excluded.reject_reason,
		    modified_at = excluded.modified_at
	`,
		a.ScriptName, a.Hash, string(a.Status),
		nullTime(a.ApprovedAt), nullString(a.ApprovedBy),
		nullTime(a.RejectedAt), nullString(a.RejectedBy), nullString(a.RejectReason),
		a.CreatedAt.UTC().Format(time.RFC3339Nano),
		a.ModifiedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("approvals: upsert %s: %w", a.ScriptName, err)
	}
	return nil
}

// Delete removes the row for a script name. Absent rows are not an
// error — ScriptStore.Delete calls this even if the script was never
// approved.
func (s *Store) Delete(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM op_approvals WHERE script_name = ?`, name)
	if err != nil {
		return fmt.Errorf("approvals: delete %s: %w", name, err)
	}
	return nil
}

// All returns a name→Approval map of every row. Used by ScriptStore
// on boot so the handler can enrich the script file list in a single
// pass instead of N round-trips.
func (s *Store) All(ctx context.Context) (map[string]*Approval, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT script_name, hash, status, approved_at, approved_by,
		       rejected_at, rejected_by, reject_reason, created_at, modified_at
		  FROM op_approvals`)
	if err != nil {
		return nil, fmt.Errorf("approvals: all: %w", err)
	}
	defer rows.Close()

	out := map[string]*Approval{}
	for rows.Next() {
		a, err := scanApprovalRow(rows)
		if err != nil {
			return nil, err
		}
		out[a.ScriptName] = a
	}
	return out, rows.Err()
}

// rowScanner matches both *sql.Row and *sql.Rows so scanApproval can
// service both the Get and All paths with one code path.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanApproval(r rowScanner) (*Approval, error) {
	a, err := scanApprovalCore(r)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func scanApprovalRow(rows *sql.Rows) (*Approval, error) {
	return scanApprovalCore(rows)
}

func scanApprovalCore(r rowScanner) (*Approval, error) {
	var (
		a                                     Approval
		status                                string
		approvedAt, rejectedAt                sql.NullString
		approvedBy, rejectedBy, rejectReason  sql.NullString
		createdAt, modifiedAt                 string
	)
	err := r.Scan(&a.ScriptName, &a.Hash, &status,
		&approvedAt, &approvedBy,
		&rejectedAt, &rejectedBy, &rejectReason,
		&createdAt, &modifiedAt)
	if err != nil {
		return nil, err
	}
	a.Status = Status(status)
	if t, ok := parseNullTime(approvedAt); ok {
		a.ApprovedAt = &t
	}
	a.ApprovedBy = approvedBy.String
	if t, ok := parseNullTime(rejectedAt); ok {
		a.RejectedAt = &t
	}
	a.RejectedBy = rejectedBy.String
	a.RejectReason = rejectReason.String
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	a.ModifiedAt, _ = time.Parse(time.RFC3339Nano, modifiedAt)
	return &a, nil
}

// nullTime returns a *time.Time's RFC3339Nano encoding, or sql.NullString{}
// when the pointer is nil — so the column is stored as NULL.
func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339Nano), Valid: true}
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func parseNullTime(s sql.NullString) (time.Time, bool) {
	if !s.Valid {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
