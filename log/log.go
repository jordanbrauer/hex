// Package log is a thin, opinionated wrapper around charmbracelet/log.
//
// The wrapper exists to give hex applications a consistent set of styled
// defaults and a single place to bump the underlying library. Consumers
// call Init once from main, then use Debug/Info/Warn/Error/Fatal directly
// or through the package-level SetLevel to change verbosity at runtime.
//
// Unlike the reference implementations in finch-cli and finch-bot this
// package does not configure the logger from an implicit init() — startup
// order is the consumer's business. Until Init runs, hex.log behaves like
// a freshly-imported charmbracelet/log (info level, no caller, no
// timestamp).
package log

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

// Level is an alias for the underlying charmbracelet/log level type. Aliased
// (not re-exported) so consumers can pass hex.log.Level anywhere the raw
// library type is required, without an extra conversion.
type Level = log.Level

// Level constants re-exported from charmbracelet/log for callers that do not
// want to import the underlying package.
const (
	DebugLevel = log.DebugLevel
	InfoLevel  = log.InfoLevel
	WarnLevel  = log.WarnLevel
	ErrorLevel = log.ErrorLevel
	FatalLevel = log.FatalLevel
)

// options captures the configuration a call to Init applies. Values are
// applied verbatim to the charmbracelet default logger.
type options struct {
	level      Level
	caller     bool
	timestamp  bool
	setStyles  bool // whether Init should apply hex's default color palette
	customized bool // Option was passed; disables Init's default level pick
}

// Option configures the logger. See Init.
type Option func(*options)

// WithLevel sets the minimum level of messages that are emitted.
func WithLevel(level Level) Option {
	return func(o *options) {
		o.level = level
		o.customized = true
	}
}

// WithCaller toggles the file:line caller annotation on each log entry.
func WithCaller(enabled bool) Option {
	return func(o *options) { o.caller = enabled }
}

// WithTimestamp toggles timestamps on each log entry.
func WithTimestamp(enabled bool) Option {
	return func(o *options) { o.timestamp = enabled }
}

// WithoutStyles disables the default hex color palette. Use when the
// consumer wants to install its own styles or produce plain output.
func WithoutStyles() Option {
	return func(o *options) { o.setStyles = false }
}

// Init configures the global charmbracelet logger with hex's defaults:
//   - level FatalLevel (silent until the app opts in)
//   - caller and timestamp disabled
//   - colored level tags: debug blue, info green, warn yellow, error red,
//     fatal cyan
//
// Init is idempotent and safe to call more than once, but the last call wins.
// A typical main calls it exactly once, before hex.App.Bootstrap.
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

	log.SetLevel(cfg.level)
	log.SetReportCaller(cfg.caller)
	log.SetReportTimestamp(cfg.timestamp)

	if cfg.setStyles {
		applyDefaultStyles()
	}
}

// SetLevel updates the minimum log level of the global logger. Callers that
// need to react to a --log-level flag after Init typically do so via this.
func SetLevel(level Level) { log.SetLevel(level) }

// GetLevel returns the current minimum log level.
func GetLevel() Level { return log.GetLevel() }

// ParseLevel converts a string like "info" or "warn" into a Level. Wraps
// the upstream parser so consumers do not need to import charmbracelet
// directly.
func ParseLevel(s string) (Level, error) { return log.ParseLevel(s) }

// Debug logs at DebugLevel.
func Debug(msg string, args ...any) { log.Debug(msg, args...) }

// Info logs at InfoLevel.
func Info(msg string, args ...any) { log.Info(msg, args...) }

// Warn logs at WarnLevel.
func Warn(msg string, args ...any) { log.Warn(msg, args...) }

// Error logs at ErrorLevel.
func Error(msg string, args ...any) { log.Error(msg, args...) }

// Fatal logs at FatalLevel and terminates the process with a non-zero
// exit code. Do not call Fatal from library code; reserve it for main.
func Fatal(msg string, args ...any) { log.Fatal(msg, args...) }

// applyDefaultStyles installs hex's color palette on the charmbracelet
// default logger. Matches finch-cli's existing colors so users switching
// to hex see no visual change.
func applyDefaultStyles() {
	styles := log.DefaultStyles()
	styles.Levels[log.DebugLevel] = styles.Levels[log.DebugLevel].Foreground(lipgloss.Color("4"))
	styles.Levels[log.InfoLevel] = styles.Levels[log.InfoLevel].Foreground(lipgloss.Color("2"))
	styles.Levels[log.WarnLevel] = styles.Levels[log.WarnLevel].Foreground(lipgloss.Color("3"))
	styles.Levels[log.ErrorLevel] = styles.Levels[log.ErrorLevel].Foreground(lipgloss.Color("1"))
	styles.Levels[log.FatalLevel] = styles.Levels[log.FatalLevel].Foreground(lipgloss.Color("6"))

	log.SetStyles(styles)
}
