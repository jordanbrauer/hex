// Package config loads application configuration from embedded TOML files,
// user override files, and environment variables.
//
// Each TOML file is a namespace. A file named "database.toml" creates a
// namespace called "database"; its keys are addressed as
// `config.String("database.<key.path>")`. This mirrors the pattern used in
// production by finch-cli and finch-bot.
//
// Priority order for each namespace (highest wins):
//
//  1. environment variables (mapped via env.yaml)
//  2. user config files with the same filename
//  3. embedded default TOML files
//
// The env.yaml file is a binding declaration, not app config. See ADR-0005.
// Its top-level keys are namespaces; its leaves map dotted config keys to
// environment variable names:
//
//	database:
//	  dsn: MYAPP_DATABASE_DSN
//	log:
//	  level: MYAPP_LOG_LEVEL
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
//	    UserDir:     "~/.config/myapp",
//	})
//	if err != nil { return err }
//
//	dsn := store.String("database.dsn")
//	port := store.Int("server.port")
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

	"cuelang.org/go/cue"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config configures a Load call.
type Config struct {
	// Defaults is an embed.FS containing baseline TOML files, one per
	// namespace. Every *.toml file found under DefaultsDir becomes a
	// namespace whose name is the filename minus ".toml".
	Defaults embed.FS

	// DefaultsDir is the subdirectory within Defaults to scan. Empty means
	// the FS root.
	DefaultsDir string

	// UserDir is an optional directory on disk containing user override
	// files. For each default TOML "<name>.toml", if UserDir contains a
	// file with the same name, it is merged over the default. Leading ~ in
	// UserDir is expanded to the user's home directory. Missing UserDir is
	// not an error.
	UserDir string

	// EnvMap is an embed.FS containing the env-var binding YAML. Optional.
	EnvMap embed.FS

	// EnvMapFile is the path within EnvMap to the binding YAML. When empty,
	// no env-var bindings are applied.
	EnvMapFile string

	// EnvFile is an optional path to a .env file loaded before env-var
	// bindings resolve. Missing files are ignored.
	EnvFile string

	// Schemas is an optional fs.FS containing CUE schema files. When nil,
	// Defaults is scanned for schemas instead — the common case where
	// TOML and schema files live in the same //go:embed'd directory.
	//
	// Recognised schemas:
	//   - schema.cue at the root: whole-tree constraints; top-level
	//     fields describe each namespace.
	//   - <namespace>.cue: per-namespace constraints for the same
	//     namespace as <namespace>.toml. Unified with schema.cue when
	//     both exist.
	Schemas fs.FS

	// SchemasDir is the subdirectory within Schemas that holds .cue
	// files. Empty means the Schemas FS root (or DefaultsDir when Schemas
	// is unset).
	SchemasDir string

	// StrictValidation, when true, causes Load to return an error if any
	// loaded TOML namespace has no matching schema. Off by default —
	// consumers opt into schemas per-namespace.
	StrictValidation bool
}

// Store holds all loaded configuration namespaces. Each namespace is a
// separate *viper.Viper with its own defaults, overrides, and env-var
// bindings. Value access uses dotted paths of the form
// "<namespace>.<key.path>".
type Store struct {
	namespaces map[string]*viper.Viper
	schemas    *schemaSet
}

// Load builds a Store by scanning cfg.Defaults for TOML files, merging any
// matching files from cfg.UserDir, and binding env-var overrides from
// cfg.EnvMap. Returns an error only for malformed inputs; missing files
// (user dir absent, no matching overrides, no .env) are not errors.
func Load(cfg Config) (*Store, error) {
	if cfg.EnvFile != "" {
		if err := loadEnvFile(cfg.EnvFile); err != nil {
			return nil, err
		}
	}

	envBindings, err := loadEnvMap(cfg)
	if err != nil {
		return nil, err
	}

	userDir, err := expandHome(cfg.UserDir)
	if err != nil {
		return nil, fmt.Errorf("config: expand user dir: %w", err)
	}

	s := &Store{namespaces: make(map[string]*viper.Viper)}

	dir := cfg.DefaultsDir
	if dir == "" {
		dir = "."
	}

	entries, err := fs.ReadDir(cfg.Defaults, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}

		return nil, fmt.Errorf("config: read defaults dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".toml")

		v, err := loadNamespace(cfg, dir, entry.Name(), userDir, envBindings[name])
		if err != nil {
			return nil, fmt.Errorf("config: load %s: %w", name, err)
		}

		s.namespaces[name] = v
	}

	if err := s.attachSchemas(cfg); err != nil {
		return nil, err
	}

	return s, nil
}

