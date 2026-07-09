package config_test

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/config"
)

//go:embed testdata/defaults/*.toml testdata/env.yaml
var testFS embed.FS

func load(t *testing.T, cfg config.Config) *config.Store {
	t.Helper()

	if len(cfg.Sources) == 0 {
		cfg = config.Config{
			Sources:    []fs.FS{testFS},
			SourcesDir: "testdata/defaults",
		}
	}

	s, err := config.Load(cfg)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}

	return s
}

func TestLoad_defaults_perFileNamespace(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	if got := s.String("database.dsn"); got != "sqlite://./data.db" {
		t.Errorf("database.dsn = %q, want sqlite dsn", got)
	}

	if got := s.Int("database.max_open_conns"); got != 10 {
		t.Errorf("database.max_open_conns = %d, want 10", got)
	}

	if got := s.Int("server.port"); got != 8080 {
		t.Errorf("server.port = %d, want 8080", got)
	}

	if got := s.String("myapp.log.level"); got != "info" {
		t.Errorf("myapp.log.level = %q, want info", got)
	}
}

func TestLoad_missingNamespace(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	if got := s.String("nope.key"); got != "" {
		t.Errorf("String on unknown namespace = %q, want empty", got)
	}

	if s.Has("nope.key") {
		t.Errorf("Has on unknown namespace = true, want false")
	}
}

func TestLoad_pathWithoutNamespaceReturnsZero(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	if got := s.String("nokey"); got != "" {
		t.Errorf("String with no namespace = %q, want empty", got)
	}
}

func TestLoad_userDirOverridesDefaults(t *testing.T) {
	userDir := t.TempDir()

	// Override server.port only. myapp and database are untouched.
	if err := os.WriteFile(
		filepath.Join(userDir, "server.toml"),
		[]byte("port = 9090\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		UserDir:    userDir,
	})

	if got := s.Int("server.port"); got != 9090 {
		t.Errorf("server.port = %d, want 9090 (user override)", got)
	}

	// Untouched key from default still present.
	if got := s.String("server.host"); got != "localhost" {
		t.Errorf("server.host = %q, want localhost (default)", got)
	}

	// Other namespaces unaffected.
	if got := s.String("database.dsn"); got != "sqlite://./data.db" {
		t.Errorf("database.dsn = %q, want default", got)
	}
}

func TestLoad_missingUserDirNotError(t *testing.T) {
	_, err := config.Load(config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		UserDir:    "/nonexistent/path",
	})

	// Missing user dir is fine because we never read files that do not exist.
	if err != nil {
		t.Errorf("Load with missing UserDir returned error: %v", err)
	}
}

func TestLoad_malformedUserFileFails(t *testing.T) {
	userDir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(userDir, "server.toml"),
		[]byte("this is not [ valid toml"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		UserDir:    userDir,
	})
	if err == nil {
		t.Errorf("Load with malformed user file returned nil error")
	}
}

func TestLoad_envOverridesDefaults(t *testing.T) {
	t.Setenv("TEST_SERVER_PORT", "7777")
	t.Setenv("TEST_DATABASE_DSN", "postgres://from-env")

	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		EnvMap:     testFS,
		EnvMapFile: "testdata/env.yaml",
	})

	if got := s.Int("server.port"); got != 7777 {
		t.Errorf("server.port = %d, want 7777 (env)", got)
	}

	if got := s.String("database.dsn"); got != "postgres://from-env" {
		t.Errorf("database.dsn = %q, want env override", got)
	}
}

func TestLoad_envOverridesUserFile(t *testing.T) {
	userDir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(userDir, "server.toml"),
		[]byte("port = 9090\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_SERVER_PORT", "1111")

	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		UserDir:    userDir,
		EnvMap:     testFS,
		EnvMapFile: "testdata/env.yaml",
	})

	if got := s.Int("server.port"); got != 1111 {
		t.Errorf("server.port = %d, want 1111 (env beats user file)", got)
	}
}

