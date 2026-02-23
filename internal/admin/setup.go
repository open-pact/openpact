package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SetupRequest represents the first-run setup request.
type SetupRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// SetupResponse represents the setup response.
type SetupResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SetupStatusResponse represents the setup status check response.
type SetupStatusResponse struct {
	SetupRequired bool   `json:"setup_required"`
	SetupStep     string `json:"setup_step"` // "account", "profile", or "complete"
}

// ProfileRequest represents the profile setup request (step 2).
type ProfileRequest struct {
	AgentName   string `json:"agent_name"`
	Personality string `json:"personality"`
	UserName    string `json:"user_name"`
	Timezone    string `json:"timezone"`
}

// SetupState tracks multi-step setup progress.
type SetupState struct {
	ProfileComplete bool `json:"profile_complete"`
}

// SetupHandler handles first-run setup.
type SetupHandler struct {
	users         *UserStore
	dataDir       string
	workspacePath string
}

// NewSetupHandler creates a new setup handler.
func NewSetupHandler(users *UserStore, dataDir, workspacePath string) *SetupHandler {
	return &SetupHandler{
		users:         users,
		dataDir:       dataDir,
		workspacePath: workspacePath,
	}
}

func (h *SetupHandler) setupStatePath() string {
	return filepath.Join(h.dataDir, "setup_state.json")
}

func (h *SetupHandler) loadSetupState() (*SetupState, error) {
	data, err := os.ReadFile(h.setupStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &SetupState{}, nil
		}
		return nil, err
	}
	var state SetupState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (h *SetupHandler) saveSetupState(state *SetupState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.setupStatePath(), data, 0644)
}

// currentSetupStep returns the current setup step.
func (h *SetupHandler) currentSetupStep() string {
	if !h.users.HasUsers() {
		return "account"
	}
	state, err := h.loadSetupState()
	if err != nil || !state.ProfileComplete {
		return "profile"
	}
	return "complete"
}

// Status returns whether setup is required.
func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	step := h.currentSetupStep()
	json.NewEncoder(w).Encode(SetupStatusResponse{
		SetupRequired: step != "complete",
		SetupStep:     step,
	})
}

// Setup handles the initial admin user creation.
func (h *SetupHandler) Setup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if setup is already complete
	if h.users.HasUsers() {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "setup_complete",
			"message": "Setup has already been completed",
		})
		return
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_request",
			"message": "Invalid request body",
		})
		return
	}

	// Validate username
	if req.Username == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_username",
			"message": "Username is required",
		})
		return
	}

	// Validate passwords match and meet requirements
	if err := ValidatePasswords(req.Password, req.ConfirmPassword); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errMsg := "Password does not meet requirements"
		if errors.Is(err, ErrPasswordMismatch) {
			errMsg = "Passwords do not match"
		}
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_password",
			"message": errMsg,
		})
		return
	}

	// Create the user
	_, err := h.users.Create(req.Username, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "create_failed",
			"message": "Failed to create user",
		})
		return
	}

	json.NewEncoder(w).Encode(SetupResponse{
		Success: true,
		Message: "Account created. Please complete your profile.",
	})
}

// Profile handles the profile setup step (step 2).
func (h *SetupHandler) Profile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "method_not_allowed",
			"message": "POST required",
		})
		return
	}

	// Must have a user account first
	if !h.users.HasUsers() {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "account_required",
			"message": "Create an account first",
		})
		return
	}

	// Check if profile is already complete
	state, err := h.loadSetupState()
	if err == nil && state.ProfileComplete {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "profile_complete",
			"message": "Profile setup has already been completed",
		})
		return
	}

	var req ProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_request",
			"message": "Invalid request body",
		})
		return
	}

	// Validate required fields
	req.AgentName = strings.TrimSpace(req.AgentName)
	req.UserName = strings.TrimSpace(req.UserName)
	req.Personality = strings.TrimSpace(req.Personality)
	req.Timezone = strings.TrimSpace(req.Timezone)

	if req.AgentName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_agent_name",
			"message": "Agent name is required",
		})
		return
	}

	if req.UserName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_user_name",
			"message": "Your name is required",
		})
		return
	}

	// Validate personality preset
	vibe, ok := PersonalityPresets[req.Personality]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_personality",
			"message": "Invalid personality preset",
		})
		return
	}

	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	// Build personalized templates
	replacer := strings.NewReplacer(
		"{{AGENT_NAME}}", req.AgentName,
		"{{AGENT_VIBE}}", vibe,
		"{{USER_NAME}}", req.UserName,
		"{{USER_TIMEZONE}}", req.Timezone,
	)

	soulContent := replacer.Replace(DefaultSoulTemplate)
	userContent := replacer.Replace(DefaultUserTemplate)

	// Write SOUL.md and USER.md to workspace root
	if err := os.WriteFile(filepath.Join(h.workspacePath, "SOUL.md"), []byte(soulContent), 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "write_failed",
			"message": "Failed to write SOUL.md",
		})
		return
	}

	if err := os.WriteFile(filepath.Join(h.workspacePath, "USER.md"), []byte(userContent), 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "write_failed",
			"message": "Failed to write USER.md",
		})
		return
	}

	// Save setup state
	if err := h.saveSetupState(&SetupState{ProfileComplete: true}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "state_failed",
			"message": "Failed to save setup state",
		})
		return
	}

	json.NewEncoder(w).Encode(SetupResponse{
		Success: true,
		Message: "Profile setup complete. Please log in.",
	})
}

// RequireSetupMiddleware blocks all requests (except setup endpoints) when setup is required.
func RequireSetupMiddleware(users *UserStore, dataDir string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always allow setup endpoints and static assets (so the SPA can load)
			if r.URL.Path == "/api/setup" || r.URL.Path == "/api/setup/status" ||
				r.URL.Path == "/api/setup/profile" ||
				r.URL.Path == "/setup" || r.URL.Path == "/" ||
				strings.HasPrefix(r.URL.Path, "/assets/") {
				next.ServeHTTP(w, r)
				return
			}

			// If no users exist, block all other endpoints (account step)
			if !users.HasUsers() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":          "setup_required",
					"message":        "Initial setup required",
					"setup_required": true,
					"setup_step":     "account",
					"redirect":       "/setup",
				})
				return
			}

			// If profile not complete, block all other endpoints (profile step)
			statePath := filepath.Join(dataDir, "setup_state.json")
			profileComplete := false
			if data, err := os.ReadFile(statePath); err == nil {
				var state SetupState
				if err := json.Unmarshal(data, &state); err == nil {
					profileComplete = state.ProfileComplete
				}
			}

			if !profileComplete {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":          "setup_required",
					"message":        "Profile setup required",
					"setup_required": true,
					"setup_step":     "profile",
					"redirect":       "/setup",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
