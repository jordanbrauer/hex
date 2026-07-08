// Package config loads application configuration from embedded TOML defaults,
// user override files, and environment variables.
//
// The Store is a thin wrapper over Viper that enforces one convention:
// application configuration is TOML, environment variable bindings are
// declared in a separate YAML file (a "binding declaration", not app
// config). See ADR-0005.
//
// Priority order (highest wins):
//
//  1. environment variables (mapped via EnvMap)
//  2. user config files (Files, later files override earlier)
//  3. embedded defaults (Defaults + DefaultsDir)
//
// Example:
//
//	//go:embed defaults/*.toml env.yaml
//	var configFS embed.FS
//
//	store, err := config.Load(config.Config{
//	    Defaults:    configFS,
//	    DefaultsDir: "defaults",
//	    EnvMap:      configFS,
//	    EnvMapFile:  "env.yaml",
//	    Files:       []string{"~/.config/myapp/config.toml"},
//	    EnvFile:     ".env",
//	})
package config

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config configures a Load call. All fields are optional; the zero value
// produces an empty Store that reads only from environment variables (if
// none are mapped, it reads nothing).
type Config struct {
	// Files are absolute or ~-prefixed paths to user config TOML files, merged
	// in order. Missing files are skipped without error; malformed files
	// return an error.
	Files []string

	// Defaults is an optional embed.FS containing baseline TOML files.
	Defaults embed.FS

	// DefaultsDir is the subdirectory within Defaults to scan for *.toml
	// files. Empty means the FS root. Every TOML file found is merged into
	// the store; filenames are informational only.
	DefaultsDir string

	// EnvMap is an optional embed.FS containing the env-var binding YAML.
	EnvMap embed.FS

	// EnvMapFile is the path within EnvMap to the binding YAML. When empty,
	// no env-var bindings are applied. The file format is a nested map from
	// dotted config key to environment variable name:
	//
	//   database:
	//     dsn: MYAPP_DATABASE_DSN
	//   server:
	//     port: MYAPP_PORT
	EnvMapFile string

	// EnvFile is an optional path to a .env file loaded before env-var
	// bindings resolve. Missing files are ignored. Existing OS env vars
	// win over .env (godotenv Load semantics).
	EnvFile string
}

// Store holds a loaded configuration. It is a thin wrapper over a single
// *viper.Viper; use Viper() to escape to the underlying instance when
// necessary.
type Store struct {
	v *viper.Viper
}

// Load builds a Store by applying Defaults, then Files, then env-var bindings,
// in that order. Later sources override earlier ones for the same key.
func Load(cfg Config) (*Store, error) {
	v := viper.New()
	v.SetConfigType("toml")

	if err := loadDefaults(v, cfg); err != nil {
		return nil, fmt.Errorf("config: load defaults: %w", err)
	}

	if err := loadFiles(v, cfg.Files); err != nil {
		return nil, fmt.Errorf("config: load files: %w", err)
	}

	if err := loadEnv(v, cfg); err != nil {
		return nil, fmt.Errorf("config: bind env: %w", err)
	}

	return &Store{v: v}, nil
}

// loadDefaults reads every *.toml under cfg.DefaultsDir in cfg.Defaults and
// merges its contents into v. Files are processed in name order for
// deterministic merge ordering.
func loadDefaults(v *viper.Viper, cfg Config) error {
	dir := cfg.DefaultsDir
	if dir == "" {
		dir = "."
	}

	entries, err := fs.ReadDir(cfg.Defaults, dir)
	if err != nil {
		// An empty embed.FS returns "file does not exist" on ReadDir. Treat
		// that as "no defaults" rather than an error.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}

		p := path.Join(dir, entry.Name())

		data, err := fs.ReadFile(cfg.Defaults, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}

		if err := v.MergeConfig(bytes.NewReader(data)); err != nil {
			return fmt.Errorf("merge %s: %w", p, err)
		}
	}

	return nil
}

// loadFiles merges each user config file into v, expanding leading ~. Missing
// files are skipped. Malformed files return an error.
func loadFiles(v *viper.Viper, files []string) error {
	for _, f := range files {
		expanded, err := expandHome(f)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(expanded)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return fmt.Errorf("read %s: %w", expanded, err)
		}

		if err := v.MergeConfig(bytes.NewReader(data)); err != nil {
			return fmt.Errorf("merge %s: %w", expanded, err)
		}
	}

	return nil
}