func TestLoad_envMapDottedKeyBindsNestedTOML(t *testing.T) {
	// myapp.toml has [log] level; env.yaml binds myapp.log.level -> TEST_LOG_LEVEL
	t.Setenv("TEST_LOG_LEVEL", "debug")

	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		EnvMap:     testFS,
		EnvMapFile: "testdata/env.yaml",
	})

	if got := s.String("myapp.log.level"); got != "debug" {
		t.Errorf("myapp.log.level = %q, want debug (env)", got)
	}
}

func TestLoad_envFileLoads(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	if err := os.WriteFile(envFile, []byte("TEST_LOG_LEVEL=warn\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Unsetenv("TEST_LOG_LEVEL")

	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		EnvMap:     testFS,
		EnvMapFile: "testdata/env.yaml",
		EnvFile:    envFile,
	})

	if got := s.String("myapp.log.level"); got != "warn" {
		t.Errorf("myapp.log.level = %q, want warn (from .env)", got)
	}
}

func TestLoad_missingEnvFileIgnored(t *testing.T) {
	_, err := config.Load(config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		EnvFile:    "/nonexistent/.env",
	})
	if err != nil {
		t.Errorf("Load with missing .env returned error: %v", err)
	}
}

func TestStore_typedAccessors(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	if got := s.Duration("database.timeout"); got != 5*time.Second {
		t.Errorf("database.timeout = %v, want 5s", got)
	}

	if !s.Has("server.port") {
		t.Errorf("Has(server.port) = false, want true")
	}

	if s.Has("server.does_not_exist") {
		t.Errorf("Has(server.does_not_exist) = true, want false")
	}
}

func TestStore_unmarshalKey(t *testing.T) {
	type LogCfg struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	}

	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	var got LogCfg
	if err := s.Unmarshal("myapp.log", &got); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if got.Level != "info" || got.Format != "text" {
		t.Errorf("Unmarshal = %+v, want {info text}", got)
	}
}

func TestStore_unmarshalNamespace(t *testing.T) {
	type ServerCfg struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
	}

	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	var got ServerCfg
	if err := s.Unmarshal("server", &got); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if got.Port != 8080 || got.Host != "localhost" {
		t.Errorf("Unmarshal = %+v, want {8080 localhost}", got)
	}
}

func TestStore_setRuntimeOverride(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	if err := s.Set("server.port", 5555); err != nil {
		t.Fatalf("Set error = %v", err)
	}

	if got := s.Int("server.port"); got != 5555 {
		t.Errorf("after Set, server.port = %d, want 5555", got)
	}
}

func TestStore_setUnknownNamespaceFails(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	if err := s.Set("nope.key", "x"); err == nil {
		t.Errorf("Set on unknown namespace returned nil error")
	}
}

func TestStore_namespaces(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	got := s.Namespaces()
	want := []string{"database", "myapp", "server"}

	if len(got) != len(want) {
		t.Fatalf("Namespaces = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Namespaces[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPriorityOrder(t *testing.T) {
	userDir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(userDir, "myapp.toml"),
		[]byte("[log]\nlevel = \"warn\"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Defaults only → "info".
	s1, _ := config.Load(config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})
	if got := s1.String("myapp.log.level"); got != "info" {
		t.Errorf("defaults only: myapp.log.level = %q, want info", got)
	}

	// + user → "warn".
	s2, _ := config.Load(config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		UserDir:    userDir,
	})
	if got := s2.String("myapp.log.level"); got != "warn" {
		t.Errorf("with user file: myapp.log.level = %q, want warn", got)
	}

	// + env → "error".
	t.Setenv("TEST_LOG_LEVEL", "error")
	s3, _ := config.Load(config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
		UserDir:    userDir,
		EnvMap:     testFS,
		EnvMapFile: "testdata/env.yaml",
	})
	if got := s3.String("myapp.log.level"); got != "error" {
		t.Errorf("with env: myapp.log.level = %q, want error", got)
	}
}
