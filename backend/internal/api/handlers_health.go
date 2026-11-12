package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/oyins/oddsvault/backend/internal/database"
)

// BuildInfo describes the running binary.
type BuildInfo struct {
	Version    string `json:"version"`
	CommitTime string `json:"commit_time,omitempty"`
	Dirty      bool   `json:"dirty,omitempty"`
}

func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"service":    s.service,
		"env":        s.env,
		"version":    s.version.Version,
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
	})
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	type check struct {
		Status    string `json:"status"`
		LatencyMS int64  `json:"latency_ms"`
		Error     string `json:"error,omitempty"`
	}
	checks := map[string]check{}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	start := time.Now()
	status := http.StatusOK
	if s.pool == nil {
		checks["database"] = check{Status: "not_configured"}
		status = http.StatusServiceUnavailable
	} else if err := database.Ping(ctx, s.pool, 2*time.Second); err != nil {
		checks["database"] = check{
			Status:    "unreachable",
			LatencyMS: time.Since(start).Milliseconds(),
			Error:     err.Error(),
		}
		status = http.StatusServiceUnavailable
	} else {
		checks["database"] = check{
			Status:    "ok",
			LatencyMS: time.Since(start).Milliseconds(),
		}
	}

	overall := "ready"
	if status != http.StatusOK {
		overall = "not_ready"
	}
	writeJSON(w, status, map[string]any{
		"status":  overall,
		"checks":  checks,
		"service": s.service,
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":    s.service,
		"env":        s.env,
		"build":      s.version,
		"started_at": s.startedAt.Format(time.RFC3339),
	})
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotFound, "not_found", "no route matches "+r.Method+" "+r.URL.Path)
}

// requestLogger is used by the access-log middleware.
func (s *Server) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}
