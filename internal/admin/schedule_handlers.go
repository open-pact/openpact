package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/robfig/cron/v3"
)

// SchedulerAPI is the interface for triggering immediate schedule runs.
type SchedulerAPI interface {
	RunNow(id string) error
	Reload() error
}

// ScheduleHandlers handles HTTP requests for schedule management.
type ScheduleHandlers struct {
	store     *ScheduleStore
	scheduler SchedulerAPI
}

// NewScheduleHandlers creates new schedule handlers.
func NewScheduleHandlers(store *ScheduleStore) *ScheduleHandlers {
	return &ScheduleHandlers{store: store}
}

// SetSchedulerAPI sets the scheduler API (called after orchestrator is created).
func (h *ScheduleHandlers) SetSchedulerAPI(api SchedulerAPI) {
	h.scheduler = api
}

// ListSchedules handles GET /api/schedules.
func (h *ScheduleHandlers) ListSchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	schedules, err := h.store.List()
	if err != nil {
		http.Error(w, `{"error":"internal","message":"Failed to list schedules"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"schedules": schedules})
}

// CreateSchedule handles POST /api/schedules.
func (h *ScheduleHandlers) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req Schedule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request","message":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Validate cron expression
	if req.CronExpr != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(req.CronExpr); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "bad_request",
				"message": "Invalid cron expression: " + err.Error(),
			})
			return
		}
	}

	sched, err := h.store.Create(&req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad_request",
			"message": err.Error(),
		})
		return
	}

	// Reload scheduler to pick up new schedule
	if h.scheduler != nil {
		h.scheduler.Reload()
	}

	writeJSON(w, http.StatusCreated, sched)
}

// HandleScheduleByID handles /api/schedules/:id requests.
func (h *ScheduleHandlers) HandleScheduleByID(w http.ResponseWriter, r *http.Request) {
	id := extractScheduleID(r.URL.Path)
	if id == "" {
		http.Error(w, `{"error":"bad_request","message":"Schedule ID required"}`, http.StatusBadRequest)
		return
	}

	// Check for action sub-paths
	suffix := strings.TrimPrefix(r.URL.Path, "/api/schedules/"+id)
	switch suffix {
	case "/enable":
		if r.Method == http.MethodPost {
			h.EnableSchedule(w, r, id)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	case "/disable":
		if r.Method == http.MethodPost {
			h.DisableSchedule(w, r, id)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	case "/run":
		if r.Method == http.MethodPost {
			h.RunSchedule(w, r, id)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	case "":
		// Fall through to standard CRUD
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.GetSchedule(w, r, id)
	case http.MethodPut:
		h.UpdateSchedule(w, r, id)
	case http.MethodDelete:
		h.DeleteSchedule(w, r, id)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

// GetSchedule handles GET /api/schedules/:id.
func (h *ScheduleHandlers) GetSchedule(w http.ResponseWriter, r *http.Request, id string) {
	sched, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, ErrScheduleNotFound) {
			http.Error(w, `{"error":"not_found","message":"Schedule not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal","message":"Failed to get schedule"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, sched)
}

// UpdateSchedule handles PUT /api/schedules/:id.
func (h *ScheduleHandlers) UpdateSchedule(w http.ResponseWriter, r *http.Request, id string) {
	var req Schedule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request","message":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Validate cron expression if provided
	if req.CronExpr != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(req.CronExpr); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "bad_request",
				"message": "Invalid cron expression: " + err.Error(),
			})
			return
		}
	}

	sched, err := h.store.Update(id, &req)
	if err != nil {
		if errors.Is(err, ErrScheduleNotFound) {
			http.Error(w, `{"error":"not_found","message":"Schedule not found"}`, http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad_request",
			"message": err.Error(),
		})
		return
	}

	// Reload scheduler to pick up changes
	if h.scheduler != nil {
		h.scheduler.Reload()
	}

	writeJSON(w, http.StatusOK, sched)
}

// DeleteSchedule handles DELETE /api/schedules/:id.
func (h *ScheduleHandlers) DeleteSchedule(w http.ResponseWriter, r *http.Request, id string) {
	err := h.store.Delete(id)
	if err != nil {
		if errors.Is(err, ErrScheduleNotFound) {
			http.Error(w, `{"error":"not_found","message":"Schedule not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal","message":"Failed to delete schedule"}`, http.StatusInternalServerError)
		return
	}

	// Reload scheduler to remove the entry
	if h.scheduler != nil {
		h.scheduler.Reload()
	}

	w.WriteHeader(http.StatusNoContent)
}

// EnableSchedule handles POST /api/schedules/:id/enable.
func (h *ScheduleHandlers) EnableSchedule(w http.ResponseWriter, r *http.Request, id string) {
	err := h.store.SetEnabled(id, true)
	if err != nil {
		if errors.Is(err, ErrScheduleNotFound) {
			http.Error(w, `{"error":"not_found","message":"Schedule not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal","message":"Failed to enable schedule"}`, http.StatusInternalServerError)
		return
	}

	if h.scheduler != nil {
		h.scheduler.Reload()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

// DisableSchedule handles POST /api/schedules/:id/disable.
func (h *ScheduleHandlers) DisableSchedule(w http.ResponseWriter, r *http.Request, id string) {
	err := h.store.SetEnabled(id, false)
	if err != nil {
		if errors.Is(err, ErrScheduleNotFound) {
			http.Error(w, `{"error":"not_found","message":"Schedule not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal","message":"Failed to disable schedule"}`, http.StatusInternalServerError)
		return
	}

	if h.scheduler != nil {
		h.scheduler.Reload()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// RunSchedule handles POST /api/schedules/:id/run.
func (h *ScheduleHandlers) RunSchedule(w http.ResponseWriter, r *http.Request, id string) {
	if h.scheduler == nil {
		http.Error(w, `{"error":"service_unavailable","message":"Scheduler not available"}`, http.StatusServiceUnavailable)
		return
	}

	if err := h.scheduler.RunNow(id); err != nil {
		if errors.Is(err, ErrScheduleNotFound) {
			http.Error(w, `{"error":"not_found","message":"Schedule not found"}`, http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad_request",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
}

// extractScheduleID extracts the schedule ID from /api/schedules/:id[/action] path.
func extractScheduleID(path string) string {
	prefix := "/api/schedules/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	// Remove any trailing path segments (e.g., /enable, /disable)
	if idx := strings.Index(rest, "/"); idx != -1 {
		return rest[:idx]
	}
	return strings.TrimRight(rest, "/")
}
