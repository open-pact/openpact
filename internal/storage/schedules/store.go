// Package schedules persists cron-scheduled Starlark scripts and
// agent prompts in the op_schedules table. Each row is a complete
// Schedule; output_target (a pointer-typed sub-record) is stored as
// a JSON blob in its own column because the admin UI either sets it
// whole or sets it to null.
package schedules

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Errors. Kept verbatim from the old JSON-backed store so callers
// don't have to rewrite their error-handling.
var (
	ErrScheduleNotFound = errors.New("schedule not found")
	ErrScheduleExists   = errors.New("schedule already exists")
)

// Schedule is the persisted row shape. JSON tags match the admin UI
// wire format.
type Schedule struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	CronExpr      string        `json:"cron_expr"`
	Type          string        `json:"type"` // script or agent
	Enabled       bool          `json:"enabled"`
	RunOnce       bool          `json:"run_once,omitempty"`
	ScriptName    string        `json:"script_name,omitempty"`
	Prompt        string        `json:"prompt,omitempty"`
	OutputTarget  *OutputTarget `json:"output_target,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	LastRunAt     *time.Time    `json:"last_run_at,omitempty"`
	LastRunStatus string        `json:"last_run_status,omitempty"`
	LastRunError  string        `json:"last_run_error,omitempty"`
	LastRunOutput string        `json:"last_run_output,omitempty"`
}

// OutputTarget specifies where to send job output. Nil = don't
// forward; handler emits a no-op confirmation line.
type OutputTarget struct {
	Provider  string `json:"provider"`
	ChannelID string `json:"channel_id"`
}

const (
	maxScheduleNameLen = 128
	maxPromptLen       = 8192
	maxOutputLen       = 2000
)

// Store is a thin handle around the shared *sql.DB for op_schedules.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over the given shared DB.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// List returns every schedule ordered by name. Empty slice when
// none exist.
func (s *Store) List(ctx context.Context) ([]*Schedule, error) {
	rows, err := s.db.QueryContext(ctx, selectAll+" ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("schedules: list: %w", err)
	}
	defer rows.Close()

	out := []*Schedule{}
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// Get returns one schedule by ID.
func (s *Store) Get(ctx context.Context, id string) (*Schedule, error) {
	row := s.db.QueryRowContext(ctx, selectAll+" WHERE id = ?", id)
	sc, err := scanSchedule(row)
	if err == sql.ErrNoRows {
		return nil, ErrScheduleNotFound
	}
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Create inserts a new schedule with a generated ID and returns the
// stored row. Validation matches the old store's rules.
func (s *Store) Create(ctx context.Context, sched *Schedule) (*Schedule, error) {
	if err := validate(sched); err != nil {
		return nil, err
	}
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("schedules: generate ID: %w", err)
	}
	now := time.Now().UTC()
	row := &Schedule{
		ID:           id,
		Name:         sched.Name,
		CronExpr:     sched.CronExpr,
		Type:         sched.Type,
		Enabled:      sched.Enabled,
		RunOnce:      sched.RunOnce,
		ScriptName:   sched.ScriptName,
		Prompt:       sched.Prompt,
		OutputTarget: sched.OutputTarget,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.insert(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Update modifies an existing schedule. Fields on `updates` replace
// the corresponding fields on the stored row; zero values on
// updates.Type/Name/CronExpr/ScriptName/Prompt are *not* applied so
// the admin UI can send partial updates.
func (s *Store) Update(ctx context.Context, id string, updates *Schedule) (*Schedule, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if updates.Name != "" {
		if len(updates.Name) > maxScheduleNameLen {
			return nil, fmt.Errorf("name exceeds %d characters", maxScheduleNameLen)
		}
		existing.Name = updates.Name
	}
	if updates.CronExpr != "" {
		existing.CronExpr = updates.CronExpr
	}
	if updates.Type != "" {
		if updates.Type != "script" && updates.Type != "agent" {
			return nil, fmt.Errorf("type must be 'script' or 'agent'")
		}
		existing.Type = updates.Type
	}
	if updates.ScriptName != "" {
		existing.ScriptName = updates.ScriptName
	}
	if updates.Prompt != "" {
		if len(updates.Prompt) > maxPromptLen {
			return nil, fmt.Errorf("prompt exceeds %d characters", maxPromptLen)
		}
		existing.Prompt = updates.Prompt
	}
	// OutputTarget / RunOnce: explicit overwrite (consistent with the
	// old JSON store's behaviour).
	existing.OutputTarget = updates.OutputTarget
	existing.RunOnce = updates.RunOnce
	existing.UpdatedAt = time.Now().UTC()

	if err := s.upsert(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// Delete removes a schedule. Returns ErrScheduleNotFound if no row.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM op_schedules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("schedules: delete %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrScheduleNotFound
	}
	return nil
}

// SetEnabled toggles a schedule's enabled flag and bumps UpdatedAt.
func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		`UPDATE op_schedules SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), now, id,
	)
	if err != nil {
		return fmt.Errorf("schedules: set_enabled %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrScheduleNotFound
	}
	return nil
}

