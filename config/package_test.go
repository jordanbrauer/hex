package config_test

import (
	"io/fs"
	"testing"

	"github.com/jordanbrauer/hex/config"
)

func TestPackageLevel_zeroValuesWhenNoDefault(t *testing.T) {
	// Ensure clean slate.
	config.SetDefault(nil)

	if got := config.String("example.log.level"); got != "" {
		t.Errorf("String w/o default = %q, want empty", got)
	}

	if got := config.Int("server.port"); got != 0 {
		t.Errorf("Int w/o default = %d, want 0", got)
	}

	if got := config.Bool("x.y"); got != false {
		t.Errorf("Bool w/o default = %v, want false", got)
	}

	if config.Has("anything.here") {
		t.Errorf("Has w/o default = true, want false")
	}

	if got := config.Namespaces(); got != nil {
		t.Errorf("Namespaces w/o default = %v, want nil", got)
	}

	if err := config.Unmarshal("x", &struct{}{}); err == nil {
		t.Errorf("Unmarshal w/o default returned nil error")
	}

	if err := config.Set("server.port", 1); err == nil {
		t.Errorf("Set w/o default returned nil error")
	}
}

func TestPackageLevel_delegatesToDefault(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	config.SetDefault(s)
	t.Cleanup(func() { config.SetDefault(nil) })

	if got := config.String("database.dsn"); got != "sqlite://./data.db" {
		t.Errorf("String = %q, want sqlite dsn", got)
	}

	if got := config.Int("server.port"); got != 8080 {
		t.Errorf("Int = %d, want 8080", got)
	}

	if got := config.String("example.log.level"); got != "info" {
		t.Errorf("String example.log.level = %q, want info", got)
	}

	if !config.Has("server.port") {
		t.Errorf("Has(server.port) = false, want true")
	}

	if got := config.Default(); got != s {
		t.Errorf("Default() != the store we set")
	}
}

func TestPackageLevel_setUpdatesDefault(t *testing.T) {
	s := load(t, config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata/defaults",
	})

	config.SetDefault(s)
	t.Cleanup(func() { config.SetDefault(nil) })

	if err := config.Set("server.port", 4242); err != nil {
		t.Fatalf("Set error = %v", err)
	}

	if got := config.Int("server.port"); got != 4242 {
		t.Errorf("after Set, Int = %d, want 4242", got)
	}
}
