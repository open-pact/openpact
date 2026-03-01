package admin

import (
	"io/fs"
	"net/http"
	"strings"

	adminui "github.com/open-pact/openpact/admin-ui"
)

// SPAHandler serves the embedded Vue SPA and falls back to index.html for client-side routing.
type SPAHandler struct {
	staticFS   http.Handler
	fileServer fs.FS
	built      bool
}

// NewSPAHandler creates a new SPA handler from the embedded filesystem.
func NewSPAHandler() (*SPAHandler, error) {
	// Get the dist subdirectory
	subFS, err := fs.Sub(adminui.DistFS, "dist")
	if err != nil {
		return nil, err
	}

	// Check if the admin UI has actually been built
	_, err = fs.Stat(subFS, "index.html")
	built := err == nil

	return &SPAHandler{
		staticFS:   http.FileServer(http.FS(subFS)),
		fileServer: subFS,
		built:      built,
	}, nil
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If the admin UI hasn't been built, show a placeholder
	if !h.built {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(placeholderHTML))
		return
	}

	// Try to serve the file directly
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Check if the file exists
	_, err := fs.Stat(h.fileServer, path)
	if err != nil {
		// File doesn't exist, serve index.html for SPA routing
		r.URL.Path = "/"
	}

	h.staticFS.ServeHTTP(w, r)
}

const placeholderHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>OpenPact Admin UI</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
      margin: 0;
      background: #f5f5f5;
      color: #333;
    }
    .container { text-align: center; max-width: 480px; padding: 2rem; }
    h1 { font-size: 1.5rem; margin-bottom: 0.5rem; }
    p { line-height: 1.6; color: #666; }
    pre {
      background: #1e1e1e;
      color: #d4d4d4;
      padding: 1rem;
      border-radius: 8px;
      text-align: left;
      overflow-x: auto;
    }
  </style>
</head>
<body>
  <div class="container">
    <h1>Admin UI Not Built</h1>
    <p>The admin frontend has not been compiled yet. Build it with:</p>
    <pre>cd admin-ui
npm install
npm run build</pre>
    <p>Then restart the server.</p>
  </div>
</body>
</html>`

// HandlerWithUI returns the HTTP handler with both API and embedded UI.
func (s *Server) HandlerWithUI() (http.Handler, error) {
	spaHandler, err := NewSPAHandler()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	// API routes (must come first due to path matching)
	mux.HandleFunc("/api/version", handleVersion)
	mux.HandleFunc("/api/setup/status", s.setupHandler.Status)
	mux.HandleFunc("/api/setup/profile", s.setupHandler.Profile)
	mux.HandleFunc("/api/setup", s.setupHandler.Setup)
	mux.HandleFunc("/api/auth/login", s.sessionHandler.Login)
	mux.HandleFunc("/api/auth/logout", s.sessionHandler.Logout)
	mux.HandleFunc("/api/session", s.sessionHandler.Session)
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

	// Schedule management endpoints
	s.registerScheduleRoutes(mux)

	// Static files and SPA fallback
	mux.Handle("/", spaHandler)

	// Apply setup middleware
	return RequireSetupMiddleware(s.users, s.config.DataDir)(mux), nil
}
