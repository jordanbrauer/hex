package i18n

import "sync/atomic"

// Package-level convenience mirrors hex/config: install a Translator once via
// SetDefault, then use T / TN / TC / TNC / TD without threading a *Translator
// around. Read from atomically so a request-scoped goroutine can call T()
// while another goroutine calls SetDefault safely.

//nolint:gochecknoglobals // package-level default translator is the whole point
var defaultTranslator atomic.Pointer[Translator]

// SetDefault installs tr as the package-level default. Subsequent calls to
// the package-level T/TN/TC functions delegate to tr.
func SetDefault(tr *Translator) { defaultTranslator.Store(tr) }

// Default returns the current default Translator, or nil if none is set.
func Default() *Translator { return defaultTranslator.Load() }

// Use switches the active language on the default Translator. Returns an
// error if no default is set or lang is not loaded.
func Use(lang string) error {
	tr := defaultTranslator.Load()
	if tr == nil {
		return errNoDefault
	}

	return tr.Use(lang)
}

// T returns the translation of msgid using the default Translator. Returns
// msgid unchanged when no default is set.
func T(msgid string, vars ...any) string {
	if tr := defaultTranslator.Load(); tr != nil {
		return tr.T(msgid, vars...)
	}

	return msgid
}

// TN returns the plural form of a translation.
func TN(singular, plural string, n int, vars ...any) string {
	if tr := defaultTranslator.Load(); tr != nil {
		return tr.TN(singular, plural, n, vars...)
	}

	if n == 1 {
		return singular
	}

	return plural
}

// TC returns a context-disambiguated translation.
func TC(msgid, ctx string, vars ...any) string {
	if tr := defaultTranslator.Load(); tr != nil {
		return tr.TC(msgid, ctx, vars...)
	}

	return msgid
}

// TNC combines plural and context.
func TNC(singular, plural string, n int, ctx string, vars ...any) string {
	if tr := defaultTranslator.Load(); tr != nil {
		return tr.TNC(singular, plural, n, ctx, vars...)
	}

	if n == 1 {
		return singular
	}

	return plural
}

// TD returns a translation from a specific domain.
func TD(domain, msgid string, vars ...any) string {
	if tr := defaultTranslator.Load(); tr != nil {
		return tr.TD(domain, msgid, vars...)
	}

	return msgid
}

// TDN is TN for a specific domain.
func TDN(domain, singular, plural string, n int, vars ...any) string {
	if tr := defaultTranslator.Load(); tr != nil {
		return tr.TDN(domain, singular, plural, n, vars...)
	}

	if n == 1 {
		return singular
	}

	return plural
}

// errNoDefault is returned by write-shaped package-level funcs when no
// default has been installed.
var errNoDefault = i18nError("i18n: no default translator set (call i18n.SetDefault)")

type i18nError string

func (e i18nError) Error() string { return string(e) }
