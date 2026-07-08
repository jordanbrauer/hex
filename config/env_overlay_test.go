package config_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/jordanbrauer/hex/config"
)

func TestEnvironment_overlayMergesOverBase(t *testing.T) {
	src := fstest.MapFS{
		"database.toml": &fstest.MapFile{Data: []byte(`
driver = "sqlite"
dsn = "./data/app.db"

[pool]
max_open_conns = 25
`)},
		"database.test.toml": &fstest.MapFile{Data: []byte(`
dsn = ":memory:"

[pool]
max_open_conns = 1
`)},
		"database.production.toml": &fstest.MapFile{Data: []byte(`
driver = "postgres"
dsn = "postgres://prod"
`)},
	}

	tests := map[string]struct {
		env         string
		wantDSN     string
		wantDriver  string
		wantMaxConn int
	}{
		"no env — base only": {
			env: "", wantDSN: "./data/app.db", wantDriver: "sqlite", wantMaxConn: 25,
		},
		"test env picks memory dsn + pool": {
			env: "test", wantDSN: ":memory:", wantDriver: "sqlite", wantMaxConn: 1,
		},
		"production env picks postgres": {
			env: "production", wantDSN: "postgres://prod", wantDriver: "postgres", wantMaxConn: 25,
		},
		"unrecognised env — base only": {
			env: "staging", wantDSN: "./data/app.db", wantDriver: "sqlite", wantMaxConn: 25,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, err := config.Load(config.Config{
				Sources:     []fs.FS{src},
				Environment: tc.env,
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			if got := store.String("database.dsn"); got != tc.wantDSN {
				t.Errorf("dsn = %q, want %q", got, tc.wantDSN)
			}

			if got := store.String("database.driver"); got != tc.wantDriver {
				t.Errorf("driver = %q, want %q", got, tc.wantDriver)
			}

			if got := store.Int("database.pool.max_open_conns"); got != tc.wantMaxConn {
				t.Errorf("pool.max_open_conns = %d, want %d", got, tc.wantMaxConn)
			}
		})
	}
}

func TestEnvironment_overlayOnlyNoBase(t *testing.T) {
	// A source can contribute a namespace via overlay only (no base).
	// Uncommon but should not break.
	src := fstest.MapFS{
		"logs.test.toml": &fstest.MapFile{Data: []byte(`level = "debug"`)},
	}

	store, err := config.Load(config.Config{
		Sources:     []fs.FS{src},
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := store.String("logs.level"); got != "debug" {
		t.Errorf("logs.level = %q, want \"debug\"", got)
	}
}

func TestEnvironment_multiSourceOverlayFromDifferentSource(t *testing.T) {
	// Base lives in source A, overlay in source B. Load should stitch
	// them together for the active env.
	frameworkFS := fstest.MapFS{
		"cache.toml": &fstest.MapFile{Data: []byte(`driver = "memory"
size = 100`)},
	}

	appFS := fstest.MapFS{
		"cache.test.toml": &fstest.MapFile{Data: []byte(`size = 5`)},
	}

	store, err := config.Load(config.Config{
		Sources:     []fs.FS{frameworkFS, appFS},
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := store.String("cache.driver"); got != "memory" {
		t.Errorf("driver = %q, want \"memory\" (framework default)", got)
	}

	if got := store.Int("cache.size"); got != 5 {
		t.Errorf("size = %d, want 5 (app test overlay)", got)
	}
}