// attachSchemas loads CUE schemas (per cfg.Schemas / cfg.Defaults) and
// validates every loaded namespace against its schema. Runs after all
// TOML + env merging so validation sees the effective values.
func (s *Store) attachSchemas(cfg Config) error {
	schemaFS := cfg.Schemas
	if schemaFS == nil {
		schemaFS = cfg.Defaults
	}

	schemaDir := cfg.SchemasDir
	if schemaDir == "" {
		schemaDir = cfg.DefaultsDir
	}

	tomlNS := make(map[string]bool, len(s.namespaces))
	for ns := range s.namespaces {
		tomlNS[ns] = true
	}

	set, err := loadSchemas(schemaFS, schemaDir, tomlNS)
	if err != nil {
		return err
	}

	s.schemas = set

	for ns, v := range s.namespaces {
		if !set.hasSchema(ns) {
			if cfg.StrictValidation {
				return fmt.Errorf("config: strict validation: no schema for namespace %q", ns)
			}

			continue
		}

		values := v.AllSettings()
		if err := set.validate(ns, values); err != nil {
			return err
		}
	}

	return nil
}

// loadNamespace loads defaults for one namespace from the embedded FS, merges
// a matching user override file if present, and installs env-var bindings.
func loadNamespace(cfg Config, embedDir, filename, userDir string, bindings map[string]string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigType("toml")

	data, err := fs.ReadFile(cfg.Defaults, path.Join(embedDir, filename))
	if err != nil {
		return nil, fmt.Errorf("read default: %w", err)
	}

	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("parse default: %w", err)
	}

	if userDir != "" {
		userPath := filepath.Join(userDir, filename)

		userData, err := os.ReadFile(userPath)
		switch {
		case err == nil:
			if err := v.MergeConfig(bytes.NewReader(userData)); err != nil {
				return nil, fmt.Errorf("merge user file %s: %w", userPath, err)
			}
		case errors.Is(err, fs.ErrNotExist):
			// no override: fine
		default:
			return nil, fmt.Errorf("read user file %s: %w", userPath, err)
		}
	}

	for key, envVar := range bindings {
		if err := v.BindEnv(key, envVar); err != nil {
			return nil, fmt.Errorf("bind %s -> %s: %w", key, envVar, err)
		}
	}

	return v, nil
}

