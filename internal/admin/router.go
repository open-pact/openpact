package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	version "github.com/open-pact/openpact"
	"github.com/stack-bound/stackllm/session"
)

// Config holds the admin server configuration.
type Config struct {
	Bind          string
	DataDir       string
	ScriptsDir    string
	WorkspacePath string
	AIDataDir     string
	Allowlist     []string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		Bind:          "localhost:8080",
		DataDir:       "./data",
		ScriptsDir:    "./scripts",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 72 * time.Hour,
	}
}

// Server is the admin HTTP server.
type Server struct {
	config           Config
	users            *UserStore
	scripts          *ScriptStore
	jwt              *JWTManager
	setupHandler     *SetupHandler
	sessionHandler   *SessionHandler
	scriptHandlers   *ScriptHandlers
	secretHandlers   *SecretHandlers
	providerHandlers *ProviderHandlers
	scheduleStore    *ScheduleStore
	scheduleHandlers *ScheduleHandlers
	configHandlers   *ConfigHandlers
	engineHandler    http.Handler // stackllm web.ManagedHandler, mounted under /api/engine/
	engineSessions   *EngineSessionsHandler
	sessionStore     session.SessionStore // set alongside engineHandler
	secureCookie     bool
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

	users, err := NewUserStore(config.DataDir)
	if err != nil {
		return nil, err
	}

	scripts, err := NewScriptStore(config.ScriptsDir, config.DataDir, config.Allowlist)
	if err != nil {
		return nil, err
	}

	secureCookie := ShouldUseSecureCookies(config.Bind)

	secretStore := NewSecretStore(config.DataDir)
	providerStore := NewProviderStore(config.DataDir)
	scheduleStore := NewScheduleStore(config.DataDir)

	s := &Server{
		config:           config,
		users:            users,
		scripts:          scripts,
		jwt:              jwt,
		setupHandler:     NewSetupHandler(users, config.DataDir, config.AIDataDir, jwt, secureCookie, nil),
		sessionHandler:   NewSessionHandler(users, jwt, secureCookie),
		scriptHandlers:   NewScriptHandlers(scripts),
		secretHandlers:   NewSecretHandlers(secretStore, nil),
		providerHandlers: NewProviderHandlers(providerStore),
		scheduleStore:    scheduleStore,
		scheduleHandlers: NewScheduleHandlers(scheduleStore),
		configHandlers:   NewConfigHandlers(config.DataDir),
		secureCookie:     secureCookie,
	}
	s.engineSessions = NewEngineSessionsHandler(s.getSessionStore)
	return s, nil
}

// getSessionStore returns the live session store (nil before SetEngineHandler
// has been called). Captured by EngineSessionsHandler so it resolves the
// store at request time rather than at server-construction time.
func (s *Server) getSessionStore() session.SessionStore {
	return s.sessionStore
}

// SetEngineHandler installs the stackllm web.ManagedHandler. Once set, it
// is mounted at /api/engine/ and drives provider login, model selection
// and SSE chat. Called from main after the orchestrator builds the stack.
func (s *Server) SetEngineHandler(h http.Handler) {
	s.engineHandler = h
}

// SetSessionStore wires the session store that backs GET /api/engine/sessions.
// Called from main alongside SetEngineHandler.
func (s *Server) SetSessionStore(store session.SessionStore) {
	s.sessionStore = store
}

// SetDefaultModelCheck wires the predicate the setup handler uses to
// gate POST /api/setup/provider. The admin package deliberately does
// not import stackllm's profile package, so the engine wiring in
// cmd/openpact passes this bool-returning closure in at startup.
func (s *Server) SetDefaultModelCheck(fn func() bool) {
	if s.setupHandler != nil {
		s.setupHandler.defaultModelSet = fn
	}
}

// Handler returns the HTTP handler for the admin API (no SPA).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerAPIRoutes(mux)
	return RequireSetupMiddleware(s.users, s.config.DataDir)(mux)
}

