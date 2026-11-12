package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer() *Server {
	return New(Options{
		Logger:         slog.New(slog.DiscardHandler),
		Pool:           nil,
		Env:            "test",
		ServiceName:    "oddsvault-api",
		RequestTimeout: 0,
	})
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected status %v", body["status"])
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header")
	}
}

func TestReadyzWithoutDatabase(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestUnknownRouteIsJSON404(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type %q", ct)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("unexpected error code %q", body.Error.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestCORSIsClosedByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	newTestServer().Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header without configuration, got %q", got)
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	server := New(Options{
		Logger:             slog.New(slog.DiscardHandler),
		Env:                "test",
		ServiceName:        "oddsvault-api",
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/events", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 preflight, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("unexpected allow-origin %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary: Origin, got %q", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	server := New(Options{
		Logger:             slog.New(slog.DiscardHandler),
		Env:                "test",
		ServiceName:        "oddsvault-api",
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/events", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 preflight, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow-origin for unknown origin, got %q", got)
	}
}

func TestJournalRequiresTokenWhenConfigured(t *testing.T) {
	server := New(Options{
		Logger:       slog.New(slog.DiscardHandler),
		Env:          "test",
		ServiceName:  "oddsvault-api",
		JournalToken: "test-token",
	})

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/my-bets", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatal("expected WWW-Authenticate header")
	}
}

func TestJournalAcceptsConfiguredToken(t *testing.T) {
	server := New(Options{
		Logger:       slog.New(slog.DiscardHandler),
		Env:          "test",
		ServiceName:  "oddsvault-api",
		JournalToken: "test-token",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/my-bets", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	// No database is attached, so reaching the handler yields 503. That still
	// proves the token check passed.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after auth, got %d", rec.Code)
	}
}

func TestJournalOpenWhenTokenNotConfigured(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer().Handler().ServeHTTP(
		rec, httptest.NewRequest(http.MethodGet, "/api/v1/my-bets", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without a token requirement, got %d", rec.Code)
	}
}

func TestJournalAllowsConfiguredDevOrigin(t *testing.T) {
	server := New(Options{
		Logger:             slog.New(slog.DiscardHandler),
		Env:                "development",
		ServiceName:        "oddsvault-api",
		JournalToken:       "test-token",
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/my-bets", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	// No database: reaching the handler returns 503, proving auth passed.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for an allowed dev origin, got %d", rec.Code)
	}
}

func TestJournalRejectsUnknownOriginWithoutToken(t *testing.T) {
	server := New(Options{
		Logger:             slog.New(slog.DiscardHandler),
		Env:                "development",
		ServiceName:        "oddsvault-api",
		JournalToken:       "test-token",
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/my-bets", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unknown origin, got %d", rec.Code)
	}
}
