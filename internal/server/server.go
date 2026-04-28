package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/anthropics/symphony/internal/config"
	"github.com/anthropics/symphony/internal/domain"
	"github.com/anthropics/symphony/internal/persona"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// SnapshotFunc requests a state snapshot from the orchestrator.
type SnapshotFunc func() *domain.Snapshot

// RefreshFunc triggers an immediate poll cycle.
type RefreshFunc func() domain.RefreshResponse

// Server is the HTTP server for the observability API and dashboard.
// SPEC Section 13.7.
type Server struct {
	config      func() *config.Config
	snapshot    SnapshotFunc
	refresh     RefreshFunc
	personaReg  *persona.Registry
	personaStore *persona.AssignmentStore
	httpServer  *http.Server
}

// New creates an HTTP server with all routes configured.
func New(
	cfgFn func() *config.Config,
	snapshotFn SnapshotFunc,
	refreshFn RefreshFunc,
	personaReg *persona.Registry,
	personaStore *persona.AssignmentStore,
) *Server {
	s := &Server{
		config:       cfgFn,
		snapshot:     snapshotFn,
		refresh:      refreshFn,
		personaReg:   personaReg,
		personaStore: personaStore,
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// JSON API. SPEC Section 13.7.2.
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/state", s.handleState)
		r.Post("/refresh", s.handleRefresh)

		// Persona CRUD. SPEC Appendix B.10.3.
		r.Get("/personas", s.handleListPersonas)
		r.Post("/personas", s.handleCreatePersona)
		r.Get("/personas/assignments", s.handleListAssignments)
		r.Get("/personas/{name}", s.handleGetPersona)
		r.Put("/personas/{name}", s.handleUpdatePersona)
		r.Delete("/personas/{name}", s.handleDeletePersona)

		// Issue detail — must be after /personas routes.
		r.Get("/{issue_identifier}", s.handleIssue)
	})

	// Serve frontend SPA for all other routes.
	r.Mount("/", frontendHandler())

	s.httpServer = &http.Server{
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start begins serving HTTP. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	slog.Info("HTTP server started", "addr", listener.Addr().String())

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(shutdownCtx)
	}()

	if err := s.httpServer.Serve(listener); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// --- API Handlers ---

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	snap := s.snapshot()
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	resp := s.refresh()
	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Server) handleIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "issue_identifier")
	snap := s.snapshot()

	// Search running sessions.
	for _, entry := range snap.Running {
		if entry.Identifier == identifier {
			writeJSON(w, http.StatusOK, map[string]any{
				"issue_identifier": entry.Identifier,
				"issue_id":         entry.IssueID,
				"status":           "running",
				"workspace":        map[string]any{"path": entry.WorkspacePath, "host": entry.WorkerHost},
				"running": map[string]any{
					"session_id":    entry.SessionID,
					"turn_count":    entry.TurnCount,
					"state":         entry.State,
					"persona":       entry.Persona,
					"started_at":    entry.StartedAt,
					"last_event":    entry.LastEvent,
					"last_message":  entry.LastMessage,
					"last_event_at": entry.LastEventAt,
					"tokens":        entry.Tokens,
				},
				"retry": nil,
			})
			return
		}
	}

	// Search retry queue.
	for _, entry := range snap.Retrying {
		if entry.Identifier == identifier {
			writeJSON(w, http.StatusOK, map[string]any{
				"issue_identifier": entry.Identifier,
				"issue_id":         entry.IssueID,
				"status":           "retrying",
				"running":          nil,
				"retry": map[string]any{
					"attempt": entry.Attempt,
					"due_at":  entry.DueAt,
					"error":   entry.Error,
				},
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "issue_not_found",
		fmt.Sprintf("issue %q not found in current state", identifier))
}

// --- Persona Handlers ---

func (s *Server) handleListPersonas(w http.ResponseWriter, r *http.Request) {
	personas := s.personaReg.List()
	items := make([]map[string]any, 0, len(personas))
	for _, p := range personas {
		items = append(items, map[string]any{
			"name":        p.Name,
			"description": p.Description,
			"source_path": p.SourcePath,
			"overrides":   p.Overrides,
			"has_prompt":  p.PromptTemplate != "",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"personas": items})
}

func (s *Server) handleGetPersona(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	p := s.personaReg.Get(name)
	if p == nil {
		writeError(w, http.StatusNotFound, "persona_not_found",
			fmt.Sprintf("persona %q not found", name))
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleCreatePersona(w http.ResponseWriter, r *http.Request) {
	var req persona.Persona
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if err := s.personaReg.Create(&req); err != nil {
		if isConflict(err) {
			writeError(w, http.StatusConflict, "persona_exists", err.Error())
		} else {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handleUpdatePersona(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req persona.Persona
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if err := s.personaReg.Update(name, &req); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "persona_not_found", err.Error())
		} else {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		}
		return
	}

	updated := s.personaReg.Get(name)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeletePersona(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.personaReg.Delete(name); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "persona_not_found", err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) handleListAssignments(w http.ResponseWriter, r *http.Request) {
	all := s.personaStore.All()
	items := make([]map[string]any, 0, len(all))
	for issueID, a := range all {
		items = append(items, map[string]any{
			"issue_id":     issueID,
			"persona_name": a.PersonaName,
			"assigned_at":  a.AssignedAt,
			"source":       a.Source,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"assignments": items})
}


// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func isConflict(err error) bool {
	return err != nil && len(err.Error()) > 0 && err.Error()[len(err.Error())-1] != 0 &&
		contains(err.Error(), "already exists")
}

func isNotFound(err error) bool {
	return err != nil && contains(err.Error(), "not found")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

const fallbackHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Symphony Dashboard</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
           background: #0a0a0f; color: #e4e4e7; }
  </style>
</head>
<body>
  <div id="root"></div>
  <script type="module" src="/assets/index.js"></script>
</body>
</html>`
