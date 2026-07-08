// Package provider is the default hex/i18n service provider.
//
// It constructs a Translator from a caller-supplied gettext tree and
// installs it as hex/i18n's package-level default so consumers can
// call i18n.T(...) without threading a *Translator around.
package provider

import (
	"errors"
	"io/fs"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/i18n"
	"github.com/jordanbrauer/hex/provider"
)

// Provider wires an *i18n.Translator into the container and installs
// it as the package-level default.
type Provider struct {
	provider.Base

	// Binding is the container name for the translator. Defaults to
	// "translator".
	Binding string

	// FS is the fs.FS containing the gettext tree.
	FS fs.FS

	// Root is the subdirectory within FS. Empty means the FS root.
	Root string

	// Languages must be non-empty (e.g. []string{"en", "es"}).
	Languages []string

	// Domains lists domains to load. Empty means []string{"messages"}.
	Domains []string

	// Fallback is the language code used when the current selection
	// has no translation. Empty means the first entry in Languages.
	Fallback string

	// SkipDefault, when true, prevents installing this translator via
	// i18n.SetDefault. Default behavior is to install so package-level
	// i18n.T() works without threading a *Translator around. Set to
	// true if you manage multiple translators manually.
	SkipDefault bool

	tr *i18n.Translator
}

// Register loads locales and binds the translator.
func (p *Provider) Register(app provider.Application) error {
	if len(p.Languages) == 0 {
		return errors.New("i18n/provider: Languages is required")
	}

	binding := p.Binding
	if binding == "" {
		binding = "translator"
	}

	tr, err := i18n.NewTranslator(i18n.Options{
		FS:        p.FS,
		Root:      p.Root,
		Languages: p.Languages,
		Domains:   p.Domains,
		Fallback:  p.Fallback,
	})
	if err != nil {
		return err
	}

	p.tr = tr

	if !p.SkipDefault {
		i18n.SetDefault(tr)
	}

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.tr, nil
	})

	return nil
}
