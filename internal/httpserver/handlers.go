package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, []string{http.MethodGet})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html><head><title>Symphony</title></head><body><h1>Symphony</h1><p>Use /api/v1/state for JSON state.</p></body></html>`))
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, []string{http.MethodGet})
		return
	}
	snap := s.orchestrator.Snapshot()
	response := map[string]any{
		"generated_at": snap.GeneratedAt,
		"counts": map[string]any{
			"running":  len(snap.Running),
			"retrying": len(snap.Retrying),
		},
		"running":      snap.Running,
		"retrying":     snap.Retrying,
		"codex_totals": snap.CodexTotals,
		"rate_limits":  snap.RateLimits,
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, []string{http.MethodGet})
		return
	}
	identifier := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		s.writeError(w, http.StatusNotFound, "issue_not_found", "issue identifier missing")
		return
	}
	details, ok := s.orchestrator.IssueDetails(identifier)
	if !ok {
		s.writeError(w, http.StatusNotFound, "issue_not_found", "issue not found")
		return
	}
	s.writeJSON(w, http.StatusOK, details)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, []string{http.MethodPost})
		return
	}
	s.orchestrator.RequestRefresh()
	s.writeJSON(w, http.StatusAccepted, map[string]any{
		"queued":       true,
		"coalesced":    false,
		"requested_at": time.Now().UTC(),
		"operations":   []string{"poll", "reconcile"},
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) writeError(w http.ResponseWriter, status int, code string, message string) {
	s.writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func (s *Server) writeMethodNotAllowed(w http.ResponseWriter, allow []string) {
	w.Header().Set("Allow", strings.Join(allow, ", "))
	s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}
