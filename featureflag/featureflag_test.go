package featureflag_test

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/featureflag"
)

//go:embed testdata/flags.yaml
var flagsFS embed.FS

func newFSClient(t *testing.T) *featureflag.Client {
	t.Helper()

	c, err := featureflag.NewFromFS(flagsFS, "testdata/flags.yaml", featureflag.Options{
		FileFormat: "yaml",
	})
	if err != nil {
		t.Fatalf("NewFromFS: %v", err)
	}

	t.Cleanup(func() { c.Close() })

	return c
}

func TestNewFromFile_readsAndEvaluates(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "flags.yaml")

	src, err := flagsFS.ReadFile("testdata/flags.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(dst, src, 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := featureflag.NewFromFile(dst, featureflag.Options{
		FileFormat:      "yaml",
		PollingInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("NewFromFile: %v", err)
	}

	defer c.Close()

	ctx := featureflag.NewContext("u1")

	got, err := c.BoolVariation("new-checkout", ctx, false)
	if err != nil {
		t.Fatalf("BoolVariation: %v", err)
	}

	if got {
		t.Errorf("new-checkout for non-beta user = true, want false")
	}
}

func TestNewFromFile_requiresPath(t *testing.T) {
	if _, err := featureflag.NewFromFile("", featureflag.Options{}); err == nil {
		t.Errorf("empty path returned nil error")
	}
}

func TestNewFromFS_missingPath(t *testing.T) {
	if _, err := featureflag.NewFromFS(flagsFS, "testdata/does-not-exist.yaml", featureflag.Options{}); err == nil {
		t.Errorf("missing FS path returned nil error")
	}
}

func TestNewFromFS_requiresPath(t *testing.T) {
	if _, err := featureflag.NewFromFS(flagsFS, "", featureflag.Options{}); err == nil {
		t.Errorf("empty FS path returned nil error")
	}
}

func TestBoolVariation_targetingRule(t *testing.T) {
	c := newFSClient(t)

	tests := []struct {
		name string
		ctx  featureflag.Context
		want bool
	}{
		{
			name: "beta user gets flag on",
			ctx:  featureflag.ContextWith(featureflag.NewContext("u1")).Set("beta", "true").Context(),
			want: true,
		},
		{
			name: "non-beta user gets flag off",
			ctx:  featureflag.ContextWith(featureflag.NewContext("u2")).Set("beta", "false").Context(),
			want: false,
		},
		{
			name: "user without attribute gets default",
			ctx:  featureflag.NewContext("u3"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.BoolVariation("new-checkout", tt.ctx, false)
			if err != nil {
				t.Fatalf("BoolVariation: %v", err)
			}

			if got != tt.want {
				t.Errorf("new-checkout = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntVariation(t *testing.T) {
	c := newFSClient(t)

	got, err := c.IntVariation("max-retries", featureflag.NewContext("u1"), 0)
	if err != nil {
		t.Fatalf("IntVariation: %v", err)
	}

	if got != 3 {
		t.Errorf("max-retries = %d, want 3", got)
	}
}

func TestStringVariation_regionTargeting(t *testing.T) {
	c := newFSClient(t)

	tests := []struct {
		region, want string
	}{
		{"uk", "Good day."},
		{"us", "Howdy, friend!"},
		{"jp", "Welcome!"},
	}

	for _, tt := range tests {
		ctx := featureflag.ContextWith(featureflag.NewContext("u1")).Set("region", tt.region).Context()

		got, err := c.StringVariation("welcome-message", ctx, "fallback")
		if err != nil {
			t.Fatalf("StringVariation(%s): %v", tt.region, err)
		}

		if got != tt.want {
			t.Errorf("welcome-message for region=%s = %q, want %q", tt.region, got, tt.want)
		}
	}
}

func TestFloat64Variation(t *testing.T) {
	c := newFSClient(t)

	got, err := c.Float64Variation("request-timeout", featureflag.NewContext("u1"), 0)
	if err != nil {
		t.Fatalf("Float64Variation: %v", err)
	}

	if got != 1.5 {
		t.Errorf("request-timeout = %v, want 1.5", got)
	}
}

func TestJSONArrayVariation(t *testing.T) {
	c := newFSClient(t)

	got, err := c.JSONArrayVariation("api-endpoints", featureflag.NewContext("u1"), nil)
	if err != nil {
		t.Fatalf("JSONArrayVariation: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("api-endpoints len = %d, want 2", len(got))
	}
}

func TestMissingFlag_returnsDefault(t *testing.T) {
	c := newFSClient(t)

	got, err := c.BoolVariation("does-not-exist", featureflag.NewContext("u1"), true)
	if err == nil {
		t.Errorf("expected error for missing flag")
	}

	if !got {
		t.Errorf("missing flag returned %v, want default true", got)
	}
}

// -- Package-level API ----------------------------------------------------

func TestPackageLevel_zeroValueReturnsDefault(t *testing.T) {
	featureflag.SetDefault(nil)

	if got := featureflag.Bool("x", featureflag.NewContext("u1"), true); !got {
		t.Errorf("Bool without default = %v, want default true", got)
	}

	if got := featureflag.Int("x", featureflag.NewContext("u1"), 42); got != 42 {
		t.Errorf("Int without default = %d, want 42", got)
	}

	if got := featureflag.String("x", featureflag.NewContext("u1"), "fb"); got != "fb" {
		t.Errorf("String without default = %q, want fb", got)
	}
}

func TestPackageLevel_delegatesToDefault(t *testing.T) {
	c := newFSClient(t)

	featureflag.SetDefault(c)
	t.Cleanup(func() { featureflag.SetDefault(nil) })

	ctx := featureflag.ContextWith(featureflag.NewContext("u1")).Set("beta", "true").Context()

	if got := featureflag.Bool("new-checkout", ctx, false); !got {
		t.Errorf("Bool via default = false, want true")
	}

	if got := featureflag.Int("max-retries", ctx, 0); got != 3 {
		t.Errorf("Int via default = %d, want 3", got)
	}

	ctxUK := featureflag.ContextWith(featureflag.NewContext("u1")).Set("region", "uk").Context()
	if got := featureflag.String("welcome-message", ctxUK, "fb"); got != "Good day." {
		t.Errorf("String via default = %q, want Good day.", got)
	}

	if got := featureflag.Float64("request-timeout", ctx, 0); got != 1.5 {
		t.Errorf("Float64 via default = %v, want 1.5", got)
	}
}

func TestNewAnonymousContext(t *testing.T) {
	c := newFSClient(t)

	// Anonymous context still gets rules evaluated by userKey.
	ctx := featureflag.ContextWith(featureflag.NewAnonymousContext("visitor-1")).Set("region", "us").Context()

	got, err := c.StringVariation("welcome-message", ctx, "fallback")
	if err != nil {
		t.Fatalf("StringVariation: %v", err)
	}

	if got != "Howdy, friend!" {
		t.Errorf("anonymous us welcome = %q, want cheerful", got)
	}
}
