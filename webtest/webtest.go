// Package webtest is a supertest-flavoured HTTP client + a
// react-testing-library-flavoured DOM query surface for testing
// hex web apps end-to-end.
//
// Two use modes:
//
//  1. Direct Go tests — build a Client, chain request/assert calls:
//
//     client := webtest.New(t, app)
//     client.Get("/dashboard").
//     StatusIs(200).
//     See("Welcome, Alice").
//     Find(".user-card").HasClass("active").
//     Find("button").Count(3)
//
//  2. BDD/Gherkin — wire the standard step vocabulary via
//     hex/webtest/bdd.Register, then write scenarios in .feature files.
//
// The client boots the app once (via httptest.NewServer wrapping the
// Echo instance the web provider registers under "http"), then runs
// real requests through the real network stack. Fast enough for a
// tight feedback loop; realistic enough that middleware, routing,
// cookies, and redirects all behave the same as production.
//
// Server-side rendering (Go templates, Jade, Markdown) is fully
// covered. JavaScript behaviour (Alpine, HTMX transitions on the
// browser side) is NOT executed — for that, a follow-up will add
// chromedp/rod bindings. HTMX-triggered server routes DO get
// exercised, since HTMX is just a normal HTTP request with special
// headers; use Client.Header("HX-Request", "true") to simulate the
// header the client would send.
package webtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"
	hexweb "github.com/jordanbrauer/hex/web"
)

// Client is a stateful HTTP client bound to an in-process hex app.
// After every request the response body is parsed as HTML (via
// goquery) so assertions like Find(".selector") work without a
// second network round-trip.
//
// Client is NOT goroutine-safe — one Client per test scenario.
type Client struct {
	t       testing.TB
	server  *httptest.Server
	http    *http.Client
	headers http.Header

	lastResp *http.Response
	lastBody []byte
	lastDoc  *goquery.Document
	lastReq  string // "GET /path" for error messages
}

// New starts a test HTTP server wrapping the app's Echo instance
// (resolved from the container under "http") and returns a Client
// bound to it. The server is torn down via t.Cleanup.
func New(t testing.TB, app *hex.App) *Client {
	t.Helper()

	srv, err := container.Make[*hexweb.Server](app.Container(), "http")
	if err != nil {
		t.Fatalf("webtest: resolve *web.Server: %v", err)
	}

	ts := httptest.NewServer(srv.Echo())
	t.Cleanup(ts.Close)

	jar, _ := newCookieJar()

	return &Client{
		t:      t,
		server: ts,
		http: &http.Client{
			Jar: jar,
		},
		headers: http.Header{},
	}
}

// URL returns the base URL of the test server. Handy for building
// absolute URLs (redirect targets, etc.).
func (c *Client) URL() string { return c.server.URL }

// Header sets a default header applied to every subsequent request.
// Multiple calls with the same name append values.
func (c *Client) Header(name, value string) *Client {
	c.headers.Add(name, value)

	return c
}

// -- Requests ------------------------------------------------------------

// Get performs a GET at path (relative to the test server root).
// Returns c for chaining.
func (c *Client) Get(path string) *Client {
	return c.do(http.MethodGet, path, nil, "")
}

// Post performs a POST with a raw body. contentType is stored in the
// Content-Type header; pass an empty string to skip.
func (c *Client) Post(path string, body io.Reader, contentType string) *Client {
	return c.do(http.MethodPost, path, body, contentType)
}

// PostForm posts a url-encoded form.
func (c *Client) PostForm(path string, form url.Values) *Client {
	return c.do(http.MethodPost, path, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
}

// PostJSON posts a JSON-encoded body.
func (c *Client) PostJSON(path string, body any) *Client {
	buf, err := json.Marshal(body)
	if err != nil {
		c.t.Fatalf("webtest: PostJSON: encode body: %v", err)
	}

	return c.do(http.MethodPost, path, bytes.NewReader(buf), "application/json")
}

// Delete performs a DELETE.
func (c *Client) Delete(path string) *Client {
	return c.do(http.MethodDelete, path, nil, "")
}

// Put performs a PUT with the given body. Empty contentType is treated
// as text/plain.
func (c *Client) Put(path string, body io.Reader, contentType string) *Client {
	return c.do(http.MethodPut, path, body, contentType)
}

func (c *Client) do(method, path string, body io.Reader, contentType string) *Client {
	c.t.Helper()

	req, err := http.NewRequest(method, c.server.URL+path, body)
	if err != nil {
		c.t.Fatalf("webtest: build request %s %s: %v", method, path, err)
	}

	for name, values := range c.headers {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("webtest: %s %s: %v", method, path, err)
	}

	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("webtest: %s %s: read body: %v", method, path, err)
	}

	c.lastResp = resp
	c.lastBody = buf
	c.lastDoc = nil // lazy re-parse
	c.lastReq = fmt.Sprintf("%s %s", method, path)

	return c
}

// -- Response assertions -------------------------------------------------

// Status returns the last response's HTTP status code. Fails the
// test if no request has been sent yet.
func (c *Client) Status() int {
	c.requireResp()

	return c.lastResp.StatusCode
}

// StatusIs asserts the last response status equals expected.
func (c *Client) StatusIs(expected int) *Client {
	c.t.Helper()
	c.requireResp()

	if c.lastResp.StatusCode != expected {
		c.t.Errorf("%s: status = %d, want %d", c.lastReq, c.lastResp.StatusCode, expected)
	}

	return c
}

// Body returns the last response body as a string.
func (c *Client) Body() string {
	c.requireResp()

	return string(c.lastBody)
}

// BodyContains asserts the response body contains substring.
func (c *Client) BodyContains(substring string) *Client {
	c.t.Helper()
	c.requireResp()

	if !strings.Contains(string(c.lastBody), substring) {
		c.t.Errorf("%s: body missing %q\nbody:\n%s", c.lastReq, substring, c.lastBody)
	}

	return c
}

// HeaderIs asserts the response has the given header name/value.
func (c *Client) HeaderIs(name, value string) *Client {
	c.t.Helper()
	c.requireResp()

	got := c.lastResp.Header.Get(name)
	if got != value {
		c.t.Errorf("%s: header %s = %q, want %q", c.lastReq, name, got, value)
	}

	return c
}

// LocationIs asserts the Location header equals path (for redirects).
func (c *Client) LocationIs(path string) *Client {
	return c.HeaderIs("Location", path)
}

// See asserts text appears anywhere in the response body.
func (c *Client) See(text string) *Client {
	return c.BodyContains(text)
}

// -- DOM queries via goquery --------------------------------------------

// Doc lazily parses the last body as HTML and returns the *goquery
// document. Escape hatch for callers who want the full goquery API.
func (c *Client) Doc() *goquery.Document {
	c.t.Helper()
	c.requireResp()

	if c.lastDoc == nil {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(c.lastBody))
		if err != nil {
			c.t.Fatalf("webtest: parse HTML: %v", err)
		}

		c.lastDoc = doc
	}

	return c.lastDoc
}

// Find selects elements matching sel. See Selection for chainable
// assertions.
func (c *Client) Find(sel string) *Selection {
	return &Selection{
		t:        c.t,
		selector: sel,
		sel:      c.Doc().Find(sel),
		client:   c,
	}
}

// -- helpers ------------------------------------------------------------

func (c *Client) requireResp() {
	c.t.Helper()

	if c.lastResp == nil {
		c.t.Fatal("webtest: no request has been sent yet")
	}
}
