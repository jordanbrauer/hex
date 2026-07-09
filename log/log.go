// Package log is a slog-first logger backed by charmbracelet/log's
// slog.Handler implementation.
//
// The public API is a small wrapper over log/slog: Debug/Info/Warn/Error
// (and hex-specific Fatal) delegate to slog.Default, which is installed
// by Init to point at a charmbracelet/log handler. Consumers who need
// attribute grouping, per-request loggers, or a distinct handler use
// Logger()/Handler() to reach the underlying slog types directly.
//
// Attribute helpers (log.String, log.Int, log.Group, ...) are re-exported
// from log/slog for convenience so callers do not have to import both
// packages side-by-side.
//
// Init runs once from main; downstream providers use SetLevel to adjust
// verbosity at runtime.
package log

import (
	"io"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
	charm "github.com/charmbracelet/log"
)

// Level is Go's standard log/slog level type. Aliased so consumers can
// pass hex.log.Level anywhere slog.Level is expected.
type Level = slog.Level

// Level constants. Numeric values match slog.Level (which in turn
// matches charmbracelet/log.Level 1:1). FatalLevel is a hex extension
// that mirrors charmbracelet/log.FatalLevel — slog has no Fatal so
// hex.log.Fatal below routes through the charm handler directly.
const (
	DebugLevel = slog.LevelDebug // -4
	InfoLevel  = slog.LevelInfo  //  0
	WarnLevel  = slog.LevelWarn  //  4
	ErrorLevel = slog.LevelError //  8
	FatalLevel = Level(12)       // matches charmbracelet/log.FatalLevel
)

// Attr is Go's standard slog attribute type. Aliased so callers writing
// log.With(log.String("k","v")) do not need to import log/slog directly.
type Attr = slog.Attr

// current holds the underlying charmbracelet handler so we can:
//  1. update its Level at runtime via SetLevel
//  2. call handler.Fatal directly (slog has no Fatal)
//
// Set explicitly by Init; auto-initialised with defaults by the first
// accessor that needs a handler (see handler()). atomic.Pointer avoids
// a lock on the hot Debug/Info/... paths.
var current atomic.Pointer[charm.Logger]

// handler returns the current charm handler, calling Init with defaults
// if the consumer never did. This preserves the original hex/log
// contract that Debug/Info/SetLevel/etc. work out of the box.
func handler() *charm.Logger {
	if h := current.Load(); h != nil {
		return h
	}

	Init()

	return current.Load()
}

// options captures the configuration a call to Init applies. Values are
// applied verbatim to a fresh charm handler.
type options struct {
	level     Level
	caller    bool
	timestamp bool
	setStyles bool
}

// Option configures the logger. See Init.
type Option func(*options)

// WithLevel sets the minimum level of messages that are emitted.
func WithLevel(level Level) Option {
	return func(o *options) { o.level = level }
}

// WithCaller toggles the file:line caller annotation on each log entry.
func WithCaller(enabled bool) Option {
	return func(o *options) { o.caller = enabled }
}

// WithTimestamp toggles timestamps on each log entry.
func WithTimestamp(enabled bool) Option {
	return func(o *options) { o.timestamp = enabled }
}

// WithoutStyles disables hex's default colour palette. Use when the
// consumer wants to install its own styles or produce plain output.
func WithoutStyles() Option {
	return func(o *options) { o.setStyles = false }
}

