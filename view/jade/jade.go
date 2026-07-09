// Package jade wires github.com/Joker/jade into hex/view as a
// template Preprocessor.
//
// Jade (renamed Pug in the JS world) is an indentation-based
// templating language. jade.Parse transpiles Jade source to
// html/template-compatible source at runtime, so hex/view can render
// it through its existing pipeline (Render / Partial / echo.Renderer)
// alongside plain .gotmpl files.
//
// Usage in an app provider factory:
//
//	import (
//	    "github.com/jordanbrauer/hex/view"
//	    viewjade "github.com/jordanbrauer/hex/view/jade"
//	)
//
//	engine, err := view.New(webViews,
//	    view.WithDir("web/views"),
//	    view.WithExtension(".gotmpl"),          // keep supporting plain templates
//	    view.WithPreprocessor(".jade", viewjade.Preprocess),
//	    view.WithPreprocessor(".pug", viewjade.Preprocess),  // both extensions work
//	)
//
// From handlers, reference the template by its path relative to the
// view root without the extension:
//
//	c.Render(http.StatusOK, "pages/home", data)
//
// The engine finds either "pages/home.jade" or "pages/home.gotmpl";
// whichever exists is loaded, transpiled if needed, and rendered.
package jade

import (
	"github.com/Joker/jade"
)

// Preprocess is a view.Preprocessor that runs Jade source through
// github.com/Joker/jade and returns the resulting html/template
// string. name is the template's path relative to the view root
// (used by Jade in error messages).
func Preprocess(name string, source []byte) (string, error) {
	return jade.Parse(name, source)
}
