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
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/kvach/internal/agent"
	"github.com/valpere/kvach/internal/config"
	"github.com/valpere/kvach/internal/git"
	"github.com/valpere/kvach/internal/provider"
	anthropicProvider "github.com/valpere/kvach/internal/provider/anthropic"
	googleProvider "github.com/valpere/kvach/internal/provider/google"
	openaiProvider "github.com/valpere/kvach/internal/provider/openai"
	"github.com/valpere/kvach/internal/session"
	"github.com/valpere/kvach/internal/tool"
)

// Server is the HTTP API server.
type Server struct {
	cfg      config.ServerConfig
	workDir  string
	sessions session.Store
	router   http.Handler

	newAgent AgentFactory

	mu     sync.RWMutex
	nextID uint64
	subs   map[string]map[uint64]chan streamedEvent

	runMu   sync.RWMutex
	runs    map[string]activePromptRun
	history map[string][]runInfo
}

// Options configures server dependencies.
type Options struct {
	WorkDir      string
	SessionStore session.Store
	AgentFactory AgentFactory
}

// AgentRunner is the subset of agent.Agent used by the server.
type AgentRunner interface {
	Run(ctx context.Context, opts agent.RunOptions) (<-chan agent.Event, error)
}

// AgentFactoryArgs are inputs for creating an AgentRunner.
type AgentFactoryArgs struct {
	WorkDir string
	Model   string
	Config  *config.Config
}

// AgentFactory creates an agent runner for request-scoped execution.
type AgentFactory func(ctx context.Context, args AgentFactoryArgs) (AgentRunner, error)

type streamedEvent struct {
	Name string
	Data any
}

type activePromptRun struct {
	RunID     string
	Cancel    context.CancelFunc
	Started   time.Time
	Model     string
	Prompt    string
	SessionID string
}

type runStatus string

const (
	runStatusRunning   runStatus = "running"
	runStatusCompleted runStatus = "completed"
	runStatusFailed    runStatus = "failed"
	runStatusCancelled runStatus = "cancelled"
	maxRunHistory                = 100
)

type runInfo struct {
	RunID         string          `json:"run_id"`
	SessionID     string          `json:"session_id"`
	Status        runStatus       `json:"status"`
	Model         string          `json:"model"`
	Prompt        string          `json:"prompt"`
	OutputPreview string          `json:"output_preview,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty"`
	FinishReason  string          `json:"finish_reason,omitempty"`
	Error         string          `json:"error,omitempty"`
	Usage         agent.UsageInfo `json:"usage"`
}

// New creates a Server with the given configuration.
func New(cfg config.ServerConfig, opts Options) *Server {
	factory := opts.AgentFactory
	if factory == nil {
		factory = defaultAgentFactory(opts.SessionStore)
	}

	s := &Server{
		cfg:      cfg,
		workDir:  opts.WorkDir,
		sessions: opts.SessionStore,
		newAgent: factory,
		subs:     make(map[string]map[uint64]chan streamedEvent),
		runs:     make(map[string]activePromptRun),
		history:  make(map[string][]runInfo),
	}
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
	mux.HandleFunc("GET /provider", s.handleProviderList)
	mux.HandleFunc("GET /provider/{id}", s.handleProviderGet)
	mux.HandleFunc("GET /provider/{id}/models", s.handleProviderModels)
	mux.HandleFunc("GET /session", s.handleSessionList)
	mux.HandleFunc("POST /session", s.handleSessionCreate)
	mux.HandleFunc("GET /session/{id}", s.handleSessionGet)
	mux.HandleFunc("DELETE /session/{id}", s.handleSessionArchive)
	mux.HandleFunc("GET /session/{id}/runs", s.handleSessionRuns)
	mux.HandleFunc("GET /session/{id}/messages", s.handleSessionMessages)
	mux.HandleFunc("POST /session/{id}/prompt", s.handleSessionPrompt)
	mux.HandleFunc("POST /session/{id}/cancel", s.handleSessionCancel)
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

type providerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type providerDetail struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	ModelCount int              `json:"model_count"`
	Models     []provider.Model `json:"models"`
}

func (s *Server) handleProviderList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []providerInfo{
		{ID: "anthropic", Name: "Anthropic"},
		{ID: "openai", Name: "OpenAI"},
		{ID: "google", Name: "Google"},
		{ID: "groq", Name: "Groq"},
		{ID: "openrouter", Name: "OpenRouter"},
		{ID: "together", Name: "Together"},
	})
}

