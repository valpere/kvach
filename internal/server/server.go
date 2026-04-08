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
	"fmt"
	"net/http"

	"github.com/valpere/kvach/internal/config"
)

// Server is the HTTP API server.
type Server struct {
	cfg    config.ServerConfig
	router http.Handler
}

// New creates a Server with the given configuration.
func New(cfg config.ServerConfig) *Server {
	s := &Server{cfg: cfg}
	s.router = s.buildRouter()
	return s
}

// ListenAndServe starts the HTTP server and blocks until it returns an error.
func (s *Server) ListenAndServe() error {
	addr := s.addr()
	return http.ListenAndServe(addr, s.router)
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

func (s *Server) buildRouter() http.Handler {
	// TODO(phase3): wire up chi router with all route modules.
	return http.NewServeMux()
}
