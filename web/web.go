// Package web is a hex-opinionated wrapper around labstack/echo/v4.
//
// A Server owns an *echo.Echo instance pre-wired with the hex-standard
// middleware stack (request ID, panic recovery, CORS, structured logging
// through hex/log) and health/readiness endpoints. Consumers register
// routes on the underlying echo instance via Echo().
//
// See ADR-0006 for scope decisions: hex/web is medium-scope. It owns the
// server, middleware, health endpoints, and graceful shutdown. It does
// not prescribe controller/router directory conventions.
//
// Example:
//
//	srv := web.New(web.Options{Address: ":8080"})
//	srv.Echo().GET("/hello", func(c echo.Context) error {
//	    return c.String(200, "hi")
//	})
//	if err := srv.Start(); err != nil { return err }
//	defer srv.Shutdown(ctx)
//
// The Server satisfies hex/provider.Shutdowner so a consumer provider can
// register the server and have shutdown wired automatically.
package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// Options configures a new Server. All fields are optional; the zero value
// yields a Server listening on :8080 with the full middleware stack.
type Options struct {
	// Address is the listen address (e.g. ":8080", "127.0.0.1:3000").
	// Defaults to ":8080".
	Address string

	// ReadTimeout, WriteTimeout, IdleTimeout configure the underlying
	// net/http server. Zero uses echo's defaults (no timeout).
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// HealthPath is the route that reports application liveness. Defaults
	// to "/healthz". Set to "" to disable.
	HealthPath string

	// ReadyPath is the route that reports application readiness. Defaults
	// to "/readyz". Set to "" to disable.
	ReadyPath string

	// ReadyFn is invoked on ReadyPath. Return nil for 200 OK, or an error
	// to render 503 with the error text. If nil, ReadyPath always returns
	// 200.
	ReadyFn func(context.Context) error

	// CORS toggles the default CORS middleware. Defaults to true. Pass
	// CORSConfig to customise; the default allows all origins/methods.
	CORS       bool
	CORSConfig *echomw.CORSConfig

	// DisableRequestID skips the RequestID middleware.
	DisableRequestID bool

	// DisableRecover skips the panic-recovery middleware. Do not disable
	// in production.
	DisableRecover bool

	// DisableLogger skips the hex-log request logger middleware.
	DisableLogger bool
}

// Server is a hex HTTP server. Zero value is not usable; call New.
type Server struct {
	e    *echo.Echo
	opts Options
}

// New constructs a Server with the hex-standard middleware stack applied.
// The server is not yet listening; call Start.
//
// Default middleware, in order: RequestID, request logger (hex/log),
// Recover. CORS is opt-in via Options.CORS or Options.CORSConfig — many
// hex services are internal-only and do not need it.
func New(opts Options) *Server {
	if opts.Address == "" {
		opts.Address = ":8080"
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	if !opts.DisableRequestID {
		e.Use(echomw.RequestID())
	}

	if !opts.DisableLogger {
		e.Use(requestLogger())
	}

	if opts.CORS || opts.CORSConfig != nil {
		if opts.CORSConfig != nil {
			e.Use(echomw.CORSWithConfig(*opts.CORSConfig))
		} else {
			e.Use(echomw.CORS())
		}
	}

	if !opts.DisableRecover {
		e.Use(echomw.Recover())
	}

	// Health / readiness routes. Convention: empty string means default (
	// /healthz, /readyz). "-" is a sentinel to disable the route entirely.
	health := resolveHealthPath(opts.HealthPath, "/healthz")
	ready := resolveHealthPath(opts.ReadyPath, "/readyz")

	if health != "" {
		e.GET(health, func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})
	}

	if ready != "" {
		e.GET(ready, func(c echo.Context) error {
			if opts.ReadyFn == nil {
				return c.String(http.StatusOK, "ready")
			}

			if err := opts.ReadyFn(c.Request().Context()); err != nil {
				return c.String(http.StatusServiceUnavailable, err.Error())
			}

			return c.String(http.StatusOK, "ready")
		})
	}

	if opts.ReadTimeout > 0 {
		e.Server.ReadTimeout = opts.ReadTimeout
	}

	if opts.WriteTimeout > 0 {
		e.Server.WriteTimeout = opts.WriteTimeout
	}

	if opts.IdleTimeout > 0 {
		e.Server.IdleTimeout = opts.IdleTimeout
	}

	return &Server{e: e, opts: opts}
}

// resolveHealthPath applies our two conventions: "" means default, "-" means
// disabled, anything else is the exact path.
func resolveHealthPath(input, fallback string) string {
	switch input {
	case "":
		return fallback
	case "-":
		return ""
	default:
		return input
	}
}

// Echo returns the underlying *echo.Echo instance. Use this to register
// routes, groups, and additional middleware.
func (s *Server) Echo() *echo.Echo { return s.e }

// Address returns the listen address the server is configured for.
func (s *Server) Address() string { return s.opts.Address }

// Start begins listening on the configured address. Start blocks until the
// server stops. Use it in a goroutine and pair with Shutdown.
//
// Start returns http.ErrServerClosed when Shutdown is invoked normally.
// Callers should treat that as success:
//
//	if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
//	    return err
//	}
func (s *Server) Start() error {
	err := s.e.Start(s.opts.Address)
	if errors.Is(err, http.ErrServerClosed) {
		return err // let caller decide; ErrServerClosed is often success
	}

	return err
}

// Shutdown gracefully stops the server, waiting up to ctx's deadline for
// in-flight requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.e == nil {
		return nil
	}

	return s.e.Shutdown(ctx)
}