func (s *Server) handleProviderGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "provider id is required", http.StatusBadRequest)
		return
	}

	p, err := providerFromID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	models, err := p.Models(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list models: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, providerDetail{
		ID:         p.ID(),
		Name:       p.Name(),
		ModelCount: len(models),
		Models:     models,
	})
}

func (s *Server) handleProviderModels(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "provider id is required", http.StatusBadRequest)
		return
	}

	p, err := providerFromID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	models, err := p.Models(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list models: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, models)
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

type createSessionRequest struct {
	Title     string `json:"title"`
	ParentID  string `json:"parent_id,omitempty"`
	Directory string `json:"directory,omitempty"`
}

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}

	var req createSessionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	workDir := s.requestWorkDir(r)
	if strings.TrimSpace(req.Directory) != "" {
		workDir = strings.TrimSpace(req.Directory)
	}
	if root, err := git.Root(r.Context(), workDir); err == nil {
		workDir = root
	}

	now := time.Now().UTC()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New session"
	}
	sess := session.Session{
		ID:        uuid.NewString(),
		ProjectID: git.SlugFromRoot(workDir),
		Directory: workDir,
		Title:     title,
		ParentID:  strings.TrimSpace(req.ParentID),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.sessions.CreateSession(r.Context(), sess); err != nil {
		http.Error(w, fmt.Sprintf("create session: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, sess)
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

type runsResponse struct {
	SessionID string    `json:"session_id"`
	Status    string    `json:"status,omitempty"`
	Limit     int       `json:"limit"`
	Offset    int       `json:"offset"`
	Total     int       `json:"total"`
	Count     int       `json:"count"`
	Active    *runInfo  `json:"active,omitempty"`
	Runs      []runInfo `json:"runs"`
}

type runsQuery struct {
	Status string
	Limit  int
	Offset int
}

func (s *Server) handleSessionRuns(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "session id is required", http.StatusBadRequest)
		return
	}

	if s.sessions != nil {
		if _, err := s.sessions.GetSession(r.Context(), id); err != nil {
			if err == session.ErrNotFound {
				http.Error(w, "session not found", http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf("get session: %v", err), http.StatusInternalServerError)
			return
		}
	}

	q, err := parseRunsQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	active, runs := s.listRuns(id)
	if q.Status != "" && (active == nil || string(active.Status) != q.Status) {
		active = nil
	}

	filtered := filterRunsByStatus(runs, q.Status)
	total := len(filtered)
	paged := paginateRuns(filtered, q.Offset, q.Limit)

	writeJSON(w, http.StatusOK, runsResponse{
		SessionID: id,
		Status:    q.Status,
		Limit:     q.Limit,
		Offset:    q.Offset,
		Total:     total,
		Count:     len(paged),
		Active:    active,
		Runs:      paged,
	})
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

type messageEnvelope struct {
	Message session.Message `json:"message"`
	Parts   []partEnvelope  `json:"parts"`
}

type partEnvelope struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Data      any       `json:"data"`
	CreatedAt time.Time `json:"createdAt"`
	Raw       string    `json:"raw,omitempty"`
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "session id is required", http.StatusBadRequest)
		return
	}

	if _, err := s.sessions.GetSession(r.Context(), id); err != nil {
		if err == session.ErrNotFound {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("get session: %v", err), http.StatusInternalServerError)
		return
	}

	msgs, err := s.sessions.GetMessages(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get messages: %v", err), http.StatusInternalServerError)
		return
	}

	out := make([]messageEnvelope, 0, len(msgs))
	for _, msg := range msgs {
		parts, err := s.sessions.GetParts(r.Context(), msg.ID)
		if err != nil {
			http.Error(w, fmt.Sprintf("get parts: %v", err), http.StatusInternalServerError)
			return
		}

		envParts := make([]partEnvelope, 0, len(parts))
		for _, p := range parts {
			decoded, raw := decodePartData(p)
			envParts = append(envParts, partEnvelope{
				ID:        p.ID,
				Type:      string(p.Type),
				Data:      decoded,
				CreatedAt: p.CreatedAt,
				Raw:       raw,
			})
		}

		out = append(out, messageEnvelope{Message: msg, Parts: envParts})
	}

	writeJSON(w, http.StatusOK, out)
}

type promptRequest struct {
	Prompt   string `json:"prompt"`
	Model    string `json:"model,omitempty"`
	MaxTurns int    `json:"max_turns,omitempty"`
}

type promptResponse struct {
	RunID        string          `json:"run_id"`
	SessionID    string          `json:"session_id"`
	Output       string          `json:"output"`
	FinishReason string          `json:"finish_reason"`
	Usage        agent.UsageInfo `json:"usage"`
}

type cancelResponse struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
	Cancelled bool   `json:"cancelled"`
}

