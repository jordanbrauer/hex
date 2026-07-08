package config

import (
	"sync/atomic"
	"time"

	"github.com/spf13/viper"
)

// Package-level convenience mirrors the finch-cli / finch-bot pattern where
// callers do `config.String("database.dsn")` without threading a *Store
// around. The default store is set by SetDefault (typically from a service
// provider during Register) and read atomically so concurrent access from
// long-running processes is safe.

//nolint:gochecknoglobals // package-level default store is the whole point
var defaultStore atomic.Pointer[Store]

// SetDefault installs s as the package-level default. Subsequent calls to the
// package-level accessors delegate to s. Safe to call at any time; typically
// invoked once from a config provider's Register hook.
func SetDefault(s *Store) { defaultStore.Store(s) }

// Default returns the current package-level Store, or nil if SetDefault has
// not been called.
func Default() *Store { return defaultStore.Load() }

// String returns the string at path in the default store, or "" if no default
// is set.
func String(path string) string {
	if s := defaultStore.Load(); s != nil {
		return s.String(path)
	}

	return ""
}

// Int returns the int at path in the default store, or 0 if no default is set.
func Int(path string) int {
	if s := defaultStore.Load(); s != nil {
		return s.Int(path)
	}

	return 0
}

// Bool returns the bool at path in the default store, or false if no default
// is set.
func Bool(path string) bool {
	if s := defaultStore.Load(); s != nil {
		return s.Bool(path)
	}

	return false
}

// Float64 returns the float64 at path in the default store, or 0.
func Float64(path string) float64 {
	if s := defaultStore.Load(); s != nil {
		return s.Float64(path)
	}

	return 0
}

// Duration returns the duration at path in the default store, or 0.
func Duration(path string) time.Duration {
	if s := defaultStore.Load(); s != nil {
		return s.Duration(path)
	}

	return 0
}

// StringSlice returns the []string at path in the default store, or nil.
func StringSlice(path string) []string {
	if s := defaultStore.Load(); s != nil {
		return s.StringSlice(path)
	}

	return nil
}

// Has reports whether path is set in the default store.
func Has(path string) bool {
	if s := defaultStore.Load(); s != nil {
		return s.Has(path)
	}

	return false
}

// Unmarshal decodes path into target using the default store. Returns an
// error if no default has been set.
func Unmarshal(path string, target any) error {
	if s := defaultStore.Load(); s != nil {
		return s.Unmarshal(path, target)
	}

	return errNoDefault
}

// Set overrides a value in the default store. Returns an error if no default
// has been set or the namespace is unknown.
func Set(path string, value any) error {
	if s := defaultStore.Load(); s != nil {
		return s.Set(path, value)
	}

	return errNoDefault
}

// ViperFor returns the raw *viper.Viper for a namespace in the default store.
// Escape hatch. Returns nil if there is no default or no such namespace.
func ViperFor(namespace string) *viper.Viper {
	if s := defaultStore.Load(); s != nil {
		return s.Viper(namespace)
	}

	return nil
}

// Namespaces returns the sorted namespace list from the default store, or nil.
func Namespaces() []string {
	if s := defaultStore.Load(); s != nil {
		return s.Namespaces()
	}

	return nil
}

// errNoDefault is returned by write-shaped package-level funcs when no
// default has been installed.
var errNoDefault = configError("config: no default store set (call config.SetDefault)")

type configError string

func (e configError) Error() string { return string(e) }
