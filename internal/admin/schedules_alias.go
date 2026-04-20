package admin

import (
	"github.com/open-pact/openpact/internal/storage/schedules"
)

// Schedule, OutputTarget, and ScheduleStore are re-exported from the
// storage/schedules package so the admin HTTP handlers and external
// callers (orchestrator, scheduler, MCP schedule tools) don't churn
// through every usage site. Direct use of the storage/schedules
// package is equally fine — these aliases exist to keep the existing
// admin import surface compatible while the underlying data moves
// from JSON to SQLite.
type (
	Schedule      = schedules.Schedule
	OutputTarget  = schedules.OutputTarget
	ScheduleStore = schedules.Store
)

// ErrScheduleNotFound is re-exported for errors.Is callers.
var ErrScheduleNotFound = schedules.ErrScheduleNotFound
