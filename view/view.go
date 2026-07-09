// Package view renders Go html/template files for hex web apps.
//
// The Engine loads *.gotmpl templates from an fs.FS (typically an
// //go:embed of web/views/) at construction and exposes:
//
//   - Render — full-page rendering, satisfies echo.Renderer so
//     handlers can call c.Render(200, "pages/home", data).
//   - Partial — renders a named {{ define }} block only, suited to
//     HTMX responses that swap fragments instead of full pages.
//
// Templates use standard html/template syntax, so layouts and
// partials compose via {{ template "name" . }}. hex ships no
// additional syntax or helpers — consumers add funcs via WithFuncs.
package view

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

// Engine holds a parsed template set + optional per-render helpers.
type Engine struct {
	tmpl *template.Template
	fsys fs.FS
	dir  string
	ext  string
}

// Option configures a new Engine.
type Option func(*config)

type config struct {
	funcs template.FuncMap
	ext   string
	dir   string
}

// WithFuncs registers template funcs available in every template.
func WithFuncs(funcs template.FuncMap) Option {
	return func(c *config) { c.funcs = funcs }
}

// WithExtension sets the template file extension the loader scans for.
// Defaults to ".gotmpl".
func WithExtension(ext string) Option {
	return func(c *config) { c.ext = ext }
}

// WithDir sets the subdirectory within the FS to scan. Defaults to the
// root of the FS.
func WithDir(dir string) Option {
	return func(c *config) { c.dir = dir }
}

// New parses every template file under the given FS matching the
// configured extension. Templates are named by their path relative to
// the scan root, minus the extension. So a file at
// `web/views/pages/home.gotmpl` (with fsys rooted at `web/views/`)
// registers as `pages/home`.
func New(fsys fs.FS, opts ...Option) (*Engine, error) {
	cfg := config{ext: ".gotmpl"}
	for _, opt := range opts {
		opt(&cfg)
	}

	if fsys == nil {
		return nil, errors.New("view: fs.FS is nil")
	}

	scanDir := cfg.dir
	if scanDir == "" {
		scanDir = "."
	}

	root := template.New("")
	if cfg.funcs != nil {
		root = root.Funcs(cfg.funcs)
	}

	err := fs.WalkDir(fsys, scanDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() || !strings.HasSuffix(p, cfg.ext) {
			return nil
		}

		data, readErr := fs.ReadFile(fsys, p)
		if readErr != nil {
			return fmt.Errorf("view: read %s: %w", p, readErr)
		}

		rel := strings.TrimPrefix(p, scanDir)
		rel = strings.TrimPrefix(rel, "/")

		name := strings.TrimSuffix(rel, cfg.ext)

		if _, parseErr := root.New(name).Parse(string(data)); parseErr != nil {
			return fmt.Errorf("view: parse %s: %w", p, parseErr)
		}

		return nil
	})
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}

	return &Engine{
		tmpl: root,
		fsys: fsys,
		dir:  scanDir,
		ext:  cfg.ext,
	}, nil
}

// Render implements echo.Renderer. Consumers call:
//
//	return c.Render(http.StatusOK, "pages/home", data)
//
// The template name is the file path relative to the view root,
// without extension.
func (e *Engine) Render(w io.Writer, name string, data any, _ echo.Context) error {
	if e == nil || e.tmpl == nil {
		return errors.New("view: engine not initialised")
	}

	t := e.tmpl.Lookup(name)
	if t == nil {
		return fmt.Errorf("view: no template %q (loaded: %s)", name, e.Names())
	}

	return t.Execute(w, data)
}

// Partial renders a named {{ define }} block, suitable for HTMX
// responses that return an HTML fragment. Unlike Render, Partial
// does not require the block to be a top-level template file — any
// {{ define "block-name" }} inside the loaded set works.
func (e *Engine) Partial(w io.Writer, block string, data any) error {
	if e == nil || e.tmpl == nil {
		return errors.New("view: engine not initialised")
	}

	t := e.tmpl.Lookup(block)
	if t == nil {
		return fmt.Errorf("view: no block %q", block)
	}

	return t.Execute(w, data)
}

// Names returns the sorted list of registered template names for
// error messages and debugging.
func (e *Engine) Names() string {
	if e == nil || e.tmpl == nil {
		return "(none)"
	}

	names := make([]string, 0)
	for _, t := range e.tmpl.Templates() {
		if n := t.Name(); n != "" && n != e.tmpl.Name() {
			names = append(names, n)
		}
	}

	return strings.Join(names, ", ")
}

// Lookup returns the underlying *template.Template for name. Nil when
// not found. Exposed for consumers who need to hold onto a specific
// template (e.g. for tests or lower-level rendering).
func (e *Engine) Lookup(name string) *template.Template {
	if e == nil || e.tmpl == nil {
		return nil
	}

	return e.tmpl.Lookup(name)
}

// Silence unused import when the ext helper is inlined by the
// compiler.
var _ = path.Join
