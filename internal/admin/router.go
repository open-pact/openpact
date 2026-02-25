package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	version "github.com/open-pact/openpact"
)

// Config holds the admin server configuration.
type Config struct {
	Bind          string
	DataDir       string
	ScriptsDir    string
	WorkspacePath string
	AIDataDir     string
	Allowlist     []string
	DevMode       bool
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
	EngineType    string // "opencode"
}

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		Bind:          "localhost:8080",
		DataDir:       "./data",
		ScriptsDir:    "./scripts",
		DevMode:       false,
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
	}
}

// Server is the admin HTTP server.
type Server struct {
	config             Config
	users              *UserStore
	scripts            *ScriptStore
	jwt                *JWTManager
	setupHandler       *SetupHandler
	sessionHandler     *SessionHandler
	scriptHandlers     *ScriptHandlers
	engineAuthHandlers *EngineAuthHandlers
	secretHandlers     *SecretHandlers
	aiSessionHandlers  *SessionHandlers
	providerHandlers   *ProviderHandlers
	secureCookie       bool
}

// NewServer creates a new admin server.
func NewServer(config Config) (*Server, error) {
	// Initialize JWT
	secret, err := GetOrCreateJWTSecret(config.DataDir)
	if err != nil {
		return nil, err
	}

	jwt := NewJWTManager(JWTConfig{
		Secret:        secret,
		AccessExpiry:  config.AccessExpiry,
		RefreshExpiry: config.RefreshExpiry,
		Issuer:        "openpact",
	})

	// Initialize user store
	users, err := NewUserStore(config.DataDir)
	if err != nil {
		return nil, err
	}

	// Initialize script store
	scripts, err := NewScriptStore(config.ScriptsDir, config.DataDir, config.Allowlist)
	if err != nil {
		return nil, err
	}

	secureCookie := ShouldUseSecureCookies(config.Bind, config.DevMode)

	engineType := config.EngineType
	if engineType == "" {
		engineType = "opencode"
	}

	secretStore := NewSecretStore(config.DataDir)
	providerStore := NewProviderStore(config.DataDir)

	return &Server{
		config:             config,
		users:              users,
		scripts:            scripts,
		jwt:                jwt,
		setupHandler:       NewSetupHandler(users, config.DataDir, config.AIDataDir),
		sessionHandler:     NewSessionHandler(users, jwt, secureCookie),
		scriptHandlers:     NewScriptHandlers(scripts),
		engineAuthHandlers: NewEngineAuthHandlers(engineType),
		secretHandlers:     NewSecretHandlers(secretStore, nil),
		providerHandlers:   NewProviderHandlers(providerStore),
		secureCookie:       secureCookie,
	}, nil
}

// Handler returns the HTTP handler for the admin API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Version endpoint (no auth required)
	mux.HandleFunc("/api/version", handleVersion)

	// Setup endpoints (no auth required, but blocked after setup complete)
	mux.HandleFunc("/api/setup/status", s.setupHandler.Status)
	mux.HandleFunc("/api/setup/profile", s.setupHandler.Profile)
	mux.HandleFunc("/api/setup", s.setupHandler.Setup)

	// Auth endpoints (no auth required)
	mux.HandleFunc("/api/auth/login", s.sessionHandler.Login)
	mux.HandleFunc("/api/auth/logout", s.sessionHandler.Logout)
	mux.HandleFunc("/api/session", s.sessionHandler.Session)

	// Protected endpoints (require auth)
	mux.HandleFunc("/api/auth/me", s.withAuth(s.sessionHandler.Me))
	mux.HandleFunc("/api/scripts", s.withAuth(s.handleScripts))
	mux.HandleFunc("/api/scripts/", s.withAuth(s.handleScriptByName))

	// Engine auth endpoints
	mux.HandleFunc("/api/engine/auth/terminal", s.withAuthWS(s.engineAuthHandlers.Terminal))
	mux.HandleFunc("/api/engine/auth", s.withAuth(s.engineAuthHandlers.HandleEngineAuth))

	// Secret management endpoints
	mux.HandleFunc("/api/secrets", s.withAuth(s.handleSecrets))
	mux.HandleFunc("/api/secrets/", s.withAuth(s.handleSecretByName))

	// AI session management endpoints
	s.registerSessionRoutes(mux)

	// Model management endpoints
	s.registerModelRoutes(mux)

	// Provider management endpoints
	s.registerProviderRoutes(mux)

	// Apply setup middleware to the entire API
	return RequireSetupMiddleware(s.users, s.config.DataDir)(mux)
}

// withAuth wraps a handler with authentication middleware.
func (s *Server) withAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract and validate access token
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"unauthorized","message":"Authorization header required"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, `{"error":"unauthorized","message":"Invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		claims, err := s.jwt.ValidateAccessToken(parts[1])
		if err != nil {
			http.Error(w, `{"error":"unauthorized","message":"Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		// Add username to context and call handler
		r = r.WithContext(WithUsername(r.Context(), claims.Username))
		handler(w, r)
	}
}

