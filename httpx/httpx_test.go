package httpx_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/httpx"
)

func TestGet_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello")
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{})

	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "hello" {
		t.Errorf("body = %q", body)
	}
}

func TestRetries_onRetryableStatus(t *testing.T) {
	var attempts int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{
		MaxAttempts: 5,
		BaseBackoff: 10 * time.Millisecond,
	})

	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("final status = %d", resp.StatusCode)
	}

	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestNoRetries_onNon2xxNotInList(t *testing.T) {
	var attempts int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{
		MaxAttempts: 5,
		BaseBackoff: 10 * time.Millisecond,
	})

	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("status = %d", resp.StatusCode)
	}

	if got := atomic.LoadInt64(&attempts); got != 1 {
		t.Errorf("attempts = %d, want 1 (400 not retryable by default)", got)
	}
}

func TestRetries_giveUpAfterMaxAttempts(t *testing.T) {
	var attempts int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{
		MaxAttempts: 3,
		BaseBackoff: 5 * time.Millisecond,
	})

	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Final response is the last (still-500) attempt.
	if resp != nil {
		resp.Body.Close()
	}

	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestPost_bodyReplayedOnRetry(t *testing.T) {
	var (
		attempts int64
		bodies   [][]byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, b)

		n := atomic.AddInt64(&attempts, 1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{
		MaxAttempts: 3,
		BaseBackoff: 5 * time.Millisecond,
	})

	resp, err := c.Post(context.Background(), srv.URL, "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	defer resp.Body.Close()

	if len(bodies) != 2 {
		t.Fatalf("bodies received = %d, want 2", len(bodies))
	}

	for i, b := range bodies {
		if string(b) != "payload" {
			t.Errorf("attempt %d body = %q, want payload", i, b)
		}
	}
}

func TestUserAgent_applied(t *testing.T) {
	var got string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{UserAgent: "hex-test/1.0"})

	resp, _ := c.Get(context.Background(), srv.URL)
	if resp != nil {
		resp.Body.Close()
	}

	if got != "hex-test/1.0" {
		t.Errorf("UA = %q, want hex-test/1.0", got)
	}
}

func TestUserAgent_doesNotOverrideCallerHeader(t *testing.T) {
	var got string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{UserAgent: "hex-default"})

	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("User-Agent", "explicit-ua")

	resp, _ := c.Do(context.Background(), req)
	if resp != nil {
		resp.Body.Close()
	}

	if got != "explicit-ua" {
		t.Errorf("UA = %q, want caller's explicit-ua", got)
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{Timeout: time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := c.Get(ctx, srv.URL)
	if err == nil {
		t.Errorf("expected error from cancelled context")
	}
}

func TestUnderlying_exposesClient(t *testing.T) {
	c := httpx.New(httpx.Options{Timeout: 5 * time.Second})

	if got := c.Underlying(); got == nil {
		t.Errorf("Underlying returned nil")
	}

	if c.Underlying().Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", c.Underlying().Timeout)
	}
}
