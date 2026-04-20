package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-pact/openpact/internal/storage/kv"
	"github.com/open-pact/openpact/internal/storage/users"
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

// SetupState tracks multi-step setup progress. Type alias to the kv
// package so the admin API shape is untouched while the data lives
// in op_kv rows (scope='setup_state').
type SetupState = kv.SetupState

// SetupHandler handles first-run setup.
type SetupHandler struct {
	users        *users.Store
	db           *sql.DB
	aiDataDir    string
	jwt          *JWTManager
	secureCookie bool
	// defaultModelSet is injected rather than imported from engine/profile
	// to keep the admin package free of stackllm types. Returns true if
	// the profile manager has a persisted default model.
	defaultModelSet func() bool
}

// NewSetupHandler creates a new setup handler. jwt and secureCookie are
// required so POST /api/setup can issue a refresh cookie on success —
// the remaining setup steps then run authenticated just like any other
// admin endpoint, so /api/engine/* never has to be publicly reachable.
func NewSetupHandler(users *users.Store, db *sql.DB, aiDataDir string, jwt *JWTManager, secureCookie bool, defaultModelSet func() bool) *SetupHandler {
	return &SetupHandler{
		users:           users,
		db:              db,
		aiDataDir:       aiDataDir,
		jwt:             jwt,
		secureCookie:    secureCookie,
		defaultModelSet: defaultModelSet,
	}
}

func (h *SetupHandler) loadSetupState() (*SetupState, error) {
	s, err := kv.LoadSetupState(context.Background(), h.db)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (h *SetupHandler) saveSetupState(state *SetupState) error {
	return kv.SaveSetupState(context.Background(), h.db, *state)
}

// currentSetupStep returns the current setup step.
func (h *SetupHandler) currentSetupStep() string {
	if !h.users.HasUsers(context.Background()) {
		return "account"
	}
	state, err := h.loadSetupState()
	if err != nil || !state.ProfileComplete {
		return "profile"
	}
	if !state.ProviderComplete {
		return "provider"
	}
	return "complete"
}

// Provider marks the LLM-provider step of setup complete. Called after
// the user has authenticated at least one provider and chosen a default
// model via the /api/engine/ endpoints.
func (h *SetupHandler) Provider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "method_not_allowed",
			"message": "POST required",
		})
		return
	}

	if !h.users.HasUsers(r.Context()) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "account_required",
			"message": "Create an account first",
		})
		return
	}

	state, err := h.loadSetupState()
	if err != nil {
		state = &SetupState{}
	}
	if !state.ProfileComplete {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "profile_required",
			"message": "Complete the profile step first",
		})
		return
	}

	// Server-side gate: a default model must actually be set before we
	// mark the wizard complete. Previously the handler trusted the
	// frontend's own check, which meant a buggy or malicious client
	// could flip the flag with no provider ever authenticated.
	if h.defaultModelSet != nil && !h.defaultModelSet() {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "default_model_required",
			"message": "Authenticate a provider and pick a default model before finishing setup",
		})
		return
	}

	state.ProviderComplete = true
	if err := h.saveSetupState(state); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "state_failed",
			"message": "Failed to save setup state",
		})
		return
	}

	json.NewEncoder(w).Encode(SetupResponse{
		Success: true,
		Message: "Setup complete. Please log in.",
	})
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
	if h.users.HasUsers(r.Context()) {
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
	if err := users.ValidatePasswords(req.Password, req.ConfirmPassword); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errMsg := "Password does not meet requirements"
		if errors.Is(err, users.ErrPasswordMismatch) {
			errMsg = "Passwords do not match"
		}
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_password",
			"message": errMsg,
		})
		return
	}

	// Create the user
	_, err := h.users.Create(r.Context(), req.Username, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "create_failed",
			"message": "Failed to create user",
		})
		return
	}

	// Issue the refresh cookie now so the wizard's subsequent calls
	// (profile + provider login + default model pick) run
	// authenticated. Previously the wizard ran entirely
	// unauthenticated and the setup middleware whitelisted
	// /api/engine/* to compensate — that exposed /chat, /sessions/*
	// and /logout to anyone who could reach the bind address during
	// the setup window. Issuing a cookie here closes that hole.
	if h.jwt != nil {
		if refreshToken, _, err := h.jwt.CreateRefreshToken(req.Username); err == nil {
			SetRefreshCookie(w, refreshToken, h.secureCookie)
		} else {
			log.Printf("Warning: failed to mint refresh token on setup: %v", err)
		}
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
	if !h.users.HasUsers(r.Context()) {
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

	// Write SOUL.md and USER.md to AI data directory
	if err := os.WriteFile(filepath.Join(h.aiDataDir, "SOUL.md"), []byte(soulContent), 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "write_failed",
			"message": "Failed to write SOUL.md",
		})
		return
	}

	if err := os.WriteFile(filepath.Join(h.aiDataDir, "USER.md"), []byte(userContent), 0644); err != nil {
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
func RequireSetupMiddleware(userStore *users.Store, db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always allow setup endpoints, session/auth endpoints
			// (so the wizard can refresh its access token after
			// creating the account), static assets, and a narrow
			// subset of /api/engine/* that step 3 of the wizard
			// genuinely needs. All whitelisted /api/engine/* paths
			// are still auth-gated by withAuth — the whitelist is
			// about the setup 503, not about authentication.
			if r.URL.Path == "/api/setup" || r.URL.Path == "/api/setup/status" ||
				r.URL.Path == "/api/setup/profile" || r.URL.Path == "/api/setup/provider" ||
				r.URL.Path == "/api/session" || r.URL.Path == "/api/auth/login" ||
				r.URL.Path == "/api/auth/logout" ||
				r.URL.Path == "/api/version" ||
				isSetupEngineEndpoint(r.URL.Path) ||
				r.URL.Path == "/setup" || r.URL.Path == "/" ||
				strings.HasPrefix(r.URL.Path, "/assets/") {
				next.ServeHTTP(w, r)
				return
			}

			// If no users exist, block all other endpoints (account step)
			if !userStore.HasUsers(r.Context()) {
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

			state, _ := kv.LoadSetupState(r.Context(), db)

			if !state.ProfileComplete {
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

			if !state.ProviderComplete {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":          "setup_required",
					"message":        "LLM provider setup required",
					"setup_required": true,
					"setup_step":     "provider",
					"redirect":       "/setup",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isSetupEngineEndpoint reports whether the given path is one of the
// narrow set of /api/engine/* endpoints step 3 of the setup wizard
// needs — provider list, login/oauth/status, model list, default
// model get/set. Notably excluded: /chat, /sessions/*, and the
// per-provider logout endpoint. All whitelisted paths are still
// auth-gated by withAuth — the whitelist governs the setup 503, not
// authentication.
func isSetupEngineEndpoint(p string) bool {
	const prefix = "/api/engine"
	if !strings.HasPrefix(p, prefix) {
		return false
	}
	q := strings.TrimPrefix(p, prefix)
	switch {
	case q == "/providers":
		return true
	case strings.HasPrefix(q, "/providers/"):
		// Only the login / status / oauth sub-paths.
		// /providers/{name}/logout is intentionally NOT whitelisted.
		return strings.HasSuffix(q, "/login") ||
			strings.HasSuffix(q, "/oauth/login") ||
			strings.HasSuffix(q, "/oauth/status") ||
			strings.HasSuffix(q, "/status")
	case q == "/models" || strings.HasPrefix(q, "/models/"):
		return true
	case q == "/default":
		return true
	}
	return false
}