// registerAPIRoutes registers every /api/* handler on the mux. Shared by
// Handler() and HandlerWithUI() so the dual-handler footgun (adding a
// route to one but not the other) can never happen.
func (s *Server) registerAPIRoutes(mux *http.ServeMux) {
	// Version endpoint (no auth required)
	mux.HandleFunc("/api/version", handleVersion)

	// Setup endpoints. /api/setup runs unauthenticated (no user exists
	// yet) and issues a refresh cookie; the remaining wizard steps
	// require auth, same as every other admin endpoint.
	mux.HandleFunc("/api/setup/status", s.setupHandler.Status)
	mux.HandleFunc("/api/setup/profile", s.withAuth(s.setupHandler.Profile))
	mux.HandleFunc("/api/setup/provider", s.withAuth(s.setupHandler.Provider))
	mux.HandleFunc("/api/setup", s.setupHandler.Setup)

	// Auth endpoints (no auth required)
	mux.HandleFunc("/api/auth/login", s.sessionHandler.Login)
	mux.HandleFunc("/api/auth/logout", s.sessionHandler.Logout)
	mux.HandleFunc("/api/session", s.sessionHandler.Session)

	// Protected endpoints (require auth)
	mux.HandleFunc("/api/auth/me", s.withAuth(s.sessionHandler.Me))
	mux.HandleFunc("/api/scripts", s.withAuth(s.handleScripts))
	mux.HandleFunc("/api/scripts/", s.withAuth(s.handleScriptByName))

	// Secret management endpoints
	mux.HandleFunc("/api/secrets", s.withAuth(s.handleSecrets))
	mux.HandleFunc("/api/secrets/", s.withAuth(s.handleSecretByName))

	// Provider management endpoints
	s.registerProviderRoutes(mux)

	// Schedule management endpoints
	s.registerScheduleRoutes(mux)

	// Advanced settings + integrations (engine + provider login live
	// under /api/engine/ via the stackllm web.ManagedHandler).
	mux.HandleFunc("/api/config/advanced", s.withAuth(s.configHandlers.HandleAdvanced))
	mux.HandleFunc("/api/config/integrations", s.withAuth(s.configHandlers.HandleIntegrations))

	// Paginated session list. Sits in front of the /api/engine/ subtree
	// mount so the fixed path wins over the prefix match. Always
	// auth-gated — the list is post-setup only, never public.
	mux.HandleFunc("/api/engine/sessions", s.withAuth(s.engineSessions.List))

	// Stackllm ManagedHandler mount. Always requires a bearer token —
	// even during the setup wizard, because POST /api/setup now issues
	// a refresh cookie and the wizard calls /api/session to pick up an
	// access token before step 3. The setup middleware is what gates
	// the path-level 503, not authentication.
	//
	// IMPORTANT (dual-handler rule, CLAUDE.md): this mount must appear
	// in both Handler() and HandlerWithUI(). registerAPIRoutes is the
	// single source of truth that both call.
	mux.HandleFunc("/api/engine/", s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		if s.engineHandler == nil {
			http.Error(w, `{"error":"engine handler not wired"}`, http.StatusServiceUnavailable)
			return
		}
		http.StripPrefix("/api/engine", s.engineHandler).ServeHTTP(w, r)
	}))
}

// withAuth wraps a handler with authentication middleware.
func (s *Server) withAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func (s *Server) Users() *UserStore { return s.users }

// Scripts returns the script store.
func (s *Server) Scripts() *ScriptStore { return s.scripts }

// SecretStore returns the secret store.
func (s *Server) SecretStore() *SecretStore { return s.secretHandlers.store }

// SetOnSecretsChanged sets the callback for secret changes.
func (s *Server) SetOnSecretsChanged(fn func()) {
	s.secretHandlers.onChange = fn
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
func (s *Server) ProviderStore() *ProviderStore { return s.providerHandlers.store }

// ScheduleStore returns the schedule store.
func (s *Server) ScheduleStore() *ScheduleStore { return s.scheduleStore }

// SetSchedulerAPI sets the scheduler API for schedule management.
func (s *Server) SetSchedulerAPI(api SchedulerAPI) {
	s.scheduleHandlers.SetSchedulerAPI(api)
}

// registerScheduleRoutes adds schedule management routes to a mux.
func (s *Server) registerScheduleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/schedules", s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.scheduleHandlers.ListSchedules(w, r)
		case http.MethodPost:
			s.scheduleHandlers.CreateSchedule(w, r)
		default:
			http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/schedules/", s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		s.scheduleHandlers.HandleScheduleByID(w, r)
	}))
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