// UpdateLastRun records a run's result. Output is truncated to
// maxOutputLen to cap DB growth.
func (s *Store) UpdateLastRun(ctx context.Context, id, status, errMsg, output string) error {
	if len(output) > maxOutputLen {
		output = output[:maxOutputLen]
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
		UPDATE op_schedules SET
		    last_run_at = ?, last_run_status = ?, last_run_error = ?, last_run_output = ?
		  WHERE id = ?`,
		now, status, errMsg, output, id)
	if err != nil {
		return fmt.Errorf("schedules: update_last_run %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrScheduleNotFound
	}
	return nil
}

// --- internals ----------------------------------------------------------

const selectAll = `SELECT id, name, cron_expr, type, enabled, run_once,
                          script_name, prompt, output_target_json,
                          created_at, updated_at,
                          last_run_at, last_run_status, last_run_error, last_run_output
                     FROM op_schedules`

func (s *Store) insert(ctx context.Context, sc *Schedule) error {
	return s.upsert(ctx, sc)
}

func (s *Store) upsert(ctx context.Context, sc *Schedule) error {
	var outputJSON sql.NullString
	if sc.OutputTarget != nil {
		b, _ := json.Marshal(sc.OutputTarget)
		outputJSON = sql.NullString{String: string(b), Valid: true}
	}
	lastRunAt := sql.NullString{}
	if sc.LastRunAt != nil {
		lastRunAt = sql.NullString{String: sc.LastRunAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO op_schedules (id, name, cron_expr, type, enabled, run_once,
		                          script_name, prompt, output_target_json,
		                          created_at, updated_at,
		                          last_run_at, last_run_status, last_run_error, last_run_output)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		    name = excluded.name,
		    cron_expr = excluded.cron_expr,
		    type = excluded.type,
		    enabled = excluded.enabled,
		    run_once = excluded.run_once,
		    script_name = excluded.script_name,
		    prompt = excluded.prompt,
		    output_target_json = excluded.output_target_json,
		    updated_at = excluded.updated_at,
		    last_run_at = excluded.last_run_at,
		    last_run_status = excluded.last_run_status,
		    last_run_error = excluded.last_run_error,
		    last_run_output = excluded.last_run_output
	`,
		sc.ID, sc.Name, sc.CronExpr, sc.Type,
		boolToInt(sc.Enabled), boolToInt(sc.RunOnce),
		sc.ScriptName, sc.Prompt, outputJSON,
		sc.CreatedAt.UTC().Format(time.RFC3339Nano),
		sc.UpdatedAt.UTC().Format(time.RFC3339Nano),
		lastRunAt, sc.LastRunStatus, sc.LastRunError, sc.LastRunOutput,
	)
	if err != nil {
		return fmt.Errorf("schedules: upsert %s: %w", sc.ID, err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanSchedule(r rowScanner) (*Schedule, error) {
	var (
		sc                                        Schedule
		enabled, runOnce                          int
		scriptName, prompt                        sql.NullString
		outputJSON                                sql.NullString
		createdAt, updatedAt                      string
		lastRunAt                                 sql.NullString
		lastRunStatus, lastRunError, lastRunOut   sql.NullString
	)
	err := r.Scan(&sc.ID, &sc.Name, &sc.CronExpr, &sc.Type,
		&enabled, &runOnce,
		&scriptName, &prompt, &outputJSON,
		&createdAt, &updatedAt,
		&lastRunAt, &lastRunStatus, &lastRunError, &lastRunOut)
	if err != nil {
		return nil, err
	}
	sc.Enabled = enabled != 0
	sc.RunOnce = runOnce != 0
	sc.ScriptName = scriptName.String
	sc.Prompt = prompt.String
	if outputJSON.Valid && outputJSON.String != "" {
		var ot OutputTarget
		if err := json.Unmarshal([]byte(outputJSON.String), &ot); err == nil {
			sc.OutputTarget = &ot
		}
	}
	sc.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	sc.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if lastRunAt.Valid {
		if t, err := time.Parse(time.RFC3339Nano, lastRunAt.String); err == nil {
			sc.LastRunAt = &t
		}
	}
	sc.LastRunStatus = lastRunStatus.String
	sc.LastRunError = lastRunError.String
	sc.LastRunOutput = lastRunOut.String
	return &sc, nil
}

func validate(s *Schedule) error {
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(s.Name) > maxScheduleNameLen {
		return fmt.Errorf("name exceeds %d characters", maxScheduleNameLen)
	}
	if s.CronExpr == "" {
		return fmt.Errorf("cron_expr is required")
	}
	if s.Type != "script" && s.Type != "agent" {
		return fmt.Errorf("type must be 'script' or 'agent'")
	}
	if s.Type == "script" && s.ScriptName == "" {
		return fmt.Errorf("script_name is required for type 'script'")
	}
	if s.Type == "agent" && s.Prompt == "" {
		return fmt.Errorf("prompt is required for type 'agent'")
	}
	if s.Type == "agent" && len(s.Prompt) > maxPromptLen {
		return fmt.Errorf("prompt exceeds %d characters", maxPromptLen)
	}
	return nil
}

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