// Init configures the global slog handler and installs it via
// slog.SetDefault. Idempotent — the last call wins.
//
// Defaults: level FatalLevel (silent until the app opts in), caller and
// timestamp off, hex's colour palette applied. A typical main calls
// Init exactly once, before hex.App.Bootstrap.
func Init(opts ...Option) {
	cfg := options{
		level:     FatalLevel,
		caller:    false,
		timestamp: false,
		setStyles: true,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	h := charm.NewWithOptions(os.Stderr, charm.Options{
		Level:           charm.Level(cfg.level),
		ReportCaller:    cfg.caller,
		ReportTimestamp: cfg.timestamp,
	})

	if cfg.setStyles {
		applyDefaultStyles(h)
	}

	current.Store(h)
	slog.SetDefault(slog.New(h))
}

// Handler returns slog.Default().Handler(). After Init installs hex's
// charmbracelet handler this is the same handler; if a consumer swaps
// slog.Default externally (custom JSON handler, OTel exporter, etc.)
// this follows them — the whole point of routing through slog is that
// downstream code can redirect logs without knowing about hex/log.
func Handler() slog.Handler { return slog.Default().Handler() }

// SetOutput redirects hex's charmbracelet handler to write to w.
// Applies globally — subsequent log calls from anywhere in the
// process go to w until the next SetOutput call.
//
// The REPL uses this per-eval to capture log output into scrollback
// instead of writing to os.Stderr behind Bubble Tea's back. General
// consumers can point logs at a file, buffer, or io.MultiWriter.
//
// No-op if a consumer has swapped slog.Default externally to a
// non-charm handler; redirect via that handler's own API instead.
func SetOutput(w io.Writer) { handler().SetOutput(w) }

// Logger returns slog.Default(). Handy for callers wanting to hold
// onto a logger reference, use With/WithGroup, or pass to libraries
// that accept *slog.Logger.
func Logger() *slog.Logger { return slog.Default() }

// With returns a *slog.Logger with the given attributes attached to
// every subsequent record. Shorthand for slog.Default().With(args...).
func With(args ...any) *slog.Logger { return slog.Default().With(args...) }

// WithGroup returns a *slog.Logger that starts a new attribute group.
// Every subsequent attribute becomes a nested field of that group in
// structured backends. Shorthand for slog.Default().WithGroup(name).
func WithGroup(name string) *slog.Logger { return slog.Default().WithGroup(name) }

// SetLevel updates the minimum log level of the global handler.
// Lazily initialises a default handler if Init has not run.
func SetLevel(level Level) { handler().SetLevel(charm.Level(level)) }

// SetCaller toggles the file:line caller annotation on each log entry.
func SetCaller(enabled bool) { handler().SetReportCaller(enabled) }

// SetTimestamp toggles timestamps on each log entry.
func SetTimestamp(enabled bool) { handler().SetReportTimestamp(enabled) }

// GetLevel returns the current minimum log level. Lazily initialises a
// default handler if Init has not run.
func GetLevel() Level { return Level(handler().GetLevel()) }

// ParseLevel converts a string like "info" or "warn" into a Level. Wraps
// charmbracelet/log's parser so callers do not need to import it.
func ParseLevel(s string) (Level, error) {
	cl, err := charm.ParseLevel(s)
	if err != nil {
		return 0, err
	}

	return Level(cl), nil
}

// Debug logs at DebugLevel via slog.Default.
func Debug(msg string, args ...any) { slog.Debug(msg, args...) }

// Info logs at InfoLevel via slog.Default.
func Info(msg string, args ...any) { slog.Info(msg, args...) }

// Warn logs at WarnLevel via slog.Default.
func Warn(msg string, args ...any) { slog.Warn(msg, args...) }

// Error logs at ErrorLevel via slog.Default.
func Error(msg string, args ...any) { slog.Error(msg, args...) }

// Fatal logs at FatalLevel via the charmbracelet handler (slog has no
// Fatal) and terminates the process with status 1. Do not call from
// library code; reserve it for main.
func Fatal(msg string, args ...any) { handler().Fatal(msg, args...) }

// Attribute helpers re-exported from log/slog so callers writing
// hex.log.Info("msg", hex.log.String("k","v")) do not need to import
// log/slog side-by-side.

// String returns an Attr for a string value.
func String(key, value string) Attr { return slog.String(key, value) }

// Int returns an Attr for an int value.
func Int(key string, value int) Attr { return slog.Int(key, value) }

// Int64 returns an Attr for an int64 value.
func Int64(key string, value int64) Attr { return slog.Int64(key, value) }

// Uint64 returns an Attr for a uint64 value.
func Uint64(key string, value uint64) Attr { return slog.Uint64(key, value) }

// Float64 returns an Attr for a float64 value.
func Float64(key string, value float64) Attr { return slog.Float64(key, value) }

// Bool returns an Attr for a bool value.
func Bool(key string, value bool) Attr { return slog.Bool(key, value) }

// Any returns an Attr for any value; the handler decides how to render.
func Any(key string, value any) Attr { return slog.Any(key, value) }

// Group returns an Attr containing a nested group of attributes. Handlers
// render this as a sub-object in structured output and as prefixed keys
// in text output.
func Group(key string, args ...any) Attr { return slog.Group(key, args...) }

// applyDefaultStyles installs hex's colour palette on the given handler.
// Matches finch-cli's existing colours so users switching to hex see no
// visual change.
func applyDefaultStyles(h *charm.Logger) {
	styles := charm.DefaultStyles()
	styles.Levels[charm.DebugLevel] = styles.Levels[charm.DebugLevel].Foreground(lipgloss.Color("4"))
	styles.Levels[charm.InfoLevel] = styles.Levels[charm.InfoLevel].Foreground(lipgloss.Color("2"))
	styles.Levels[charm.WarnLevel] = styles.Levels[charm.WarnLevel].Foreground(lipgloss.Color("3"))
	styles.Levels[charm.ErrorLevel] = styles.Levels[charm.ErrorLevel].Foreground(lipgloss.Color("1"))
	styles.Levels[charm.FatalLevel] = styles.Levels[charm.FatalLevel].Foreground(lipgloss.Color("6"))
	h.SetStyles(styles)
}
