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
	subFS, err := fs.Sub(adminui.DistFS, "dist")
	if err != nil {
		return nil, err
	}

	_, err = fs.Stat(subFS, "index.html")
	built := err == nil

	return &SPAHandler{
		staticFS:   http.FileServer(http.FS(subFS)),
		fileServer: subFS,
		built:      built,
	}, nil
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.built {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(placeholderHTML))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	_, err := fs.Stat(h.fileServer, path)
	if err != nil {
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
//
// DUAL HANDLER RULE (CLAUDE.md): API routes are registered by calling
// registerAPIRoutes, the same method Handler() uses. Adding a route to
// one but not the other is structurally impossible.
func (s *Server) HandlerWithUI() (http.Handler, error) {
	spaHandler, err := NewSPAHandler()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	s.registerAPIRoutes(mux)

	// SPA fallback — must come after every /api/* route so the prefix
	// match on specific API paths wins over the root catch-all.
	mux.Handle("/", spaHandler)

	return RequireSetupMiddleware(s.users, s.config.DataDir)(mux), nil
}
