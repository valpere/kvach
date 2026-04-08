// Package server implements the HTTP API that exposes agent sessions to
// external clients (IDE extensions, web UIs, remote CLIs).
//
// Architecture:
//   - net/http with chi router
//   - SSE (Server-Sent Events) for streaming session events
//   - WebSocket for PTY (interactive terminal) sessions
//   - Optional HTTP Basic Auth via config
//
// Every request is scoped to a working directory, passed via the
// X-Kvach-Directory header or the ?dir query parameter. This allows one
// server process to serve multiple projects.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/valpere/kvach/internal/config"
	"github.com/valpere/kvach/internal/git"
	"github.com/valpere/kvach/internal/session"
)

// Server is the HTTP API server.
type Server struct {
	cfg      config.ServerConfig
	workDir  string
	sessions session.Store
	router   http.Handler
}

// Options configures server dependencies.
type Options struct {
	WorkDir      string
	SessionStore session.Store
}

// New creates a Server with the given configuration.
func New(cfg config.ServerConfig, opts Options) *Server {
	s := &Server{cfg: cfg, workDir: opts.WorkDir, sessions: opts.SessionStore}
	s.router = s.buildRouter()
	return s
}

// ListenAndServe starts the HTTP server and blocks until it returns an error.
func (s *Server) ListenAndServe() error {
	addr := s.addr()
	h := s.router
	if strings.TrimSpace(s.cfg.Password) != "" {
		h = s.withBasicAuth(h)
	}
	return http.ListenAndServe(addr, h)
}

func (s *Server) addr() string {
	host := s.cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := s.cfg.Port
	if port == 0 {
		port = 7777
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// Addr returns the resolved host:port bind address.
func (s *Server) Addr() string {
	return s.addr()
}

func (s *Server) buildRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /project", s.handleProject)
	mux.HandleFunc("GET /config", s.handleConfig)
	mux.HandleFunc("GET /session", s.handleSessionList)
	mux.HandleFunc("GET /session/{id}", s.handleSessionGet)
	mux.HandleFunc("DELETE /session/{id}", s.handleSessionArchive)
	mux.HandleFunc("GET /session/{id}/events", s.handleSessionEvents)
	return mux
}

func (s *Server) withBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, password, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="kvach"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	workDir := s.requestWorkDir(r)
	projectID := git.SlugFromRoot(workDir)
	if root, err := git.Root(r.Context(), workDir); err == nil {
		projectID = git.SlugFromRoot(root)
		workDir = root
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"projectID": projectID,
		"directory": workDir,
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"host":               s.cfg.Host,
		"port":               s.cfg.Port,
		"password_protected": strings.TrimSpace(s.cfg.Password) != "",
	})
}

func (s *Server) handleSessionList(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}

	projectID := s.projectIDForRequest(r)
	items, err := s.sessions.ListSessions(r.Context(), projectID)
	if err != nil {
		http.Error(w, fmt.Sprintf("list sessions: %v", err), http.StatusInternalServerError)
		return
	}

	type sessionItem struct {
		ID        string     `json:"id"`
		ProjectID string     `json:"projectID"`
		Directory string     `json:"directory"`
		Title     string     `json:"title"`
		UpdatedAt time.Time  `json:"updatedAt"`
		Archived  *time.Time `json:"archivedAt,omitempty"`
	}
	out := make([]sessionItem, 0, len(items))
	for _, sess := range items {
		out = append(out, sessionItem{
			ID:        sess.ID,
			ProjectID: sess.ProjectID,
			Directory: sess.Directory,
			Title:     sess.Title,
			UpdatedAt: sess.UpdatedAt,
			Archived:  sess.ArchivedAt,
		})
	}

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "session id is required", http.StatusBadRequest)
		return
	}

	sess, err := s.sessions.GetSession(r.Context(), id)
	if err != nil {
		if err == session.ErrNotFound {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("get session: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleSessionArchive(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "session id is required", http.StatusBadRequest)
		return
	}

	if err := s.sessions.ArchiveSession(r.Context(), id); err != nil {
		if err == session.ErrNotFound {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("archive session: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "archived": true})
}

func (s *Server) handleSessionEvents(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "session id is required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	_, _ = fmt.Fprintf(w, "event: session.ready\n")
	_, _ = fmt.Fprintf(w, "data: {\"sessionID\":%q}\n\n", id)
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, "event: ping\n")
			_, _ = fmt.Fprintf(w, "data: {\"ts\":%d}\n\n", time.Now().UTC().Unix())
			flusher.Flush()
		}
	}
}

func (s *Server) requestWorkDir(r *http.Request) string {
	if d := strings.TrimSpace(r.URL.Query().Get("dir")); d != "" {
		return d
	}
	if d := strings.TrimSpace(r.Header.Get("X-Kvach-Directory")); d != "" {
		return d
	}
	if strings.TrimSpace(s.workDir) != "" {
		return s.workDir
	}
	if wd, err := osGetwd(); err == nil {
		return wd
	}
	return "."
}

func (s *Server) projectIDForRequest(r *http.Request) string {
	workDir := s.requestWorkDir(r)
	if root, err := git.Root(r.Context(), workDir); err == nil {
		return git.SlugFromRoot(root)
	}
	return git.SlugFromRoot(workDir)
}

var osGetwd = func() (string, error) {
	return os.Getwd()
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
