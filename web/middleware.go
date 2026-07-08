package web

import (
	"time"

	"github.com/labstack/echo/v4"

	hexlog "github.com/jordanbrauer/hex/log"
)

// requestLogger returns middleware that emits a structured log line for each
// request via hex/log. Successful requests log at Info; 4xx at Warn; 5xx and
// handler errors at Error.
//
// The line includes: method, path, status, latency, request_id (if the
// RequestID middleware is installed before this one), and remote_ip.
func requestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()

			status := res.Status
			if err != nil && status == 0 {
				status = 500
			}

			latency := time.Since(start)

			fields := []any{
				"method", req.Method,
				"path", req.URL.Path,
				"status", status,
				"latency", latency,
				"remote_ip", c.RealIP(),
			}

			if id := res.Header().Get(echo.HeaderXRequestID); id != "" {
				fields = append(fields, "request_id", id)
			}

			if err != nil {
				fields = append(fields, "error", err)
			}

			switch {
			case status >= 500 || err != nil:
				hexlog.Error("http", fields...)
			case status >= 400:
				hexlog.Warn("http", fields...)
			default:
				hexlog.Info("http", fields...)
			}

			return err
		}
	}
}
