package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/jordanbrauer/hex/config"
)

// writeEnv drops a .env-style file at path and registers a cleanup to
// unset every key it contained (so tests don't leak env vars into
// each other).
func writeEnv(t *testing.T, path string, keyvals ...string) {
	t.Helper()

	if len(keyvals)%2 != 0 {
		t.Fatalf("writeEnv: keyvals must be pairs, got %d entries", len(keyvals))
	}

	var body []byte
	for i := 0; i < len(keyvals); i += 2 {
		body = append(body, []byte(keyvals[i]+"="+keyvals[i+1]+"\n")...)
	}

	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	t.Cleanup(func() {
		for i := 0; i < len(keyvals); i += 2 {
			os.Unsetenv(keyvals[i])
		}
	})
}

// baseSources returns a source with a base database.toml + env.yaml
// binding every field we test on.
func baseSources() fs.FS {
	return fstest.MapFS{
		"database.toml": &fstest.MapFile{Data: []byte(`
driver = "sqlite"
dsn = "./data/app.db"

[pool]
max_open_conns = 25
`)},
		"env.yaml": &fstest.MapFile{Data: []byte(`
database:
  driver: DATABASE_DRIVER
  dsn: DATABASE_DSN
  pool:
    max_open_conns: DATABASE_POOL_MAX_OPEN_CONNS
`)},
	}
}

func TestDotEnvEnvOverlay_layersOverBaseEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	envTestPath := envPath + ".test"

	// Base .env sets a dev value; overlay .env.test overrides it.
	writeEnv(t, envPath,
		"DATABASE_DSN", "./data/dev.db",
	)
	writeEnv(t, envTestPath,
		"DATABASE_DSN", ":memory:",
		"DATABASE_POOL_MAX_OPEN_CONNS", "1",
	)

	src := baseSources()

	store, err := config.Load(config.Config{
		Sources:     []fs.FS{src},
		EnvMap:      src,
		EnvMapFile:  "env.yaml",
		EnvFile:     envPath,
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := store.String("database.dsn"); got != ":memory:" {
		t.Errorf("dsn = %q, want :memory: (from .env.test)", got)
	}

	if got := store.Int("database.pool.max_open_conns"); got != 1 {
		t.Errorf("max_open_conns = %d, want 1 (from .env.test)", got)
	}

	// Fields untouched by .env.test should fall back to base TOML.
	if got := store.String("database.driver"); got != "sqlite" {
		t.Errorf("driver = %q, want sqlite (from base TOML)", got)
	}
}

func TestDotEnvEnvOverlay_osEnvBeatsBothFiles(t *testing.T) {
	// OS env is the ultimate winner regardless of what .env files
	// say. godotenv's default no-overwrite semantics guarantee this.
	t.Setenv("DATABASE_DSN", "os-env-wins")

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	writeEnv(t, envPath+".test", "DATABASE_DSN", ":memory:")
	writeEnv(t, envPath, "DATABASE_DSN", "./data/app.db")

	src := baseSources()

	store, err := config.Load(config.Config{
		Sources:     []fs.FS{src},
		EnvMap:      src,
		EnvMapFile:  "env.yaml",
		EnvFile:     envPath,
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := store.String("database.dsn"); got != "os-env-wins" {
		t.Errorf("dsn = %q, want os-env-wins", got)
	}
}

func TestDotEnvEnvOverlay_missingOverlayNotAnError(t *testing.T) {
	// .env.production doesn't exist — Load should not fail.
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	writeEnv(t, envPath, "DATABASE_DSN", "./data/dev.db")

	src := baseSources()

	store, err := config.Load(config.Config{
		Sources:     []fs.FS{src},
		EnvMap:      src,
		EnvMapFile:  "env.yaml",
		EnvFile:     envPath,
		Environment: "production", // no .env.production on disk
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := store.String("database.dsn"); got != "./data/dev.db" {
		t.Errorf("dsn = %q, want ./data/dev.db (from base .env)", got)
	}
}

func TestDotEnvEnvOverlay_cueValidatesEnvBoundValues(t *testing.T) {
	// The whole point of leaning on env.yaml: CUE catches bad env
	// values at Load time, same as bad TOML values.
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	writeEnv(t, envPath+".test",
		"DATABASE_DRIVER", "gibberish",
		"DATABASE_DSN", ":memory:",
	)

	src := fstest.MapFS{
		"database.toml": &fstest.MapFile{Data: []byte(`
driver = "sqlite"
dsn = "./data/app.db"
`)},
		"database.cue": &fstest.MapFile{Data: []byte(`
driver!: "sqlite" | "postgres"
dsn!:    string & !=""
`)},
		"env.yaml": &fstest.MapFile{Data: []byte(`
database:
  driver: DATABASE_DRIVER
  dsn: DATABASE_DSN
`)},
	}

	_, err := config.Load(config.Config{
		Sources:     []fs.FS{src},
		EnvMap:      src,
		EnvMapFile:  "env.yaml",
		EnvFile:     envPath,
		Environment: "test",
	})
	if err == nil {
		t.Fatalf("Load succeeded; want CUE validation error for driver=gibberish")
	}
}
