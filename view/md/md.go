// Package md wires github.com/yuin/goldmark into hex/view as a
// Markdown → HTML Preprocessor.
//
// .md files convert to HTML at Engine construction; the resulting
// string flows through html/template's parser like any other
// template. This means Markdown files can still contain
// {{ .Title }} expressions — html/template substitutes them at
// render time, unchanged by goldmark.
//
// Usage in an app provider factory:
//
//	import (
//	    "github.com/jordanbrauer/hex/view"
//	    viewmd "github.com/jordanbrauer/hex/view/md"
//	)
//
//	engine, err := view.New(webViews,
//	    view.WithDir("web/views"),
//	    view.WithExtension(".gotmpl"),
//	    view.WithPreprocessor(".md", viewmd.Preprocess),
//	)
//
// From handlers, the extension is stripped as usual:
//
//	c.Render(http.StatusOK, "docs/getting-started", data)
//
// # Frontmatter
//
// Optional YAML frontmatter at the top of a .md file is tolerated:
//
//	---
//	title: My Docs Page
//	description: Everything about the app
//	---
//
//	# Page body
//
// The frontmatter block is stripped from the rendered HTML (via
// goldmark-meta). v1 does not surface the parsed key/value data
// back to the caller — templates that need dynamic data still get
// it from the map passed to c.Render(). Frontmatter extraction is
// a planned follow-up; for now the value is that .md files with
// existing frontmatter don't have to be edited before hex renders
// them.
//
// # Defaults
//
// The default Preprocess uses these goldmark options:
//
//   - GFM (tables, task lists, strikethrough, autolink)
//   - Footnotes
//   - DefinitionList
//   - Typographer (smart quotes, ellipses, en/em dashes)
//   - Linkify (bare URLs become clickable)
//   - goldmark-meta (frontmatter tolerated + stripped)
//   - Auto-generated heading IDs (h2/h3 anchor links work)
//   - html.WithUnsafe (raw HTML passes through; template content is
//     developer-authored, not user-supplied — same trust model as
//     .gotmpl files)
//   - chroma highlighting with CSS classes (users control colours
//     via their own stylesheet; no inline colours emitted)
//
// Advanced use — build your own goldmark.Markdown (with extra
// extensions like wikilinks or callouts) and wrap it with
// viewmd.New(...) to plug into hex/view.
package md

import (
	"bytes"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/jordanbrauer/hex/view"
)

// Preprocess is a hex/view.Preprocessor that renders Markdown to
// HTML using Default(). Suitable for the common case where you just
// want to serve .md files as HTML pages.
//
// For custom goldmark configurations, see New.
func Preprocess(name string, source []byte) (string, error) {
	return preprocess(defaultMD, source)
}

// New returns a Preprocessor bound to md. Use this when you've
// configured goldmark with extra extensions (wikilinks, callouts,
// frontmatter metadata, custom node renderers).
//
//	md := goldmark.New(
//	    goldmark.WithExtensions(
//	        extension.GFM,
//	        meta.New(meta.WithStoresInDocument()),
//	        myWikilinks(resolver),
//	    ),
//	)
//	view.New(fsys, view.WithPreprocessor(".md", viewmd.New(md)))
func New(md goldmark.Markdown) view.Preprocessor {
	return func(name string, source []byte) (string, error) {
		return preprocess(md, source)
	}
}

// Default returns a goldmark.Markdown configured with hex's default
// extensions + options (see the package doc). Callers who want to
// extend but keep the defaults typically wrap this via
// goldmark.New(...) instead.
func Default() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.DefinitionList,
			extension.Typographer,
			extension.Linkify,
			meta.New(meta.WithStoresInDocument()),
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.ClassPrefix(""),
					chromahtml.WithLineNumbers(false),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
}

// defaultMD is the shared goldmark instance backing Preprocess. Built
// once at package init; goldmark.Markdown is safe for concurrent use.
var defaultMD = Default()

// preprocess is the shared body used by both Preprocess and the
// New-returned closure.
func preprocess(md goldmark.Markdown, source []byte) (string, error) {
	var buf bytes.Buffer

	if err := md.Convert(source, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}
