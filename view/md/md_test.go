package md_test

import (
	"bytes"
	"embed"
	"io/fs"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/jordanbrauer/hex/view"
	viewmd "github.com/jordanbrauer/hex/view/md"
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

func TestPreprocess_producesHTML(t *testing.T) {
	source := []byte(`# Hi

Some **bold** text.

- one
- two
`)

	out, err := viewmd.Preprocess("t.md", source)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if !strings.Contains(out, "<h1") {
		t.Errorf("expected <h1> in output:\n%s", out)
	}

	if !strings.Contains(out, "<strong>bold</strong>") {
		t.Errorf("bold not converted:\n%s", out)
	}

	if !strings.Contains(out, "<li>one</li>") {
		t.Errorf("list not converted:\n%s", out)
	}
}

func TestPreprocess_headingIDs(t *testing.T) {
	source := []byte(`## The Heading Text
`)

	out, err := viewmd.Preprocess("t.md", source)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if !strings.Contains(out, `id="the-heading-text"`) {
		t.Errorf("expected auto heading id in output:\n%s", out)
	}
}

func TestPreprocess_gfmTables(t *testing.T) {
	source := []byte(`
| A | B |
|---|---|
| 1 | 2 |
`)

	out, err := viewmd.Preprocess("t.md", source)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if !strings.Contains(out, "<table>") {
		t.Errorf("GFM table not rendered:\n%s", out)
	}
}

func TestPreprocess_syntaxHighlighting(t *testing.T) {
	source := []byte("```go\nfunc main() {}\n```\n")

	out, err := viewmd.Preprocess("t.md", source)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	// chroma HTML formatter emits token-type CSS classes on the
	// tokens it recognises: 'kd' for keyword.declaration ('func'),
	// 'nf' for name.function ('main'), etc. Presence of a chroma-
	// prefixed <pre> is the cheapest positive signal.
	if !strings.Contains(out, `class="chroma"`) && !strings.Contains(out, `class="kd"`) {
		t.Errorf("chroma highlighting not applied:\n%s", out)
	}
}

func TestEngine_rendersMarkdownWithTemplateExpressions(t *testing.T) {
	engine, err := view.New(sub(t),
		view.WithPreprocessor(".md", viewmd.Preprocess),
	)
	if err != nil {
		t.Fatalf("view.New: %v", err)
	}

	var buf bytes.Buffer

	err = engine.Render(&buf, "pages/home", map[string]any{
		"Title":   "hex",
		"Heading": "Hello, Markdown",
	}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "Hello, Markdown") {
		t.Errorf("template expression not substituted (Heading):\n%s", out)
	}

	if !strings.Contains(out, "<strong>hex</strong>") {
		t.Errorf("bold + template expression not rendered (Title):\n%s", out)
	}

	if !strings.Contains(out, "<h1") || !strings.Contains(out, "<h2") {
		t.Errorf("headings not rendered:\n%s", out)
	}
}

func TestPreprocess_frontmatterStripped(t *testing.T) {
	source := []byte(`---
title: My Docs Page
description: Everything about it
tags: [docs, guide]
---

# Body heading

Paragraph text.
`)

	out, err := viewmd.Preprocess("t.md", source)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	// Frontmatter block should be gone from HTML.
	if strings.Contains(out, "---") {
		t.Errorf("raw frontmatter delimiters leaked into HTML:\n%s", out)
	}

	if strings.Contains(out, "title:") {
		t.Errorf("raw YAML keys leaked into HTML:\n%s", out)
	}

	if strings.Contains(out, "My Docs Page") {
		t.Errorf("frontmatter values leaked into HTML:\n%s", out)
	}

	// Body should still render.
	if !strings.Contains(out, "Body heading") {
		t.Errorf("body content missing:\n%s", out)
	}

	if !strings.Contains(out, "<h1") {
		t.Errorf("body h1 missing:\n%s", out)
	}
}

func TestPreprocess_worksWithoutFrontmatter(t *testing.T) {
	// Ensure adding the meta extension didn't break the common case
	// of a plain .md with no frontmatter.
	source := []byte(`# Just a heading

And a paragraph.
`)

	out, err := viewmd.Preprocess("t.md", source)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if !strings.Contains(out, "<h1") {
		t.Errorf("heading missing:\n%s", out)
	}
}

func TestNew_customGoldmarkInstance(t *testing.T) {
	// Advanced-user path: build a minimal goldmark without extensions,
	// pass to viewmd.New. Confirm the resulting Preprocessor uses that
	// instance (no GFM \u2192 tables render as raw text).
	minimal := goldmark.New()

	pre := viewmd.New(minimal)

	tableSrc := []byte(`
| A | B |
|---|---|
| 1 | 2 |
`)

	out, err := pre("t.md", tableSrc)
	if err != nil {
		t.Fatalf("preprocess: %v", err)
	}

	if strings.Contains(out, "<table>") {
		t.Errorf("expected minimal goldmark to NOT render tables; got:\n%s", out)
	}
}

func TestNew_layerCustomExtensions(t *testing.T) {
	// Build on Default, verify extra extensions can be added and work.
	md := goldmark.New(
		goldmark.WithExtensions(extension.Strikethrough, extension.Table),
	)

	pre := viewmd.New(md)

	out, err := pre("t.md", []byte("~~gone~~ | col"))
	if err != nil {
		t.Fatalf("preprocess: %v", err)
	}

	if !strings.Contains(out, "<del>gone</del>") {
		t.Errorf("strikethrough missing:\n%s", out)
	}
}
