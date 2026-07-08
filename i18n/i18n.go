// Package i18n is a thin wrapper around github.com/leonelquinteros/gotext that
// gives hex applications GNU gettext-compatible internationalisation with
// PO file support.
//
// Design (see ADR-0012):
//
//   - Type alias `Locale = gotext.Locale` — consumers get the full gotext
//     API through the alias.
//   - hex-owned constructors that load a locale from disk (NewLocale) or
//     from an fs.FS (NewLocaleFS), rooted at the standard gettext layout:
//     <lang>/LC_MESSAGES/<domain>.po
//   - Translator: a multi-locale container. Consumers pick a language per
//     call; missing locales fall back to a configured default.
//   - Package-level default via SetDefault + T / TN / TC / etc., mirroring
//     the pattern used by hex/config.
//
// Example (embed.FS):
//
//	//go:embed locales
//	var localesFS embed.FS
//
//	tr, err := i18n.NewTranslator(i18n.Options{
//	    FS:        localesFS,
//	    Root:      "locales",
//	    Languages: []string{"en", "es", "fr"},
//	    Domain:    "messages",
//	    Fallback:  "en",
//	})
//	if err != nil { return err }
//
//	i18n.SetDefault(tr)
//	tr.Use("es")
//
//	fmt.Println(i18n.T("Hello, world"))              // "Hola, mundo"
//	fmt.Println(i18n.TN("%d apple", "%d apples", 3)) // "3 manzanas"
package i18n

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/leonelquinteros/gotext"
)

// Locale is the type alias for gotext.Locale. Consumers can call every method
// gotext exposes on the aliased type directly.
type Locale = gotext.Locale

// NewLocale loads a locale from disk. root is the directory containing the
// gettext tree (e.g. "./locales"), lang is the language code (e.g. "en"),
// domain is the .po file name without extension (e.g. "messages").
//
// The expected on-disk layout is: <root>/<lang>/LC_MESSAGES/<domain>.po
func NewLocale(root, lang, domain string) *Locale {
	l := gotext.NewLocale(root, lang)
	l.AddDomain(domain)

	return l
}

// NewLocaleFS loads a locale from an fs.FS (typically an //go:embed FS).
// The FS is expected to contain <lang>/LC_MESSAGES/<domain>.po. If your
// embed root is not the language directory itself, use NewLocaleFSPath.
func NewLocaleFS(fsys fs.FS, lang, domain string) *Locale {
	l := gotext.NewLocaleFS(lang, fsys)
	l.AddDomain(domain)

	return l
}

// NewLocaleFSPath is like NewLocaleFS but reads from a subdirectory of the
// FS (e.g. embed root is "locales" and the language dirs live under it).
func NewLocaleFSPath(fsys fs.FS, root, lang, domain string) *Locale {
	l := gotext.NewLocaleFSWithPath(lang, fsys, root)
	l.AddDomain(domain)

	return l
}

// -- Translator ------------------------------------------------------------

// Options configures a Translator.
type Options struct {
	// FS is an optional embed.FS or fs.FS containing the gettext tree.
	// When set, Root is treated as a subdirectory of the FS; when unset,
	// the Translator loads locales from disk starting at Root.
	FS fs.FS

	// Root is either the on-disk directory containing the gettext tree
	// or the subdirectory within FS. May be "" when FS points directly
	// at the language dirs.
	Root string

	// Languages lists language codes to load (e.g. []string{"en", "es"}).
	// Each becomes an available Locale.
	Languages []string

	// Domains lists the .po file names (without extension) to load per
	// language. If empty, defaults to []string{"messages"}. The first
	// domain is the "primary" one used by unqualified T() calls.
	Domains []string

	// Domain is a shorthand for a single-domain setup. Ignored when
	// Domains is non-empty.
	Domain string

	// Fallback is the language code used when the current selection has
	// no translation for a given key. Defaults to the first entry in
	// Languages.
	Fallback string
}

// Translator holds multiple Locales and picks one per call. Safe for
// concurrent use for reads; Use() should be called before concurrent T()
// calls if the caller wants to change the active language.
type Translator struct {
	locales    map[string]*Locale
	current    string
	fallback   string
	primaryDom string
}

// NewTranslator loads every language declared in opts and returns a ready
// Translator.
func NewTranslator(opts Options) (*Translator, error) {
	if len(opts.Languages) == 0 {
		return nil, errors.New("i18n: Options.Languages is required")
	}

	domains := opts.Domains
	if len(domains) == 0 {
		if opts.Domain != "" {
			domains = []string{opts.Domain}
		} else {
			domains = []string{"messages"}
		}
	}

	tr := &Translator{
		locales:    make(map[string]*Locale, len(opts.Languages)),
		primaryDom: domains[0],
	}

	for _, lang := range opts.Languages {
		var loc *Locale

		switch {
		case opts.FS != nil && opts.Root != "":
			loc = gotext.NewLocaleFSWithPath(lang, opts.FS, opts.Root)
		case opts.FS != nil:
			loc = gotext.NewLocaleFS(lang, opts.FS)
		default:
			loc = gotext.NewLocale(opts.Root, lang)
		}

		for _, dom := range domains {
			loc.AddDomain(dom)
		}

		// AddDomain leaves the last-added domain active; explicitly reset
		// to the primary domain so unqualified T() calls hit the expected
		// PO file.
		loc.SetDomain(domains[0])

		tr.locales[lang] = loc
	}

	fallback := opts.Fallback
	if fallback == "" {
		fallback = opts.Languages[0]
	}

	if _, ok := tr.locales[fallback]; !ok {
		return nil, fmt.Errorf("i18n: fallback language %q not loaded", fallback)
	}

	tr.fallback = fallback
	tr.current = fallback

	return tr, nil
}

