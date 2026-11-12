package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxAttempts    = 4
	attemptTimeout = 30 * time.Second
	defaultBackoff = 1500 * time.Millisecond
	maxRetryAfter  = 30 * time.Second
)

// Request describes one upstream read.
type Request struct {
	// Provider identifies the source, e.g. "openfootball".
	Provider string
	// CacheKey is a human-readable cache identifier, e.g. "2026-27/en.1.json".
	CacheKey string
	// URL is the absolute upstream URL.
	URL string
	// MaxAge is how long a cached response stays fresh. Zero means the response
	// never expires (completed historical data is immutable).
	MaxAge time.Duration
	// Force bypasses the cache.
	Force bool
	// Headers are added to the request (User-Agent is always set).
	Headers map[string]string
}

// Result is a cached or freshly fetched response.
type Result struct {
	Body      []byte
	FromCache bool
	FetchedAt time.Time
}

// Client is the single outbound HTTP path for every sports provider. It
// implements the cache, retry, backoff and timeout rules in the master plan
// (sections 7 and 8) and keeps responses in PostgreSQL rather than local files.
type Client struct {
	pool        *pgxpool.Pool
	log         *slog.Logger
	http        *http.Client
	ua          string
	backoffBase time.Duration
}

// NewClient builds a provider HTTP client.
func NewClient(pool *pgxpool.Pool, log *slog.Logger, userAgent string) *Client {
	if userAgent == "" {
		userAgent = "OddsVault/0.1 (research; +https://github.com/oyins/oddsvault)"
	}
	if log == nil {
		log = slog.Default()
	}
	return &Client{
		pool: pool,
		log:  log,
		ua:   userAgent,
		http: &http.Client{Timeout: attemptTimeout},
	}
}

// Get returns the response body, using the database cache when it is fresh.
func (c *Client) Get(ctx context.Context, req Request) (Result, error) {
	if req.Provider == "" || req.URL == "" {
		return Result{}, errors.New("provider and URL are required")
	}
	hash := requestHash(req)

	if !req.Force {
		if res, ok := c.fromCache(ctx, req.Provider, hash); ok {
			return res, nil
		}
	}

	body, contentType, err := c.fetch(ctx, req)
	if err != nil {
		return Result{}, err
	}
	fetchedAt := time.Now().UTC()
	if err := c.store(ctx, req, hash, body, contentType, fetchedAt); err != nil {
		// A cache write failure must not lose a successful read.
		c.log.WarnContext(ctx, "provider cache write failed",
			slog.String("provider", req.Provider),
			slog.String("cache_key", req.CacheKey),
			slog.String("error", err.Error()))
	}
	return Result{Body: body, FetchedAt: fetchedAt}, nil
}

func (c *Client) fromCache(ctx context.Context, provider, hash string) (Result, bool) {
	if c.pool == nil {
		return Result{}, false
	}
	var body []byte
	var fetchedAt time.Time
	err := c.pool.QueryRow(ctx, `
		SELECT body, fetched_at
		FROM provider_request_cache
		WHERE provider = $1 AND request_hash = $2
		  AND body IS NOT NULL
		  AND (expires_at IS NULL OR expires_at > now())`,
		provider, hash).Scan(&body, &fetchedAt)
	if err != nil {
		return Result{}, false
	}
	return Result{Body: body, FromCache: true, FetchedAt: fetchedAt}, true
}

func (c *Client) store(ctx context.Context, req Request, hash string, body []byte, contentType string, fetchedAt time.Time) error {
	if c.pool == nil {
		return nil
	}
	var expiresAt *time.Time
	if req.MaxAge > 0 {
		t := fetchedAt.Add(req.MaxAge)
		expiresAt = &t
	}
	sum := sha256.Sum256(body)
	_, err := c.pool.Exec(ctx, `
		INSERT INTO provider_request_cache
			(provider, request_key, request_hash, status_code, content_type, body,
			 payload_hash, size_bytes, fetched_at, expires_at)
		VALUES ($1, $2, $3, 200, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (provider, request_hash) DO UPDATE SET
			request_key = EXCLUDED.request_key,
			status_code = EXCLUDED.status_code,
			content_type = EXCLUDED.content_type,
			body = EXCLUDED.body,
			payload_hash = EXCLUDED.payload_hash,
			size_bytes = EXCLUDED.size_bytes,
			fetched_at = EXCLUDED.fetched_at,
			expires_at = EXCLUDED.expires_at`,
		req.Provider, req.CacheKey, hash, contentType, body,
		hex.EncodeToString(sum[:]), len(body), fetchedAt, expiresAt)
	return err
}

func (c *Client) fetch(ctx context.Context, req Request) ([]byte, string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		httpReq, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, req.URL, nil)
		if err != nil {
			cancel()
			return nil, "", fmt.Errorf("build request: %w", err)
		}
		httpReq.Header.Set("User-Agent", c.ua)
		for key, value := range req.Headers {
			httpReq.Header.Set(key, value)
		}

		resp, err := c.http.Do(httpReq)
		if err != nil {
			cancel()
			lastErr = err
			if attempt == maxAttempts || ctx.Err() != nil {
				break
			}
			c.sleep(ctx, c.backoff(attempt))
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		contentType := resp.Header.Get("Content-Type")
		cancel()

		if readErr != nil {
			lastErr = readErr
			if attempt == maxAttempts {
				break
			}
			c.sleep(ctx, c.backoff(attempt))
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if len(body) == 0 {
				lastErr = errors.New("empty response body")
				if attempt == maxAttempts {
					break
				}
				c.sleep(ctx, c.backoff(attempt))
				continue
			}
			return body, contentType, nil
		}

		lastErr = fmt.Errorf("%s returned HTTP %d", req.URL, resp.StatusCode)
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !retryable || attempt == maxAttempts {
			break
		}
		c.log.WarnContext(ctx, "provider request failed; retrying",
			slog.String("provider", req.Provider),
			slog.String("cache_key", req.CacheKey),
			slog.Int("status", resp.StatusCode),
			slog.Int("attempt", attempt))
		c.sleep(ctx, c.retryDelay(resp.Header.Get("Retry-After"), attempt))
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return nil, "", fmt.Errorf("fetch %s: %w", req.Provider, lastErr)
}

func (c *Client) sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (c *Client) backoff(attempt int) time.Duration {
	return time.Duration(1<<uint(attempt-1)) * c.backoffBase
}

func (c *Client) retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
		d := time.Duration(seconds) * time.Second
		if d > maxRetryAfter {
			return maxRetryAfter
		}
		return d
	}
	return c.backoff(attempt)
}

func requestHash(req Request) string {
	sum := sha256.Sum256([]byte(req.Provider + "\n" + req.CacheKey + "\n" + req.URL))
	return hex.EncodeToString(sum[:])
}