// loadEnvMap parses cfg.EnvMap/cfg.EnvMapFile and returns a nested map of
// namespace -> dotted config key -> env var name.
func loadEnvMap(cfg Config) (map[string]map[string]string, error) {
	if cfg.EnvMapFile == "" {
		return nil, nil
	}

	data, err := fs.ReadFile(cfg.EnvMap, cfg.EnvMapFile)
	if err != nil {
		return nil, fmt.Errorf("config: read env map %s: %w", cfg.EnvMapFile, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("config: parse env map %s: %w", cfg.EnvMapFile, err)
	}

	out := make(map[string]map[string]string)
	for namespace, val := range raw {
		flat := make(map[string]string)
		flattenEnvMap("", val, flat)

		if len(flat) > 0 {
			out[namespace] = flat
		}
	}

	return out, nil
}

// flattenEnvMap turns a nested YAML branch into dotted keys mapped to env-var
// names. Non-string leaves are ignored — env-var names must be strings.
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

// loadEnvFile applies a .env file if present. Missing files are ignored;
// malformed files return an error. OS env wins over .env.
func loadEnvFile(pathTo string) error {
	err := godotenv.Load(pathTo)
	if err == nil {
		return nil
	}

	var pathErr *os.PathError

	if errors.As(err, &pathErr) && errors.Is(pathErr.Err, fs.ErrNotExist) {
		return nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("config: load env file %s: %w", pathTo, err)
}

// expandHome expands a leading ~ in p to the user's home directory. Empty p
// returns empty; p without a leading ~ is returned unchanged.
func expandHome(p string) (string, error) {
	if p == "" {
		return "", nil
	}

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

// source splits a dotted path into (namespace, remaining key). The namespace
// must be the first segment. Returns an error if the path lacks a "." or
// the namespace is unknown.
func (s *Store) source(fullpath string) (*viper.Viper, string, error) {
	segments := strings.SplitN(fullpath, ".", 2)
	if len(segments) < 2 {
		return nil, "", fmt.Errorf("config: path %q missing namespace (expected \"<namespace>.<key>\")", fullpath)
	}

	v, ok := s.namespaces[segments[0]]
	if !ok {
		return nil, "", fmt.Errorf("config: unknown namespace %q", segments[0])
	}

	return v, segments[1], nil
}

// Namespaces returns the sorted list of loaded namespace names. Useful for
// diagnostics and for consumers wiring commands like `myapp config list`.
func (s *Store) Namespaces() []string {
	out := make([]string, 0, len(s.namespaces))
	for k := range s.namespaces {
		out = append(out, k)
	}

	// sort in place; small n, this is fine
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}

	return out
}

// Has reports whether path resolves to a set value.
func (s *Store) Has(fullpath string) bool {
	v, key, err := s.source(fullpath)
	if err != nil {
		return false
	}

	return v.IsSet(key)
}

// Viper returns the underlying *viper.Viper for a namespace, or nil if the
// namespace is not loaded. Escape hatch for consumers that need Viper
// methods hex does not expose.
func (s *Store) Viper(namespace string) *viper.Viper {
	return s.namespaces[namespace]
}

// Schema returns the merged CUE schema value for a namespace, or the
// zero cue.Value when no schema was registered. Useful for doc
// generation or programmatic constraint inspection.
func (s *Store) Schema(namespace string) cue.Value {
	return s.schemas.schemaValue(namespace)
}

// Validate re-runs schema validation against the current in-memory
// values for a namespace. Runtime overrides via Set() bypass validation
// by default; call Validate to check them explicitly.
func (s *Store) Validate(namespace string) error {
	if s.schemas == nil || !s.schemas.hasSchema(namespace) {
		return nil
	}

	v, ok := s.namespaces[namespace]
	if !ok {
		return fmt.Errorf("config: unknown namespace %q", namespace)
	}

	return s.schemas.validate(namespace, v.AllSettings())
}

// -- Typed accessors -------------------------------------------------------
//
// Each accessor takes a dotted path "<namespace>.<key.path>". Missing values
// return the zero value. Missing or malformed paths return the zero value —
// use Has() first when the distinction matters.

// String returns the string at path, or "" if unset or invalid.
func (s *Store) String(fullpath string) string {
	v, key, err := s.source(fullpath)
	if err != nil {
		return ""
	}

	return v.GetString(key)
}

// Int returns the int at path, or 0 if unset or invalid.
func (s *Store) Int(fullpath string) int {
	v, key, err := s.source(fullpath)
	if err != nil {
		return 0
	}

	return v.GetInt(key)
}

// Bool returns the bool at path, or false if unset or invalid.
func (s *Store) Bool(fullpath string) bool {
	v, key, err := s.source(fullpath)
	if err != nil {
		return false
	}

	return v.GetBool(key)
}

// Float64 returns the float64 at path, or 0 if unset or invalid.
func (s *Store) Float64(fullpath string) float64 {
	v, key, err := s.source(fullpath)
	if err != nil {
		return 0
	}

	return v.GetFloat64(key)
}

// Duration returns the time.Duration at path, parsed via time.ParseDuration
// on strings like "5s" or "1h30m". Zero on error.
func (s *Store) Duration(fullpath string) time.Duration {
	v, key, err := s.source(fullpath)
	if err != nil {
		return 0
	}

	return v.GetDuration(key)
}

// StringSlice returns the []string at path, or nil.
func (s *Store) StringSlice(fullpath string) []string {
	v, key, err := s.source(fullpath)
	if err != nil {
		return nil
	}

	return v.GetStringSlice(key)
}

// Unmarshal decodes the value at path into target (a pointer). To unmarshal
// an entire namespace, pass "<namespace>".
func (s *Store) Unmarshal(fullpath string, target any) error {
	// A bare namespace with no dot decodes the whole namespace.
	if !strings.Contains(fullpath, ".") {
		v, ok := s.namespaces[fullpath]
		if !ok {
			return fmt.Errorf("config: unknown namespace %q", fullpath)
		}

		return v.Unmarshal(target)
	}

	v, key, err := s.source(fullpath)
	if err != nil {
		return err
	}

	return v.UnmarshalKey(key, target)
}

// Set overrides a value at runtime. Useful for tests; production paths should
// prefer files or env vars.
func (s *Store) Set(fullpath string, value any) error {
	v, key, err := s.source(fullpath)
	if err != nil {
		return err
	}

	v.Set(key, value)

	return nil
}
