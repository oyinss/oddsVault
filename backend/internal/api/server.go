// Package api implements the Go HTTP API.
//
// Endpoints are domain oriented (see the master plan, section 63). Phase 0
// only exposes operational endpoints; domain endpoints are added together with
// the data they serve, never ahead of it.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Options configures a Server.
type Options struct {
	Logger         *slog.Logger
	Pool           *pgxpool.Pool
	Env            string
	ServiceName    string
	RequestTimeout time.Duration
	// CORSAllowedOrigins is empty in production: the UI reaches this API
	// through the same-origin Cloudflare Pages proxy.
	CORSAllowedOrigins []string
	// JournalToken guards the personal journal when non-empty.
	JournalToken string
}

// Server holds API dependencies.
type Server struct {
	log            *slog.Logger
	pool           *pgxpool.Pool
	env            string
	service        string
	requestTimeout time.Duration
	corsOrigins    map[string]struct{}
	journalToken   string
	version        BuildInfo
	startedAt      time.Time
}

// New builds the API server.
func New(opts Options) *Server {
	origins := make(map[string]struct{}, len(opts.CORSAllowedOrigins))
	for _, origin := range opts.CORSAllowedOrigins {
		origins[origin] = struct{}{}
	}
	return &Server{
		log:            opts.Logger,
		pool:           opts.Pool,
		env:            opts.Env,
		service:        opts.ServiceName,
		requestTimeout: opts.RequestTimeout,
		corsOrigins:    origins,
		journalToken:   opts.JournalToken,
		version:        readBuildInfo(),
		startedAt:      time.Now().UTC(),
	}
}

// Handler returns the fully wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleLiveness)
	mux.HandleFunc("/healthz", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("GET /readyz", s.handleReadiness)
	mux.HandleFunc("/readyz", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("GET /api/v1/version", s.handleVersion)
	mux.HandleFunc("/api/v1/version", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("GET /api/v1/competitions", s.handleCompetitions)
	mux.HandleFunc("GET /api/v1/events", s.handleEvents)
	mux.HandleFunc("GET /api/v1/live", s.handleLive)
	mux.HandleFunc("GET /api/v1/my-bets", s.journalAuth(s.handleMyBets))
	mux.HandleFunc("GET /api/v1/teams/{id}/crest", s.handleTeamCrest)
	mux.HandleFunc("GET /api/v1/events/{id}/movement", s.handleEventMovement)
	mux.HandleFunc("GET /api/v1/events/{id}/prices", s.handleEventPrices)
	mux.HandleFunc("GET /api/v1/analytics/odds-ranges", s.handleOddsRanges)
	mux.HandleFunc("GET /api/v1/snapshots", s.handleSnapshots)
	mux.HandleFunc("GET /api/v1/admin/health", s.journalAuth(s.handleAdminHealth))
	mux.HandleFunc("POST /api/v1/admin/journal/import-json", s.journalAuth(s.handleJournalImportJSON))
	mux.HandleFunc("POST /api/v1/admin/journal/import-screenshots", s.journalAuth(s.handleJournalImportScreenshots))
	mux.HandleFunc("GET /api/v1/events/{id}", s.handleEvent)
	mux.HandleFunc("/", s.handleNotFound)

	return s.recoverer(s.cors(s.requestID(s.accessLog(s.timeout(mux)))))
}

// timeout bounds every request.
func (s *Server) timeout(next http.Handler) http.Handler {
	if s.requestTimeout <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// StartedAt reports process start time, used by /api/v1/version.
func (s *Server) StartedAt() time.Time { return s.startedAt }

// readBuildInfo extracts VCS metadata when the binary was built inside a repo.
func readBuildInfo() BuildInfo {
	info := BuildInfo{Version: "dev"}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, setting := range bi.Settings {
		switch setting.Key {
		case "vcs.revision":
			info.Version = setting.Value
			if len(setting.Value) >= 12 {
				info.Version = setting.Value[:12]
			}
		case "vcs.time":
			info.CommitTime = setting.Value
		case "vcs.modified":
			info.Dirty = setting.Value == "true"
		}
	}
	return info
}
