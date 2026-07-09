package view_test

import (
	"bytes"
	"embed"
	"io/fs"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex/view"
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

func TestNew_scansTemplates(t *testing.T) {
	e, err := view.New(sub(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	names := e.Names()
	for _, want := range []string{"pages/home", "partials/greeting"} {
		if !strings.Contains(names, want) {
			t.Errorf("Names() = %q, want to contain %q", names, want)
		}
	}
}

func TestRender_full(t *testing.T) {
	e, err := view.New(sub(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data := map[string]any{"Title": "home", "Heading": "Welcome", "Name": "world"}

	var buf bytes.Buffer
	if err := e.Render(&buf, "pages/home", data, nil); err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"<title>home</title>", "<h1>Welcome</h1>", "Hello, world!"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull:\n%s", want, out)
		}
	}
}

func TestPartial_rendersBlockOnly(t *testing.T) {
	e, err := view.New(sub(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var buf bytes.Buffer
	if err := e.Partial(&buf, "partials/greeting", map[string]any{"Name": "htmx"}); err != nil {
		t.Fatalf("Partial: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "Hello, htmx!") {
		t.Errorf("partial output = %q, want greeting", out)
	}

	if strings.Contains(out, "<html>") {
		t.Errorf("partial should not include the layout, got: %s", out)
	}
}

func TestRender_unknownTemplateErrors(t *testing.T) {
	e, err := view.New(sub(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = e.Render(&bytes.Buffer{}, "pages/nope", nil, nil)
	if err == nil {
		t.Errorf("Render of missing template returned nil error")
	}
}
