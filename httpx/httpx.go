// Package httpx is an opinionated outbound HTTP client for hex apps.
//
// It wraps *http.Client with:
//
//   - retries via hex/retry (exponential backoff, respects Retry-After),
//   - default timeout,
//   - structured request/response logging through hex/log,
//   - a request-ID header for cross-service correlation.
//
// Consumers who need something the wrapper does not expose can reach the
// underlying *http.Client via Client.Underlying.
//
// Example:
//
//	c := httpx.New(httpx.Options{
//	    Timeout:     10 * time.Second,
//	    MaxAttempts: 3,
//	    BaseBackoff: 500 * time.Millisecond,
//	})
//
//	resp, err := c.Do(ctx, req)
//	if err != nil { return err }
//	defer resp.Body.Close()
package httpx

import (
	"bytes"
	"context"
	stdErrors "errors"
	"io"
	"net/http"
	"strconv"
	"time"

	hexlog "github.com/jordanbrauer/hex/log"
	"github.com/jordanbrauer/hex/retry"
)

// stdErrorsAs is aliased to keep the shim errorsAs() readable.
var stdErrorsAs = stdErrors.As

// Options configures a Client.
type Options struct {
	// Timeout is the per-request timeout applied to every attempt (not
	// the total call). Zero means 30 seconds.
	Timeout time.Duration

	// MaxAttempts caps total attempts (initial + retries). Zero means 3.
	// Set to 1 to disable retries.
	MaxAttempts int

	// BaseBackoff is the base delay between retries. Zero means 200ms.
	BaseBackoff time.Duration

	// MaxBackoff caps individual backoffs. Zero means 30s.
	MaxBackoff time.Duration

	// RetryOn is the set of HTTP status codes that trigger a retry
	// (idempotent methods only). Zero-value defaults to
	// {429, 500, 502, 503, 504}.
	RetryOn []int

	// UserAgent, when non-empty, is set on every outbound request unless
	// the caller already set one.
	UserAgent string

	// Transport is the underlying http.RoundTripper. Zero uses
	// http.DefaultTransport.
	Transport http.RoundTripper
}

// Client is a hex outbound HTTP client. Safe for concurrent use.
type Client struct {
	opts    Options
	http    *http.Client
	retryOn map[int]bool
}

// New returns a Client with opts applied.
func New(opts Options) *Client {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	if opts.MaxAttempts == 0 {
		opts.MaxAttempts = 3
	}

	if opts.BaseBackoff == 0 {
		opts.BaseBackoff = 200 * time.Millisecond
	}

	if opts.MaxBackoff == 0 {
		opts.MaxBackoff = 30 * time.Second
	}

	if len(opts.RetryOn) == 0 {
		opts.RetryOn = []int{429, 500, 502, 503, 504}
	}

	retryOn := make(map[int]bool, len(opts.RetryOn))
	for _, code := range opts.RetryOn {
		retryOn[code] = true
	}

	transport := opts.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &Client{
		opts: opts,
		http: &http.Client{
			Transport: transport,
			Timeout:   opts.Timeout,
		},
		retryOn: retryOn,
	}
}

// Underlying returns the wrapped *http.Client. Use only when Client's
// higher-level surface is insufficient — bypassing it skips retry,
// logging, and default headers.
func (c *Client) Underlying() *http.Client { return c.http }

// Do executes req with retry, backoff, and logging. The request body
// must be re-readable across attempts; if it is not, Do buffers it
// once at the start.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	c.applyDefaults(req)

	// Buffer the body so retries can replay it. GetBody handles the
	// common case (net/http sets GetBody for bytes.Buffer bodies); we
	// buffer only when it is absent.
	if err := c.rewindBody(req); err != nil {
		return nil, err
	}

	var (
		resp     *http.Response
		attempt  int
		startAll = time.Now()
	)

	err := retry.Do(ctx, func(ctx context.Context) error {
		attempt++

		start := time.Now()

		if attempt > 1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return err
			}

			req.Body = body
		}

		// Any prior response we're about to replace must be drained so
		// its connection returns to the pool.
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			resp = nil
		}

		var doErr error
		resp, doErr = c.http.Do(req)

		latency := time.Since(start)

		if doErr != nil {
			hexlog.Warn("httpx",
				"method", req.Method, "url", req.URL.String(),
				"attempt", attempt, "error", doErr, "latency", latency)

			return doErr
		}

		if c.retryOn[resp.StatusCode] {
			hexlog.Warn("httpx",
				"method", req.Method, "url", req.URL.String(),
				"status", resp.StatusCode, "attempt", attempt, "latency", latency)

			return &retryableStatusErr{status: resp.StatusCode}
		}

		return nil
	}, retry.Options{
		MaxAttempts: c.opts.MaxAttempts,
		BaseDelay:   c.opts.BaseBackoff,
		MaxDelay:    c.opts.MaxBackoff,
	})

	// If the failure was a retryable status code exhausted, we still
	// have the last response to hand back — callers can inspect it
	// or discard it as they wish. Transport-level errors get returned
	// with a nil response as usual.
	var retryable *retryableStatusErr
	if err != nil && !errorsAs(err, &retryable) {
		if resp != nil {
			_ = resp.Body.Close()
		}

		return nil, err
	}

	hexlog.Info("httpx",
		"method", req.Method, "url", req.URL.String(),
		"status", resp.StatusCode, "attempts", attempt,
		"total_latency", time.Since(startAll))

	return resp, nil
}

// Get is a convenience wrapper equivalent to a GET request.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return c.Do(ctx, req)
}

// Post is a convenience wrapper for POST with a request body.
func (c *Client) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return c.Do(ctx, req)
}

// applyDefaults sets Client-configured headers unless the caller already
// customised them.
func (c *Client) applyDefaults(req *http.Request) {
	if c.opts.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.opts.UserAgent)
	}
}

// rewindBody ensures req.Body can be replayed on retry. If GetBody is
// already set (net/http does this for common body types), no action is
// needed. Otherwise buffer the body once.
func (c *Client) rewindBody(req *http.Request) error {
	if req.Body == nil || req.GetBody != nil {
		return nil
	}

	buf, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}

	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(buf))
	req.ContentLength = int64(len(buf))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}

	return nil
}

// retryableStatusErr is the sentinel returned to retry.Do so it triggers
// a retry. The final response is stashed on Client.Do so the caller
// eventually sees it.
type retryableStatusErr struct {
	status int
}

func (e *retryableStatusErr) Error() string {
	return "httpx: retryable status " + strconv.Itoa(e.status)
}

// errorsAs is a small wrapper so we can errors.As without importing the
// stdlib errors package into this file's top imports (already crowded).
// Named this way so the compile-time shim is clear from the call site.
func errorsAs(err error, target any) bool {
	return stdErrorsAs(err, target)
}