func (s *Server) handleSessionPrompt(w http.ResponseWriter, r *http.Request) {
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
	if sess.ArchivedAt != nil {
		http.Error(w, "session is archived", http.StatusConflict)
		return
	}

	var req promptRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	workDir := sess.Directory
	if strings.TrimSpace(workDir) == "" {
		workDir = s.requestWorkDir(r)
	}

	cfg, err := config.Load(workDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("load config: %v", err), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(req.Model) != "" {
		cfg.Model = strings.TrimSpace(req.Model)
	}
	if req.MaxTurns > 0 {
		cfg.MaxTurns = req.MaxTurns
	}

	runner, err := s.newAgent(r.Context(), AgentFactoryArgs{WorkDir: workDir, Model: cfg.Model, Config: cfg})
	if err != nil {
		http.Error(w, fmt.Sprintf("create agent: %v", err), http.StatusInternalServerError)
		return
	}

	runID := uuid.NewString()
	startedAt := time.Now().UTC()
	runCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	s.registerRun(id, activePromptRun{
		RunID:     runID,
		Cancel:    cancel,
		Started:   startedAt,
		Model:     cfg.Model,
		Prompt:    req.Prompt,
		SessionID: id,
	})
	defer s.clearRun(id, runID)

	events, err := runner.Run(runCtx, agent.RunOptions{Prompt: req.Prompt, SessionID: id})
	if err != nil {
		cancel()
		finishedAt := time.Now().UTC()
		s.appendRunHistory(id, runInfo{
			RunID:      runID,
			SessionID:  id,
			Status:     runStatusFailed,
			Model:      cfg.Model,
			Prompt:     req.Prompt,
			StartedAt:  startedAt,
			FinishedAt: &finishedAt,
			Error:      err.Error(),
		})
		http.Error(w, fmt.Sprintf("run agent: %v", err), http.StatusInternalServerError)
		return
	}

	streaming := shouldStreamPrompt(r)
	var flusher http.Flusher
	if streaming {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		var ok bool
		flusher, ok = w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
	}

	var (
		output       strings.Builder
		finishReason = ""
		usage        agent.UsageInfo
		agentErr     string
	)

	for evt := range events {
		envelope := toEventEnvelope(id, evt)
		s.publishEvent(id, string(evt.Type), envelope)
		if streaming {
			if err := writeSSE(w, string(evt.Type), envelope); err != nil {
				return
			}
			flusher.Flush()
		}

		switch evt.Type {
		case agent.EventTextDelta:
			if s, ok := evt.Payload.(string); ok {
				output.WriteString(s)
			}
		case agent.EventUsageUpdated:
			if u, ok := evt.Payload.(agent.UsageInfo); ok {
				usage = u
			}
		case agent.EventDone:
			if reason, ok := evt.Payload.(string); ok {
				finishReason = reason
			}
		case agent.EventError:
			if e, ok := evt.Payload.(string); ok {
				agentErr = e
			}
		}
	}

	status := runStatusCompleted
	if finishReason == string(agent.ReasonAborted) {
		status = runStatusCancelled
	}
	if agentErr != "" {
		status = runStatusFailed
	}
	finishedAt := time.Now().UTC()
	s.appendRunHistory(id, runInfo{
		RunID:         runID,
		SessionID:     id,
		Status:        status,
		Model:         cfg.Model,
		Prompt:        req.Prompt,
		OutputPreview: clamp(output.String(), 1000),
		StartedAt:     startedAt,
		FinishedAt:    &finishedAt,
		FinishReason:  finishReason,
		Error:         agentErr,
		Usage:         usage,
	})

	resp := promptResponse{
		RunID:        runID,
		SessionID:    id,
		Output:       output.String(),
		FinishReason: finishReason,
		Usage:        usage,
	}

	if streaming {
		if agentErr != "" {
			_ = writeSSE(w, "error", map[string]any{"message": agentErr})
			flusher.Flush()
			return
		}
		_ = writeSSE(w, "response.completed", resp)
		flusher.Flush()
		return
	}

	if agentErr != "" {
		http.Error(w, agentErr, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSessionCancel(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "session id is required", http.StatusBadRequest)
		return
	}

	run, ok := s.cancelRun(id)
	if !ok {
		http.Error(w, "no active run for session", http.StatusConflict)
		return
	}

	writeJSON(w, http.StatusOK, cancelResponse{
		SessionID: id,
		RunID:     run.RunID,
		Cancelled: true,
	})
}

func (s *Server) handleSessionEvents(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "session id is required", http.StatusBadRequest)
		return
	}
	if s.sessions != nil {
		if _, err := s.sessions.GetSession(r.Context(), id); err != nil {
			if err == session.ErrNotFound {
				http.Error(w, "session not found", http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf("get session: %v", err), http.StatusInternalServerError)
			return
		}
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

	ch, cancel := s.subscribeSession(id)
	defer cancel()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt := <-ch:
			if err := writeSSE(w, evt.Name, evt.Data); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			_ = writeSSE(w, "ping", map[string]any{"ts": time.Now().UTC().Unix()})
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

func decodeJSONBody(r *http.Request, out any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func decodePartData(p session.Part) (any, string) {
	switch p.Type {
	case session.PartTypeText:
		var d session.TextData
		if err := json.Unmarshal(p.Data, &d); err == nil {
			return d, ""
		}
	case session.PartTypeReasoning:
		var d session.ReasoningData
		if err := json.Unmarshal(p.Data, &d); err == nil {
			return d, ""
		}
	case session.PartTypeToolUse:
		var d session.ToolUseData
		if err := json.Unmarshal(p.Data, &d); err == nil {
			return d, ""
		}
	case session.PartTypeToolResult:
		var d session.ToolResultData
		if err := json.Unmarshal(p.Data, &d); err == nil {
			return d, ""
		}
	case session.PartTypeFile:
		var d session.FileData
		if err := json.Unmarshal(p.Data, &d); err == nil {
			return d, ""
		}
	case session.PartTypeCompaction:
		var d session.CompactionData
		if err := json.Unmarshal(p.Data, &d); err == nil {
			return d, ""
		}
	case session.PartTypeTodo:
		var d session.TodoData
		if err := json.Unmarshal(p.Data, &d); err == nil {
			return d, ""
		}
	}

	var generic any
	if err := json.Unmarshal(p.Data, &generic); err == nil {
		return generic, ""
	}
	return nil, string(p.Data)
}

func (s *Server) publishEvent(sessionID, name string, payload any) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}

	s.mu.RLock()
	subs := s.subs[sessionID]
	chans := make([]chan streamedEvent, 0, len(subs))
	for _, ch := range subs {
		chans = append(chans, ch)
	}
	s.mu.RUnlock()

	evt := streamedEvent{Name: name, Data: payload}
	for _, ch := range chans {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (s *Server) registerRun(sessionID string, run activePromptRun) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	s.runMu.Lock()
	if prev, ok := s.runs[sessionID]; ok && prev.Cancel != nil {
		prev.Cancel()
	}
	s.runs[sessionID] = run
	s.runMu.Unlock()
}

func (s *Server) clearRun(sessionID, runID string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	s.runMu.Lock()
	if run, ok := s.runs[sessionID]; ok {
		if runID == "" || run.RunID == runID {
			delete(s.runs, sessionID)
		}
	}
	s.runMu.Unlock()
}

func (s *Server) cancelRun(sessionID string) (activePromptRun, bool) {
	s.runMu.Lock()
	run, ok := s.runs[sessionID]
	if ok {
		delete(s.runs, sessionID)
	}
	s.runMu.Unlock()

	if !ok {
		return activePromptRun{}, false
	}
	if run.Cancel != nil {
		run.Cancel()
	}
	return run, true
}

func (s *Server) appendRunHistory(sessionID string, info runInfo) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}

	s.runMu.Lock()
	history := append([]runInfo{info}, s.history[sessionID]...)
	if len(history) > maxRunHistory {
		history = history[:maxRunHistory]
	}
	s.history[sessionID] = history
	s.runMu.Unlock()
}

func (s *Server) listRuns(sessionID string) (*runInfo, []runInfo) {
	s.runMu.RLock()
	defer s.runMu.RUnlock()

	var active *runInfo
	if run, ok := s.runs[sessionID]; ok {
		info := runInfo{
			RunID:     run.RunID,
			SessionID: run.SessionID,
			Status:    runStatusRunning,
			Model:     run.Model,
			Prompt:    run.Prompt,
			StartedAt: run.Started,
		}
		active = &info
	}

	history := append([]runInfo(nil), s.history[sessionID]...)
	return active, history
}

func parseRunsQuery(r *http.Request) (runsQuery, error) {
	q := runsQuery{Limit: 20, Offset: 0}

	status := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("status")))
	if status != "" {
		switch runStatus(status) {
		case runStatusRunning, runStatusCompleted, runStatusFailed, runStatusCancelled:
			q.Status = status
		default:
			return runsQuery{}, fmt.Errorf("invalid status %q", status)
		}
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return runsQuery{}, fmt.Errorf("invalid limit %q", raw)
		}
		if n > 200 {
			n = 200
		}
		q.Limit = n
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return runsQuery{}, fmt.Errorf("invalid offset %q", raw)
		}
		q.Offset = n
	}

	return q, nil
}