// Use switches the active language. Returns an error if lang was not
// loaded by NewTranslator.
func (t *Translator) Use(lang string) error {
	if _, ok := t.locales[lang]; !ok {
		return fmt.Errorf("i18n: language %q not loaded", lang)
	}

	t.current = lang

	return nil
}

// Current returns the active language code.
func (t *Translator) Current() string { return t.current }

// Fallback returns the fallback language code.
func (t *Translator) Fallback() string { return t.fallback }

// Languages returns the sorted list of loaded language codes.
func (t *Translator) Languages() []string {
	out := make([]string, 0, len(t.locales))
	for lang := range t.locales {
		out = append(out, lang)
	}

	// tiny n; simple insertion sort keeps sort import out
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}

	return out
}

// Locale returns the *Locale for a specific language, or nil if it was
// not loaded. Consumers who need gotext's per-locale API directly can
// grab it via this method.
func (t *Translator) Locale(lang string) *Locale { return t.locales[lang] }

// T returns the translation of msgid in the active language. If missing,
// falls back to the fallback language; if still missing, returns msgid
// unchanged (per gotext's behavior).
func (t *Translator) T(msgid string, vars ...any) string {
	return t.get(t.current, func(l *Locale) string {
		return l.Get(msgid, vars...)
	})
}

// TN returns the translation of a plural msgid based on n.
func (t *Translator) TN(singular, plural string, n int, vars ...any) string {
	return t.get(t.current, func(l *Locale) string {
		return l.GetN(singular, plural, n, vars...)
	})
}

// TC returns the translation of msgid disambiguated by msgctxt. Used when
// a single word has different meanings depending on context (e.g. "File"
// as a noun vs. a verb).
func (t *Translator) TC(msgid, ctx string, vars ...any) string {
	return t.get(t.current, func(l *Locale) string {
		return l.GetC(msgid, ctx, vars...)
	})
}

// TNC combines plural (TN) and context (TC).
func (t *Translator) TNC(singular, plural string, n int, ctx string, vars ...any) string {
	return t.get(t.current, func(l *Locale) string {
		return l.GetNC(singular, plural, n, ctx, vars...)
	})
}

// TD returns a translation from a specific domain (not the primary one).
func (t *Translator) TD(domain, msgid string, vars ...any) string {
	return t.get(t.current, func(l *Locale) string {
		return l.GetD(domain, msgid, vars...)
	})
}

// TDN is TN for a specific domain.
func (t *Translator) TDN(domain, singular, plural string, n int, vars ...any) string {
	return t.get(t.current, func(l *Locale) string {
		return l.GetND(domain, singular, plural, n, vars...)
	})
}

// IsTranslated reports whether msgid has a non-empty translation in the
// active language's primary domain (not just falling back to msgid).
//
// Passes n=1 into gotext's IsTranslatedND — under every common plural
// rule (including the English "n != 1" one) that maps to the singular
// form, which is what a caller asking "is this translated?" for a
// non-plural msgid actually wants. Passing gotext's default n=0 would
// check the plural form and false-negative for singular-only messages.
func (t *Translator) IsTranslated(msgid string) bool {
	if loc, ok := t.locales[t.current]; ok {
		return loc.IsTranslatedND(t.primaryDom, msgid, 1)
	}

	return false
}

// get is the shared lookup path used by T/TN/TC/etc. It picks the active
// locale (falling back to the fallback locale if the current key is
// missing) and delegates to render, which knows which gotext method to
// call (Get/GetN/GetC/etc.).
//
// Whole-key fallback ("return msgid" → try fallback locale) is not done
// here — the render closure has no msgid to compare against. Callers
// that need strict fallback should query IsTranslated first and switch
// via Use.
func (t *Translator) get(_ string, render func(*Locale) string) string {
	loc := t.locales[t.current]
	if loc == nil {
		loc = t.locales[t.fallback]
	}

	return render(loc)
}

// -- embed.FS convenience -------------------------------------------------

// NewEmbedded is a shortcut for the common case: an //go:embed FS holding
// one or more languages under a root directory, all sharing one domain.
//
//	//go:embed locales
//	var lf embed.FS
//	tr, err := i18n.NewEmbedded(lf, "locales", []string{"en", "es"}, "messages")
func NewEmbedded(fsys embed.FS, root string, languages []string, domain string) (*Translator, error) {
	return NewTranslator(Options{
		FS:        fsys,
		Root:      root,
		Languages: languages,
		Domain:    domain,
	})
}
