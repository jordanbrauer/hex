package config_test

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/config"
)

//go:embed testdata/defaults/*.toml testdata/env.yaml
var testFS embed.FS

func TestLoad_defaultsOnly(t *testing.T) {
	s, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
	})
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if got := s.String("server.host"); got != "localhost" {
		t.Errorf("server.host = %q, want %q", got, "localhost")
	}

	if got := s.Int("server.port"); got != 8080 {
		t.Errorf("server.port = %d, want 8080", got)
	}

	if got := s.String("database.dsn"); got != "sqlite://./data.db" {
		t.Errorf("database.dsn = %q, want sqlite dsn", got)
	}

	if got := s.Duration("database.timeout"); got != 5*time.Second {
		t.Errorf("database.timeout = %v, want 5s", got)
	}
}

func TestLoad_emptyConfigIsUsable(t *testing.T) {
	s, err := config.Load(config.Config{})
	if err != nil {
		t.Fatalf("Load(zero) error = %v", err)
	}

	if got := s.String("nope"); got != "" {
		t.Errorf("String on empty store = %q, want empty", got)
	}

	if s.IsSet("nope") {
		t.Errorf("IsSet(\"nope\") = true on empty store")
	}
}

func TestLoad_userFileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	override := filepath.Join(dir, "override.toml")
	if err := os.WriteFile(override, []byte("[server]\nport = 9090\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
		Files:       []string{override},
	})
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if got := s.Int("server.port"); got != 9090 {
		t.Errorf("server.port = %d, want 9090", got)
	}

	// Untouched key from defaults still present.
	if got := s.String("server.host"); got != "localhost" {
		t.Errorf("server.host = %q, want localhost", got)
	}
}

func TestLoad_missingUserFileIsSkipped(t *testing.T) {
	_, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
		Files:       []string{"/nonexistent/path/config.toml"},
	})
	if err != nil {
		t.Errorf("Load with missing file returned error: %v", err)
	}
}

func TestLoad_malformedUserFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("this is not [ toml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(config.Config{Files: []string{bad}})
	if err == nil {
		t.Error("Load with malformed file returned nil error")
	}
}

func TestLoad_envOverridesDefaults(t *testing.T) {
	t.Setenv("TEST_SERVER_PORT", "7777")
	t.Setenv("TEST_DATABASE_DSN", "postgres://from-env")

	s, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
		EnvMap:      testFS,
		EnvMapFile:  "testdata/env.yaml",
	})
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if got := s.Int("server.port"); got != 7777 {
		t.Errorf("server.port = %d, want 7777 (from env)", got)
	}

	if got := s.String("database.dsn"); got != "postgres://from-env" {
		t.Errorf("database.dsn = %q, want env override", got)
	}
}

func TestLoad_envOverridesUserFile(t *testing.T) {
	dir := t.TempDir()
	override := filepath.Join(dir, "override.toml")
	if err := os.WriteFile(override, []byte("[server]\nport = 9090\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_SERVER_PORT", "1111")

	s, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
		Files:       []string{override},
		EnvMap:      testFS,
		EnvMapFile:  "testdata/env.yaml",
	})
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if got := s.Int("server.port"); got != 1111 {
		t.Errorf("server.port = %d, want 1111 (env wins over user file)", got)
	}
}

func TestLoad_envFileLoads(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("TEST_LOG_LEVEL=debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ensure real env doesn't already have it.
	os.Unsetenv("TEST_LOG_LEVEL")

	s, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
		EnvMap:      testFS,
		EnvMapFile:  "testdata/env.yaml",
		EnvFile:     envFile,
	})
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if got := s.String("log.level"); got != "debug" {
		t.Errorf("log.level = %q, want debug (from .env)", got)
	}
}

func TestLoad_missingEnvFileIsIgnored(t *testing.T) {
	_, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
		EnvFile:     "/nonexistent/.env",
	})
	if err != nil {
		t.Errorf("Load with missing .env returned error: %v", err)
	}
}

func TestUnmarshal_typedStruct(t *testing.T) {
	type Server struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
	}

	s, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
	})
	if err != nil {
		t.Fatal(err)
	}

	var got Server
	if err := s.Unmarshal("server", &got); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if got.Port != 8080 || got.Host != "localhost" {
		t.Errorf("Unmarshal = %+v, want {Port:8080 Host:localhost}", got)
	}
}

func TestSet_runtimeOverride(t *testing.T) {
	s, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.Set("server.port", 5555)

	if got := s.Int("server.port"); got != 5555 {
		t.Errorf("after Set, port = %d, want 5555", got)
	}
}

func TestPriorityOrder_envBeatsUserBeatsDefaults(t *testing.T) {
	// Explicit end-to-end priority verification.
	dir := t.TempDir()
	userFile := filepath.Join(dir, "user.toml")
	if err := os.WriteFile(userFile, []byte("[log]\nlevel = \"warn\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Baseline: defaults only → "info" from app.toml.
	s1, _ := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
	})
	if got := s1.String("log.level"); got != "info" {
		t.Errorf("defaults only: log.level = %q, want info", got)
	}

	// + user file → "warn".
	s2, _ := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
		Files:       []string{userFile},
	})
	if got := s2.String("log.level"); got != "warn" {
		t.Errorf("defaults + user: log.level = %q, want warn", got)
	}

	// + env → "error".
	t.Setenv("TEST_LOG_LEVEL", "error")
	s3, _ := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
		Files:       []string{userFile},
		EnvMap:      testFS,
		EnvMapFile:  "testdata/env.yaml",
	})
	if got := s3.String("log.level"); got != "error" {
		t.Errorf("defaults + user + env: log.level = %q, want error", got)
	}
}
