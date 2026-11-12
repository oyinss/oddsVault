package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRetriesRetryableStatus(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient(nil, nil, "test")
	client.backoffBase = time.Millisecond
	res, err := client.Get(context.Background(), Request{
		Provider: "test",
		CacheKey: "retry",
		URL:      server.URL,
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(res.Body) != "ok" {
		t.Fatalf("body = %q", res.Body)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if res.FromCache {
		t.Fatal("expected a fresh response")
	}
}

func TestClientDoesNotRetryClientError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(nil, nil, "test")
	client.backoffBase = time.Millisecond
	_, err := client.Get(context.Background(), Request{Provider: "test", CacheKey: "404", URL: server.URL})
	if err == nil {
		t.Fatal("expected an error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no blind 4xx retry)", attempts)
	}
}

func TestClientRejectsEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(nil, nil, "test")
	client.backoffBase = time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.Get(ctx, Request{Provider: "test", CacheKey: "empty", URL: server.URL}); err == nil {
		t.Fatal("expected an error for an empty body")
	}
}
