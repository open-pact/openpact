// Package channels persists two per-chat-channel records:
//
//   - Session: the stackllm session UUID the orchestrator reuses for
//     every incoming message on a given (provider, channel_id) pair.
//     Keeps conversation continuity across orchestrator restarts.
//
//   - Mode: the admin-UI-selected "detail mode" (simple / thinking /
//     tools / full) that controls how the bot formats responses in
//     that channel.
//
// Both lookups are keyed on (provider, channel_id) composite — a
// natural primary key, so no surrogate ID.
package channels

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SessionStore persists per-channel stackllm session IDs.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore returns a SessionStore for op_channel_sessions.
func NewSessionStore(db *sql.DB) *SessionStore { return &SessionStore{db: db} }

// Get returns the persisted session ID for (provider, channelID), or
// ("", false, nil) if none exists.
func (s *SessionStore) Get(ctx context.Context, provider, channelID string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT session_id FROM op_channel_sessions WHERE provider = ? AND channel_id = ?`,
		provider, channelID,
	).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("channels: get session: %w", err)
	}
	return id, true, nil
}

// Set upserts the session ID for a channel.
func (s *SessionStore) Set(ctx context.Context, provider, channelID, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO op_channel_sessions (provider, channel_id, session_id, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(provider, channel_id) DO UPDATE SET
		    session_id = excluded.session_id,
		    updated_at = excluded.updated_at
	`, provider, channelID, sessionID, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("channels: set session: %w", err)
	}
	return nil
}

// All returns a flat map keyed "provider:channel_id" → session_id.
// Matches the map shape the orchestrator's in-memory cache uses.
func (s *SessionStore) All(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT provider, channel_id, session_id FROM op_channel_sessions`,
	)
	if err != nil {
		return nil, fmt.Errorf("channels: all sessions: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var p, c, id string
		if err := rows.Scan(&p, &c, &id); err != nil {
			return nil, err
		}
		out[p+":"+c] = id
	}
	return out, rows.Err()
}

// ModeStore persists per-channel detail-mode selections.
type ModeStore struct {
	db *sql.DB
}

// NewModeStore returns a ModeStore for op_channel_modes.
func NewModeStore(db *sql.DB) *ModeStore { return &ModeStore{db: db} }

// Get returns the mode for (provider, channelID). If no row exists,
// returns ("", false, nil) — the caller applies its default (usually
// "simple").
func (s *ModeStore) Get(ctx context.Context, provider, channelID string) (string, bool, error) {
	var m string
	err := s.db.QueryRowContext(ctx,
		`SELECT mode FROM op_channel_modes WHERE provider = ? AND channel_id = ?`,
		provider, channelID,
	).Scan(&m)
	switch {
	case err == sql.ErrNoRows:
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("channels: get mode: %w", err)
	}
	return m, true, nil
}

// Set upserts the mode for a channel.
func (s *ModeStore) Set(ctx context.Context, provider, channelID, mode string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO op_channel_modes (provider, channel_id, mode, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(provider, channel_id) DO UPDATE SET
		    mode = excluded.mode,
		    updated_at = excluded.updated_at
	`, provider, channelID, mode, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("channels: set mode: %w", err)
	}
	return nil
}

// All returns a flat map keyed "provider:channel_id" → mode. Matches
// the map shape ListChannelModes uses.
func (s *ModeStore) All(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT provider, channel_id, mode FROM op_channel_modes`,
	)
	if err != nil {
		return nil, fmt.Errorf("channels: all modes: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var p, c, m string
		if err := rows.Scan(&p, &c, &m); err != nil {
			return nil, err
		}
		out[p+":"+c] = m
	}
	return out, rows.Err()
}