func filterRunsByStatus(runs []runInfo, status string) []runInfo {
	if status == "" {
		return append([]runInfo(nil), runs...)
	}
	out := make([]runInfo, 0, len(runs))
	for _, run := range runs {
		if string(run.Status) == status {
			out = append(out, run)
		}
	}
	return out
}

func paginateRuns(runs []runInfo, offset, limit int) []runInfo {
	if offset >= len(runs) {
		return nil
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = len(runs)
	}
	end := offset + limit
	if end > len(runs) {
		end = len(runs)
	}
	return append([]runInfo(nil), runs[offset:end]...)
}

func (s *Server) subscribeSession(sessionID string) (<-chan streamedEvent, func()) {
	ch := make(chan streamedEvent, 128)

	s.mu.Lock()
	id := s.nextID
	s.nextID++
	if s.subs[sessionID] == nil {
		s.subs[sessionID] = make(map[uint64]chan streamedEvent)
	}
	s.subs[sessionID][id] = ch
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		if subs, ok := s.subs[sessionID]; ok {
			if _, ok := subs[id]; ok {
				delete(subs, id)
			}
			if len(subs) == 0 {
				delete(s.subs, sessionID)
			}
		}
		s.mu.Unlock()
	}

	return ch, cancel
}

func writeSSE(w http.ResponseWriter, eventName string, data any) error {
	if strings.TrimSpace(eventName) == "" {
		eventName = "message"
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}
	return nil
}

