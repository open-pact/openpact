package migrate

// LatestVersion is the highest migration index currently defined.
// When adding a new migration: append to `migrations` and bump this.
const LatestVersion = 1

// migrations[i] is the SQL that upgrades the DB from version i to
// i+1. So migrations[0] is applied to a fresh (version-0) database.
//
// Every table in this file uses the op_ prefix — we share the SQLite
// file with stackllm, which reserves stackllm_*. Timestamps are
// stored as ISO-8601 TEXT (matching stackllm's convention) so a dump
// of the file reads naturally and date comparisons work with
// string ordering.
var migrations = []string{
	schemaV1,
}

const schemaV1 = `
CREATE TABLE op_schema_version (
    version INTEGER NOT NULL
);

-- users.json replacement: admin accounts authenticated against the
-- admin UI. Bcrypt hashes only, never plaintext.
CREATE TABLE op_users (
    username        TEXT PRIMARY KEY,
    password_hash   TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    last_login_at   TEXT
);

-- approvals.json replacement: script-name keyed, tracks the hash the
-- approval was granted against so a script modified after approval
-- falls back to pending automatically. The script source itself
-- stays on disk (see ai/specs/starklark-to-db.md for the future
-- migration that moves it here too).
CREATE TABLE op_approvals (
    script_name     TEXT PRIMARY KEY,
    hash            TEXT NOT NULL,
    status          TEXT NOT NULL,  -- pending/approved/rejected
    approved_at     TEXT,
    approved_by     TEXT,
    rejected_at     TEXT,
    rejected_by     TEXT,
    reject_reason   TEXT,
    created_at      TEXT NOT NULL,
    modified_at     TEXT NOT NULL
);

-- starlark_secrets.json replacement. value is base64(nonce(12) ||
-- ciphertext) under AES-256-GCM using the data_encryption_key
-- bootstrapped at <workspace>/secure/data/data_encryption_key.
CREATE TABLE op_secrets (
    name        TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

-- chat_providers.json replacement. tokens/allowed_users/allowed_chans
-- are JSON-encoded (lists and small maps per row — querying by
-- individual allowed-user is not a workflow we have). Provider
-- tokens remain plaintext at the SQL row level, protected by the
-- DB file's 0600 perms; a future migration can move them into
-- op_secrets with encryption.
CREATE TABLE op_chat_providers (
    name            TEXT PRIMARY KEY,
    enabled         INTEGER NOT NULL,
    tokens_json     TEXT NOT NULL,
    allowed_users_json TEXT NOT NULL,
    allowed_chans_json TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

-- schedules.json replacement: cron-scheduled Starlark scripts and
-- agent prompts. output_target is JSON (nullable).
CREATE TABLE op_schedules (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    cron_expr        TEXT NOT NULL,
    type             TEXT NOT NULL,  -- script or agent
    enabled          INTEGER NOT NULL,
    run_once         INTEGER NOT NULL,
    script_name      TEXT,
    prompt           TEXT,
    output_target_json TEXT,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    last_run_at      TEXT,
    last_run_status  TEXT,
    last_run_error   TEXT,
    last_run_output  TEXT
);
CREATE INDEX idx_op_schedules_name ON op_schedules(name);

-- Scalar key-value store for settings that are flat (setup_state,
-- advanced_settings, integrations.vault.*, integrations.github.*).
-- scope groups related rows; key is dotted inside a scope.
-- Lists of records (e.g. calendars) get their own table.
CREATE TABLE op_kv (
    scope   TEXT NOT NULL,
    key     TEXT NOT NULL,
    value   TEXT NOT NULL,
    PRIMARY KEY (scope, key)
);

-- Calendar feeds — a list of records, so its own table. position
-- controls display order in the admin UI.
CREATE TABLE op_calendars (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    position  INTEGER NOT NULL,
    name      TEXT NOT NULL,
    url       TEXT NOT NULL
);
CREATE INDEX idx_op_calendars_position ON op_calendars(position);

-- channel_sessions.json replacement: (provider, channel_id) → stackllm
-- session uuid. Persists the "reuse this session for the same channel"
-- behavior across orchestrator restarts.
CREATE TABLE op_channel_sessions (
    provider    TEXT NOT NULL,
    channel_id  TEXT NOT NULL,
    session_id  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (provider, channel_id)
);

-- channel_modes.json replacement: (provider, channel_id) → detail
-- mode (simple/thinking/tools/full).
CREATE TABLE op_channel_modes (
    provider    TEXT NOT NULL,
    channel_id  TEXT NOT NULL,
    mode        TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (provider, channel_id)
);
`