// loadEnv applies the .env file (if any) and installs env-var bindings from
// the YAML declaration.
func loadEnv(v *viper.Viper, cfg Config) error {
	if cfg.EnvFile != "" {
		if err := godotenv.Load(cfg.EnvFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
			// godotenv wraps as *os.PathError; unwrap for the sentinel check.
			var pathErr *os.PathError
			if !(errors.As(err, &pathErr) && errors.Is(pathErr.Err, fs.ErrNotExist)) {
				return fmt.Errorf("load env file %s: %w", cfg.EnvFile, err)
			}
		}
	}

	if cfg.EnvMapFile == "" {
		return nil
	}

	data, err := fs.ReadFile(cfg.EnvMap, cfg.EnvMapFile)
	if err != nil {
		return fmt.Errorf("read env map %s: %w", cfg.EnvMapFile, err)
	}

	bindings, err := parseEnvMap(data)
	if err != nil {
		return fmt.Errorf("parse env map %s: %w", cfg.EnvMapFile, err)
	}

	for key, envVar := range bindings {
		if err := v.BindEnv(key, envVar); err != nil {
			return fmt.Errorf("bind %s → %s: %w", key, envVar, err)
		}
	}

	return nil
}

// parseEnvMap flattens the nested YAML into a map of dotted keys to env-var
// names. This is the format both finch-cli and finch-bot use in production.
func parseEnvMap(data []byte) (map[string]string, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	out := make(map[string]string)
	flattenEnvMap("", raw, out)

	return out, nil
}

// flattenEnvMap walks the parsed YAML tree, joining keys with "." and
// collecting leaves that are strings. Non-string leaves are ignored — the
// env-var name must be a string.
func flattenEnvMap(prefix string, node any, out map[string]string) {
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			next := k
			if prefix != "" {
				next = prefix + "." + k
			}

			flattenEnvMap(next, v, out)
		}
	case string:
		if prefix != "" {
			out[prefix] = n
		}
	}
}

// expandHome expands a leading ~/ to the user's home directory. Paths not
// starting with ~/ are returned unchanged.
func expandHome(p string) (string, error) {
	if !strings.HasPrefix(p, "~/") && p != "~" {
		return p, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if p == "~" {
		return home, nil
	}

	return filepath.Join(home, p[2:]), nil
}

// -- Accessors -------------------------------------------------------------

// String returns the string value for key, or "" if unset.
func (s *Store) String(key string) string { return s.v.GetString(key) }

// Int returns the int value for key, or 0 if unset.
func (s *Store) Int(key string) int { return s.v.GetInt(key) }

// Bool returns the bool value for key, or false if unset.
func (s *Store) Bool(key string) bool { return s.v.GetBool(key) }

// Float64 returns the float64 value for key, or 0 if unset.
func (s *Store) Float64(key string) float64 { return s.v.GetFloat64(key) }

// Duration returns the time.Duration value for key. Viper parses strings like
// "5s", "1h30m" per time.ParseDuration.
func (s *Store) Duration(key string) time.Duration { return s.v.GetDuration(key) }

// StringSlice returns the []string value for key, or nil if unset.
func (s *Store) StringSlice(key string) []string { return s.v.GetStringSlice(key) }

// IsSet reports whether key was explicitly set by any source.
func (s *Store) IsSet(key string) bool { return s.v.IsSet(key) }

// Unmarshal decodes the value at key into target, which must be a pointer.
// Passing an empty key unmarshals the entire store.
func (s *Store) Unmarshal(key string, target any) error {
	if key == "" {
		return s.v.Unmarshal(target)
	}

	return s.v.UnmarshalKey(key, target)
}

// Set overrides a value at runtime. Useful for tests; consumer apps should
// prefer file or env-based configuration.
func (s *Store) Set(key string, value any) { s.v.Set(key, value) }

// Viper returns the underlying *viper.Viper. Escape hatch — prefer the typed
// accessors when possible so consumers can be ported to a different
// backend later without touching call sites.
func (s *Store) Viper() *viper.Viper { return s.v }