// withAuthWS wraps a handler with authentication that supports both
// Bearer tokens and query parameter tokens (for WebSocket connections).
func (s *Server) withAuthWS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try Authorization header first
		token := ""
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				token = parts[1]
			}
		}

		// Fall back to query parameter (for WebSocket)
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			http.Error(w, `{"error":"unauthorized","message":"Token required"}`, http.StatusUnauthorized)
			return
		}

		claims, err := s.jwt.ValidateAccessToken(token)
		if err != nil {
			http.Error(w, `{"error":"unauthorized","message":"Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		r = r.WithContext(WithUsername(r.Context(), claims.Username))
		handler(w, r)
	}
}

// handleScripts routes /api/scripts requests.
func (s *Server) handleScripts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.scriptHandlers.ListScripts(w, r)
	case http.MethodPost:
		s.scriptHandlers.CreateScript(w, r)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleScriptByName routes /api/scripts/:name requests.
func (s *Server) handleScriptByName(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Check for action endpoints
	if strings.HasSuffix(path, "/approve") {
		if r.Method == http.MethodPost {
			s.scriptHandlers.ApproveScript(w, r)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if strings.HasSuffix(path, "/reject") {
		if r.Method == http.MethodPost {
			s.scriptHandlers.RejectScript(w, r)
			return
		}
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Standard CRUD operations
	switch r.Method {
	case http.MethodGet:
		s.scriptHandlers.GetScript(w, r)
	case http.MethodPut:
		s.scriptHandlers.UpdateScript(w, r)
	case http.MethodDelete:
		s.scriptHandlers.DeleteScript(w, r)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

// SetupRequired returns true if initial setup is required.
func (s *Server) SetupRequired() bool {
	return !s.users.HasUsers()
}

// Users returns the user store.
func (s *Server) Users() *UserStore {
	return s.users
}

// Scripts returns the script store.
func (s *Server) Scripts() *ScriptStore {
	return s.scripts
}

// SecretStore returns the secret store.
func (s *Server) SecretStore() *SecretStore {
	return s.secretHandlers.store
}

// SetOnSecretsChanged sets the callback for secret changes.
func (s *Server) SetOnSecretsChanged(fn func()) {
	s.secretHandlers.onChange = fn
}

// SetSessionAPI sets the session API for AI session management endpoints.
func (s *Server) SetSessionAPI(api SessionAPI) {
	s.aiSessionHandlers = NewSessionHandlers(api)
}

// handleSecrets routes /api/secrets requests.
func (s *Server) handleSecrets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.secretHandlers.ListSecrets(w, r)
	case http.MethodPost:
		s.secretHandlers.CreateSecret(w, r)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

// registerSessionRoutes adds AI session routes to a mux.
// Routes are always registered; they return 503 if the session API hasn't been wired yet.
func (s *Server) registerSessionRoutes(mux *http.ServeMux) {
	sessionGuard := func(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if s.aiSessionHandlers == nil {
				http.Error(w, `{"error":"session API not available"}`, http.StatusServiceUnavailable)
				return
			}
			handler(w, r)
		}
	}
	mux.HandleFunc("/api/sessions", s.withAuth(sessionGuard(func(w http.ResponseWriter, r *http.Request) {
		s.aiSessionHandlers.HandleSessions(w, r)
	})))
	mux.HandleFunc("/api/sessions/", s.withAuthWS(sessionGuard(func(w http.ResponseWriter, r *http.Request) {
		s.aiSessionHandlers.HandleSessionByID(w, r)
	})))
}

// registerModelRoutes adds model management routes to a mux.
func (s *Server) registerModelRoutes(mux *http.ServeMux) {
	sessionGuard := func(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if s.aiSessionHandlers == nil {
				http.Error(w, `{"error":"session API not available"}`, http.StatusServiceUnavailable)
				return
			}
			handler(w, r)
		}
	}
	mux.HandleFunc("/api/models/default", s.withAuth(sessionGuard(func(w http.ResponseWriter, r *http.Request) {
		s.aiSessionHandlers.SetDefaultModel(w, r)
	})))
	mux.HandleFunc("/api/models", s.withAuth(sessionGuard(func(w http.ResponseWriter, r *http.Request) {
		s.aiSessionHandlers.ListModels(w, r)
	})))
}

// registerProviderRoutes adds provider management routes to a mux.
func (s *Server) registerProviderRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/providers", s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		s.providerHandlers.ListProviders(w, r)
	}))
	mux.HandleFunc("/api/providers/", s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		s.providerHandlers.HandleProviderByName(w, r)
	}))
}

// SetProviderManagerAPI sets the provider manager for lifecycle operations.
func (s *Server) SetProviderManagerAPI(api ProviderManagerAPI) {
	s.providerHandlers.SetManager(api)
}

// SetChannelModeAPI sets the channel mode API for detail mode management.
func (s *Server) SetChannelModeAPI(api ChannelModeAPI) {
	s.providerHandlers.SetModeAPI(api)
}

// ProviderStore returns the provider store.
func (s *Server) ProviderStore() *ProviderStore {
	return s.providerHandlers.store
}

// handleVersion returns the application version.
func handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"version": version.Get()})
}

// handleSecretByName routes /api/secrets/:name requests.
func (s *Server) handleSecretByName(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		s.secretHandlers.UpdateSecret(w, r)
	case http.MethodDelete:
		s.secretHandlers.DeleteSecret(w, r)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}
