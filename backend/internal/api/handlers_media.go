package api

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	crestMaxBytes = 2 << 20 // 2 MiB
	crestTimeout  = 10 * time.Second
)

// crestHosts is the allowlist for proxied image bytes. Only URLs already stored
// on a canonical entity are fetched, and only from these hosts.
var crestHosts = map[string]bool{
	"a.espncdn.com":       true,
	"r2.thesportsdb.com":  true,
	"www.thesportsdb.com": true,
}

func crestCacheDir() string {
	if dir := os.Getenv("MEDIA_CACHE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "oddsvault-crests")
}

// handleTeamCrest proxies a team logo through our own origin and caches the
// bytes, so the browser never calls the provider and CORS is a non-issue.
func (s *Server) handleTeamCrest(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_id", "team id must be a positive integer")
		return
	}

	var sourceURL string
	err = s.pool.QueryRow(r.Context(), `
		SELECT source_url FROM media_assets
		WHERE entity_type = 'team' AND entity_id = $1 AND asset_type = 'team_logo' AND active
		LIMIT 1`, id).Scan(&sourceURL)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme != "https" || !crestHosts[parsed.Hostname()] {
		s.log.WarnContext(r.Context(), "refusing non-allowlisted crest host",
			"host", parsed.Hostname(), "team_id", id)
		http.NotFound(w, r)
		return
	}

	dir := crestCacheDir()
	cachePath := filepath.Join(dir, strconv.FormatInt(id, 10)+filepath.Ext(parsed.Path))
	if data, err := os.ReadFile(cachePath); err == nil && len(data) > 0 {
		writeImage(w, contentTypeFor(cachePath), data)
		return
	}

	client := &http.Client{Timeout: crestTimeout}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, sourceURL, nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	req.Header.Set("User-Agent", "OddsVault/0.1 (research)")
	req.Header.Set("Accept", "image/*")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		http.NotFound(w, r)
		return
	}
	defer resp.Body.Close()

	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		http.NotFound(w, r)
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, crestMaxBytes+1))
	if err != nil || len(data) == 0 || len(data) > crestMaxBytes {
		http.NotFound(w, r)
		return
	}

	if err := os.MkdirAll(dir, 0o755); err == nil {
		_ = os.WriteFile(cachePath, data, 0o644)
	}
	writeImage(w, contentType, data)
}

func writeImage(w http.ResponseWriter, contentType string, data []byte) {
	if contentType == "" {
		contentType = "image/png"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func contentTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/png"
	}
}
