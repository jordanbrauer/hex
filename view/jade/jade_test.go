package jade_test

import (
	"bytes"
	"embed"
	"io/fs"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex/view"
	viewjade "github.com/jordanbrauer/hex/view/jade"
)

//go:embed testdata
var testFS embed.FS

func sub(t *testing.T) fs.FS {
	t.Helper()

	sub, err := fs.Sub(testFS, "testdata")
	if err != nil {
		t.Fatalf("sub: %v", err)
	}

	return sub
}

func TestPreprocess_producesHTMLTemplateSource(t *testing.T) {
	source := []byte(`
p= .Message
`)

	out, err := viewjade.Preprocess("t.jade", source)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if !strings.Contains(out, "<p>") {
		t.Errorf("expected <p> tag in output:\n%s", out)
	}

	if !strings.Contains(out, ".Message") {
		t.Errorf("expected .Message expression in output:\n%s", out)
	}
}

func TestEngine_rendersJadeTemplate(t *testing.T) {
	engine, err := view.New(sub(t),
		view.WithPreprocessor(".jade", viewjade.Preprocess),
	)
	if err != nil {
		t.Fatalf("view.New: %v", err)
	}

	var buf bytes.Buffer

	err = engine.Render(&buf, "pages/home", map[string]any{
		"Title":   "Test",
		"Heading": "Hello, Jade",
	}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Errorf("missing doctype: %s", out)
	}

	if !strings.Contains(out, "<title>Test</title>") {
		t.Errorf("title not injected: %s", out)
	}

	if !strings.Contains(out, "<h1>Hello, Jade</h1>") {
		t.Errorf("heading not injected: %s", out)
	}

	if !strings.Contains(out, "Welcome") {
		t.Errorf("body text missing: %s", out)
	}
}

func TestEngine_mixesJadeAndGotmpl(t *testing.T) {
	// Verify that both preprocessors coexist. The default .gotmpl
	// still works when we opt into .jade.
	engine, err := view.New(sub(t),
		view.WithExtension(".gotmpl"),
		view.WithPreprocessor(".jade", viewjade.Preprocess),
	)
	if err != nil {
		t.Fatalf("view.New: %v", err)
	}

	// If the engine loaded both, calling Render on the jade one works.
	var buf bytes.Buffer
	err = engine.Render(&buf, "pages/home", map[string]any{
		"Title":   "T",
		"Heading": "H",
	}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
}