func shouldStreamPrompt(r *http.Request) bool {
	v := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("stream")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	}
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	return strings.Contains(accept, "text/event-stream")
}

func toEventEnvelope(sessionID string, evt agent.Event) map[string]any {
	if evt.SessionID == "" {
		evt.SessionID = sessionID
	}
	return map[string]any{
		"type":       string(evt.Type),
		"session_id": evt.SessionID,
		"message_id": evt.MessageID,
		"part_id":    evt.PartID,
		"payload":    evt.Payload,
	}
}

func providerFromID(id string) (provider.Provider, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	switch id {
	case "anthropic":
		return anthropicProvider.New("", ""), nil
	case "openai":
		return openaiProvider.New(""), nil
	case "google", "gemini":
		return googleProvider.New(""), nil
	case "groq", "openrouter", "together":
		return openaiProvider.NewCompatible(id, strings.Title(id), "", ""), nil
	default:
		return nil, fmt.Errorf("provider %q not found", id)
	}
}

func defaultAgentFactory(store session.Store) AgentFactory {
	return func(_ context.Context, args AgentFactoryArgs) (AgentRunner, error) {
		cfg := args.Config
		if cfg == nil {
			loaded, err := config.Load(args.WorkDir)
			if err != nil {
				return nil, fmt.Errorf("load config: %w", err)
			}
			cfg = loaded
		}

		model := strings.TrimSpace(args.Model)
		if model == "" {
			model = cfg.Model
		}
		providerName, modelID := splitModel(model)

		p, err := providerFromID(providerName)
		if err != nil {
			return nil, err
		}

		systemPrompt := cfg.Instructions
		if systemPrompt == "" {
			systemPrompt = "You are kvach, an AI coding agent. You have access to tools for reading files, writing files, executing shell commands, and searching code. Use them to help the user."
		}

		a := agent.New(p, tool.DefaultRegistry, store, agent.Config{
			MaxTurns:     cfg.MaxTurns,
			WorkDir:      args.WorkDir,
			SystemPrompt: systemPrompt,
			Model:        modelID,
		})
		return a, nil
	}
}

func splitModel(model string) (providerName, modelID string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "anthropic", "claude-sonnet-4-5"
	}
	if i := strings.IndexByte(model, '/'); i > 0 {
		return strings.ToLower(model[:i]), model[i+1:]
	}
	if strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o") {
		return "openai", model
	}
	if strings.HasPrefix(model, "gemini") {
		return "google", model
	}
	return "anthropic", model
}

func clamp(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}
