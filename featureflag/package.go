package featureflag

import (
	"sync/atomic"
)

// Package-level convenience mirrors hex/config and hex/i18n. Install a Client
// once via SetDefault; then Bool/Int/String/Float64/JSON evaluate flags
// against that client without threading it through every call site.

//nolint:gochecknoglobals // package-level default client is the whole point
var defaultClient atomic.Pointer[Client]

// SetDefault installs c as the package-level default client. Subsequent calls
// to Bool/Int/String/Float64/JSON delegate to c. Safe to call more than once.
func SetDefault(c *Client) { defaultClient.Store(c) }

// Default returns the current default Client, or nil if none is set.
func Default() *Client { return defaultClient.Load() }

// Bool evaluates flag as a boolean. Returns defaultValue if the flag is
// missing, the default client is unset, or evaluation fails.
func Bool(flag string, ctx Context, defaultValue bool) bool {
	c := defaultClient.Load()
	if c == nil {
		return defaultValue
	}

	v, err := c.BoolVariation(flag, ctx, defaultValue)
	if err != nil {
		return defaultValue
	}

	return v
}

// Int evaluates flag as an int.
func Int(flag string, ctx Context, defaultValue int) int {
	c := defaultClient.Load()
	if c == nil {
		return defaultValue
	}

	v, err := c.IntVariation(flag, ctx, defaultValue)
	if err != nil {
		return defaultValue
	}

	return v
}

// String evaluates flag as a string.
func String(flag string, ctx Context, defaultValue string) string {
	c := defaultClient.Load()
	if c == nil {
		return defaultValue
	}

	v, err := c.StringVariation(flag, ctx, defaultValue)
	if err != nil {
		return defaultValue
	}

	return v
}

// Float64 evaluates flag as a float64.
func Float64(flag string, ctx Context, defaultValue float64) float64 {
	c := defaultClient.Load()
	if c == nil {
		return defaultValue
	}

	v, err := c.Float64Variation(flag, ctx, defaultValue)
	if err != nil {
		return defaultValue
	}

	return v
}

// JSONArray evaluates flag as a JSON array.
func JSONArray(flag string, ctx Context, defaultValue []any) []any {
	c := defaultClient.Load()
	if c == nil {
		return defaultValue
	}

	v, err := c.JSONArrayVariation(flag, ctx, defaultValue)
	if err != nil {
		return defaultValue
	}

	return v
}

// JSON evaluates flag as a JSON object.
func JSON(flag string, ctx Context, defaultValue map[string]any) map[string]any {
	c := defaultClient.Load()
	if c == nil {
		return defaultValue
	}

	v, err := c.JSONVariation(flag, ctx, defaultValue)
	if err != nil {
		return defaultValue
	}

	return v
}
