package admin

import (
	"encoding/json"
	"net/http"
)

// LoginRequest represents a login request.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SessionResponse represents the session/access token response.
type SessionResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   string `json:"expires_at"`
	Username    string `json:"username"`
}

// SessionHandler handles authentication endpoints.
type SessionHandler struct {
	users        *UserStore
	jwt          *JWTManager
	secureCookie bool
}

// NewSessionHandler creates a new session handler.
func NewSessionHandler(users *UserStore, jwt *JWTManager, secureCookie bool) *SessionHandler {
	return &SessionHandler{
		users:        users,
		jwt:          jwt,
		secureCookie: secureCookie,
	}
}

// Login authenticates a user and sets the refresh token cookie.
func (h *SessionHandler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_request",
			"message": "Invalid request body",
		})
		return
	}

	// Validate credentials
	_, err := h.users.Validate(req.Username, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_credentials",
			"message": "Invalid username or password",
		})
		return
	}

	// Create refresh token
	refreshToken, _, err := h.jwt.CreateRefreshToken(req.Username)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "token_error",
			"message": "Failed to create session",
		})
		return
	}

	// Set refresh token as HTTP-only cookie
	SetRefreshCookie(w, refreshToken, h.secureCookie)

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login successful",
	})
}

// Session exchanges a refresh token cookie for an access token.
func (h *SessionHandler) Session(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie, err := r.Cookie("refresh")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "no_refresh_token",
			"message": "No refresh token provided",
		})
		return
	}

	claims, err := h.jwt.ValidateRefreshToken(cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_refresh_token",
			"message": "Invalid or expired refresh token",
		})
		return
	}

	// Create short-lived access token
	accessToken, expiresAt, err := h.jwt.CreateAccessToken(claims.Username)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "token_error",
			"message": "Failed to create access token",
		})
		return
	}

	json.NewEncoder(w).Encode(SessionResponse{
		AccessToken: accessToken,
		ExpiresAt:   expiresAt.Format("2006-01-02T15:04:05Z07:00"),
		Username:    claims.Username,
	})
}

// Logout clears the refresh token cookie.
func (h *SessionHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ClearRefreshCookie(w, h.secureCookie)
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the current user's information.
func (h *SessionHandler) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	username, ok := UsernameFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_authenticated",
			"message": "Not authenticated",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"username": username,
		"role":     "admin", // Single user for v1, always admin
	})
}
