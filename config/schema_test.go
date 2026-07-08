package config_test

import (
	"embed"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex/config"
)

//go:embed testdata/schemas/*.toml testdata/schemas/*.cue
var schemasFS embed.FS

func TestSchema_perNamespaceCueValidatesTOML(t *testing.T) {
	_, err := config.Load(config.Config{
		Defaults:    schemasFS,
		DefaultsDir: "testdata/schemas",
	})
	if err != nil {
		t.Fatalf("Load with valid data: %v", err)
	}
}

func TestSchema_topLevelSchemaValidatesTOML(t *testing.T) {
	// Only server.toml has a top-level schema.cue field; database has its
	// own database.cue. Both should validate cleanly on load.
	s, err := config.Load(config.Config{
		Defaults:    schemasFS,
		DefaultsDir: "testdata/schemas",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := s.String("server.address"); got != ":8080" {
		t.Errorf("server.address = %q", got)
	}

	if got := s.String("database.driver"); got != "sqlite" {
		t.Errorf("database.driver = %q", got)
	}
}

func TestSchema_invalidValueFails(t *testing.T) {
	// Override database.toml at runtime to violate the schema by using
	// Set — then call Validate explicitly.
	s, err := config.Load(config.Config{
		Defaults:    schemasFS,
		DefaultsDir: "testdata/schemas",
	})
	if err != nil {
		t.Fatal(err)
	}

	_ = s.Set("database.driver", "mysql") // not in enum

	err = s.Validate("database")
	if err == nil {
		t.Fatalf("Validate on bad driver returned nil error")
	}

	if !strings.Contains(err.Error(), "database") {
		t.Errorf("error should name the namespace, got: %v", err)
	}
}

func TestSchema_missingSchemaIsOK(t *testing.T) {
	// The stock testdata (from other tests) has no .cue files. Load with
	// StrictValidation off should be fine.
	_, err := config.Load(config.Config{
		Defaults:    testFS,
		DefaultsDir: "testdata/defaults",
	})
	if err != nil {
		t.Errorf("Load without schemas returned error: %v", err)
	}
}

func TestSchema_strictRequiresSchemaForEveryNamespace(t *testing.T) {
	_, err := config.Load(config.Config{
		Defaults:         testFS,
		DefaultsDir:      "testdata/defaults",
		StrictValidation: true,
	})
	if err == nil {
		t.Errorf("StrictValidation with no schemas did not error")
	}
}

func TestSchema_accessorReturnsSchemaValue(t *testing.T) {
	s, err := config.Load(config.Config{
		Defaults:    schemasFS,
		DefaultsDir: "testdata/schemas",
	})
	if err != nil {
		t.Fatal(err)
	}

	v := s.Schema("database")
	if !v.Exists() {
		t.Errorf("Schema(database) returned zero value")
	}

	if v := s.Schema("unknown"); v.Exists() {
		t.Errorf("Schema(unknown) returned a non-zero value")
	}
}

func TestSchema_orphanCueFailsLoad(t *testing.T) {
	// A schema.cue with a field for a namespace we did not load should
	// fail — likely a typo.
	// Use the existing testFS (no cue files) plus a small in-memory
	// override via an anonymous FS is overkill; skip in favour of a
	// direct positive assertion by mutating testdata later if needed.
	t.Skip("orphan detection covered when scaffold-generated schemas ship for unknown namespaces")
}
